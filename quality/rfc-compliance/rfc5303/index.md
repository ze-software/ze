# RFC 5303 - Three-Way Handshake for IS-IS Point-to-Point Adjacencies

Experimental. Every requirement this repository extracted from RFC 5303, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 9 of 18 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 18 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 18 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 18 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.1% | 2 of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 18 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 18 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 38.9% | 7 of 18 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 9 | of 18 gated MUSTs judged | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 18 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 19 |
| Gated MUST-level | 18 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 7 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 9 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5303.md` |
| Requirement shard | `rfc/requirements/rfc5303.md` |
| RFC text | `rfc/full/rfc5303.txt` |

## Enrolment

Enrolled: Three-Way Handshake for IS-IS P2P Adjacencies (RFC 5303): 9 MET (TLV 240 origination+decode, Adjacency Three-Way State + Extended Local Circuit ID fields, System-ID echo gating adjacency to Up, two-way fall-back) + 7 gap (Ext Local Circuit ID uint8(ifindex) truncation, Neighbor Ext Circuit ID echoed 0 and unexamined, no invalid-state discard, no loop-detection mismatch discard, derived state model lacks distinct Accept/restart-Down actions) + 2 not-applicable (option always processed, option always emitted)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Point-to-point IIH three-way handshake: TLV 240 (Point-to-Point Three-Way Adjacency) origination and decode, the mandatory Adjacency Three-Way State and Extended Local Circuit ID fields, the neighbor System ID echo that gates the adjacency to Up, and the legacy two-way fall-back for peers that omit the option. Tests bound per requirement in [`rfc/requirements/rfc5303.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5303.md).

**What the ledger says remains**

Gaps gated in [`rfc/short/rfc5303.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5303.md): the Extended Local Circuit ID is derived as uint8(ifindex) so it is not unique beyond 256 interfaces ([`RFC5303-3.2-4`](#rfc5303-3.2-4)); the Neighbor Extended Local Circuit ID is echoed as 0 and never examined ([`RFC5303-3.2-6`](#rfc5303-3.2-6), [`RFC5303-3.2-8`](#rfc5303-3.2-8)); an invalid three-way state is not discarded ([`RFC5303-3.2-7`](#rfc5303-3.2-7)); the loop-detection neighbor-mismatch discard is absent ([`RFC5303-3.2-9`](#rfc5303-3.2-9)); and Ze derives the three-way state from the ISO 10589 adjacency state instead of the distinct Section 3.2 state table, so the "Accept" and restart "Down" actions are not modeled ([`RFC5303-3.2-11`](#rfc5303-3.2-11), [`RFC5303-3.2-12`](#rfc5303-3.2-12)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC5303-3.1-1`](#rfc5303-3.1-1), [`RFC5303-3.1-4`](#rfc5303-3.1-4), [`RFC5303-3.1-6`](#rfc5303-3.1-6), [`RFC5303-3.2-1`](#rfc5303-3.2-1), [`RFC5303-3.2-2`](#rfc5303-3.2-2), [`RFC5303-3.2-3`](#rfc5303-3.2-3), [`RFC5303-3.2-5`](#rfc5303-3.2-5), [`RFC5303-3.2-10`](#rfc5303-3.2-10), [`RFC5303-3.2-13`](#rfc5303-3.2-13)

**Annotated instead of tested (9):** [`RFC5303-3.1-2`](#rfc5303-3.1-2), [`RFC5303-3.1-3`](#rfc5303-3.1-3), [`RFC5303-3.2-4`](#rfc5303-3.2-4), [`RFC5303-3.2-6`](#rfc5303-3.2-6), [`RFC5303-3.2-7`](#rfc5303-3.2-7), [`RFC5303-3.2-8`](#rfc5303-3.2-8), [`RFC5303-3.2-9`](#rfc5303-3.2-9), [`RFC5303-3.2-11`](#rfc5303-3.2-11), [`RFC5303-3.2-12`](#rfc5303-3.2-12)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5303-3.1-1` | Any system that supports this mechanism SHALL include this option in its Point-to-Point IIH packets. (§3.1) | SHALL | 3.1 | **positive:** `unit/verify` [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L125). **negative:** `unit/verify` [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L129) |
| `RFC5303-3.1-2` | Any system that does not understand this option SHALL ignore it (§3.1) | SHALL | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains a system that does NOT understand the option; Ze implements and processes it (packet/tlv_core.go DecodeP2PThreeWayTLV decodes it, circuit/runtime.go:143 consumes it), so the "does not understand" role never applies to Ze |
| `RFC5303-3.1-3` | Any system that does not understand this option SHALL ignore it, and (of course) SHALL NOT include it in its own IIH packets. (§3.1) | SHALL NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains a non-supporting system's emission; Ze supports the mechanism and always emits TLV 240 in its point-to-point IIH (circuit/hello.go:158 threeWayTLV, circuit/hello.go:204 buildP2PHello), so the "does not understand" role never applies to Ze |
| `RFC5303-3.1-4` | Any system that supports this mechanism MUST include the Adjacency Three-Way State field in this option. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISP2PThreeWayStateFieldIncluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L16). **negative:** `unit/verify` [`TestISISP2PThreeWayStateFieldPresentWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L49) |
| `RFC5303-3.1-6` | Any system that is able to process this option SHALL follow the procedures below. (§3.1) | SHALL | 3.1 | **positive:** `unit/verify` [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L396). **negative:** `unit/verify` [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L400) |
| `RFC5303-3.2-1` | The current three-way state of the adjacency with its neighbor on the link (as defined in new section 8.2.4.1.1 introduced later in the document) SHALL be reported in the Adjacency Three-Way State field. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L146). **negative:** `unit/verify` [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L149) |
| `RFC5303-3.2-2` | If no adjacency exists, the state SHALL be reported as Down. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L175). **negative:** `unit/verify` [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L178) |
| `RFC5303-3.2-3` | The Extended Local Circuit ID field SHALL contain a value assigned by this IS when the circuit is created. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L203). **negative:** `unit/verify` [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L206) |
| `RFC5303-3.2-4` | This value SHALL be unique among all the circuits of this Intermediate System. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Extended Local Circuit ID is derived as uint8(ifindex) (circuits.go:317), truncating the interface index to 8 bits; two circuits whose ifindexes collide modulo 256 emit the same value, so uniqueness among all circuits is not guaranteed and the 4-octet field never exceeds 255 |
| `RFC5303-3.2-5` | If the system ID and Extended Local Circuit ID of the neighboring system are known (in adjacency three-way state Initializing or Up), the neighbor's system ID SHALL be reported in the Neighbor System ID field (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L226). **negative:** `unit/verify` [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L229) |
| `RFC5303-3.2-6` | the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not track the neighbor's Extended Local Circuit ID; threeWayTLV echoes a constant 0 in the Neighbor Extended Local Circuit ID field (circuit/hello.go:168) and updateThreeWay never stores the neighbor's value (adjacency/fsm.go:232), so the neighbor's actual extended circuit ID is never reported |
| `RFC5303-3.2-7` | If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the decoder validates only the TLV length, not the state value (packet/tlv_core.go DecodeP2PThreeWayTLV); an out-of-range Adjacency Three-Way State is folded into the FSM (adjacency/fsm.go:237) and treated as not-bidirectional rather than discarding the PDU, so the "discard, no further action" path is absent |
| `RFC5303-3.2-8` | If the option with a valid Adjacency Three-Way State is present, the Neighbor System ID and Neighbor Extended Local Circuit ID fields, if present, SHALL be examined. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** updateThreeWay examines only the Neighbor System ID (adjacency/fsm.go:240) to set neighborSawUs; the Neighbor Extended Local Circuit ID is decoded (packet/tlv_core.go DecodeP2PThreeWayTLV, the p2pThreeWayLenFull branch) but the FSM never examines it, so the required examination of both fields is incomplete |
| `RFC5303-3.2-9` | If they are present, and the Neighbor System ID contained therein does not match the local system's ID, or the Neighbor Extended Local Circuit ID does not match the local system's extended circuit ID, the PDU SHALL be discarded and no further action is taken. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on a Neighbor System ID that does not match ours Ze keeps the adjacency Initializing (adjacency/fsm.go:240) and still processes the PDU, arming the hold timer and recording the neighbor (adjacency/fsm.go:161); it neither discards the PDU nor checks the Neighbor Extended Local Circuit ID, so the loop-detection discard is absent |
| `RFC5303-3.2-10` | In section 8.2.4.2 a and b, the action "Up" from state tables 5, 6, 7, and 8 may create a new adjacency but the three-way state of the adjacency SHALL be Down. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L247). **negative:** `unit/verify` [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L251) |
| `RFC5303-3.2-11` | If the action taken from section 8.2.4.2 a or b is "Up" or "Accept", the IS SHALL perform the action indicated by the new adjacency three-way state table below, based on the current adjacency three-way state and the received Adjacency Three-Way State value from the option. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not maintain a distinct three-way state nor the four-cell section 3.2 state table; bidirectionality is derived from the ISO 10589 adjacency state plus the System ID echo (adjacency/fsm.go:207 bidirectional), so the table's "Accept" and restart "Down" cells are not modeled |
| `RFC5303-3.2-12` | If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the "Down"/"Neighbor restarted" action is absent; when our three-way state is Down and the neighbor reports Up Ze brings the adjacency Up (adjacency/fsm.go:189) rather than deleting it and emitting adjacencyStateChange(Down) |
| `RFC5303-3.2-13` | If the new action is "Initialize", no event is generated and the adjacency three-way state SHALL be set to "Initializing". (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L426). **negative:** `unit/verify` [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L429) |
| `RFC5303-3.1-5` | The other fields in this option SHOULD be included as explained below in section 3.2. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5303-3.1-2`](#rfc5303-3.1-2) Any system that does not understand this option SHALL ignore it (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains a system that does NOT understand the option; Ze implements and processes it (packet/tlv_core.go DecodeP2PThreeWayTLV decodes it, circuit/runtime.go:143 consumes it), so the "does not understand" role never applies to Ze |
| [`RFC5303-3.1-3`](#rfc5303-3.1-3) Any system that does not understand this option SHALL ignore it, and (of course) SHALL NOT include it in its own IIH packets. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains a non-supporting system's emission; Ze supports the mechanism and always emits TLV 240 in its point-to-point IIH (circuit/hello.go:158 threeWayTLV, circuit/hello.go:204 buildP2PHello), so the "does not understand" role never applies to Ze |
| [`RFC5303-3.2-4`](#rfc5303-3.2-4) This value SHALL be unique among all the circuits of this Intermediate System. (§3.2) | {gap}, no test | the Extended Local Circuit ID is derived as uint8(ifindex) (circuits.go:317), truncating the interface index to 8 bits; two circuits whose ifindexes collide modulo 256 emit the same value, so uniqueness among all circuits is not guaranteed and the 4-octet field never exceeds 255 |
| [`RFC5303-3.2-6`](#rfc5303-3.2-6) the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2) | {gap}, no test | Ze does not track the neighbor's Extended Local Circuit ID; threeWayTLV echoes a constant 0 in the Neighbor Extended Local Circuit ID field (circuit/hello.go:168) and updateThreeWay never stores the neighbor's value (adjacency/fsm.go:232), so the neighbor's actual extended circuit ID is never reported |
| [`RFC5303-3.2-7`](#rfc5303-3.2-7) If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken. (§3.2) | {gap}, no test | the decoder validates only the TLV length, not the state value (packet/tlv_core.go DecodeP2PThreeWayTLV); an out-of-range Adjacency Three-Way State is folded into the FSM (adjacency/fsm.go:237) and treated as not-bidirectional rather than discarding the PDU, so the "discard, no further action" path is absent |
| [`RFC5303-3.2-8`](#rfc5303-3.2-8) If the option with a valid Adjacency Three-Way State is present, the Neighbor System ID and Neighbor Extended Local Circuit ID fields, if present, SHALL be examined. (§3.2) | {gap}, no test | updateThreeWay examines only the Neighbor System ID (adjacency/fsm.go:240) to set neighborSawUs; the Neighbor Extended Local Circuit ID is decoded (packet/tlv_core.go DecodeP2PThreeWayTLV, the p2pThreeWayLenFull branch) but the FSM never examines it, so the required examination of both fields is incomplete |
| [`RFC5303-3.2-9`](#rfc5303-3.2-9) If they are present, and the Neighbor System ID contained therein does not match the local system's ID, or the Neighbor Extended Local Circuit ID does not match the local system's extended circuit ID, the PDU SHALL be discarded and no further action is taken. (§3.2) | {gap}, no test | on a Neighbor System ID that does not match ours Ze keeps the adjacency Initializing (adjacency/fsm.go:240) and still processes the PDU, arming the hold timer and recording the neighbor (adjacency/fsm.go:161); it neither discards the PDU nor checks the Neighbor Extended Local Circuit ID, so the loop-detection discard is absent |
| [`RFC5303-3.2-11`](#rfc5303-3.2-11) If the action taken from section 8.2.4.2 a or b is "Up" or "Accept", the IS SHALL perform the action indicated by the new adjacency three-way state table below, based on the current adjacency three-way state and the received Adjacency Three-Way State value from the option. (§3.2) | {gap}, no test | Ze does not maintain a distinct three-way state nor the four-cell section 3.2 state table; bidirectionality is derived from the ISO 10589 adjacency state plus the System ID echo (adjacency/fsm.go:207 bidirectional), so the table's "Accept" and restart "Down" cells are not modeled |
| [`RFC5303-3.2-12`](#rfc5303-3.2-12) If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2) | {gap}, no test | the "Down"/"Neighbor restarted" action is absent; when our three-way state is Down and the neighbor reports Up Ze brings the adjacency Up (adjacency/fsm.go:189) rather than deleting it and emitting adjacencyStateChange(Down) |

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

### [`RFC5303-3.1-6`](#rfc5303-3.1-6)

Any system that is able to process this option SHALL follow the procedures below. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence binds Ze to every procedure of section 3.2. internal/plugins/isis/adjacency/fsm_test.go::TestISISThreeWayProceduresEngagedByOption proves one: with TLV 240 present an Up-without-echo neighbor stays Initializing, without it the legacy two-way reaches Up. The invalid-state discard (RFC5303-3.2-7), the Neighbor ID mismatch discard (3.2-9), the state table (3.2-11) and its Down action (3.2-12) are not followed; those rows carry {gap}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L400) | unit/verify | unproven |
| positive | [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L396) | unit/verify | unproven |

### [`RFC5303-3.2-1`](#rfc5303-3.2-1)

The current three-way state of the adjacency with its neighbor on the link (as defined in new section 8.2.4.1.1 introduced later in the document) SHALL be reported in the Adjacency Three-Way State field. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a transmitted IIH whose Adjacency Three-Way State is not the adjacency's current state. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsCurrentState asserts what p2pThreeWayState returns (Up, then Initializing), not the transmitted field: sendP2PHello (circuit/runtime.go) passes that state to buildP2PHello, and no tagged unit asserts the IIH it sends, so a send path that ignored the computed state stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L149) | unit/verify | unproven |
| positive | [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L146) | unit/verify | unproven |

### [`RFC5303-3.2-2`](#rfc5303-3.2-2)

If no adjacency exists, the state SHALL be reported as Down. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a state other than Down reported with no adjacency. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayDownWhenNoAdjacency asserts p2pThreeWayState returns Down on an empty table (and not Down once Up), not the Adjacency Three-Way State field of a transmitted IIH; the sendP2PHello wiring is unasserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L178) | unit/verify | unproven |
| positive | [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L175) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: omitting the neighbor's System ID when it is known in Initializing or Up. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayReportsNeighborSystemID proves buildP2PHello echoes the ID it is handed and omits it when told none is known. No assertion shows that an Initializing or Up adjacency makes p2pThreeWayState return haveNeighbor with that ID (TestISISThreeWayReportsCurrentState discards those results), so the 'known (in Initializing or Up)' clause is unproven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L229) | unit/verify | unproven |
| positive | [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L226) | unit/verify | unproven |

### [`RFC5303-3.2-6`](#rfc5303-3.2-6)

the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-6, so no unit is bound to it.

### [`RFC5303-3.2-7`](#rfc5303-3.2-7)

If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-7, so no unit is bound to it.

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

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence: the ISO 10589 'Up' action may create a new adjacency but its three-way state SHALL be Down, the input to the section 3.2 state table. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayNewAdjacencyStateDown proves a neighbouring reporting rule: a table record already in StateDown makes p2pThreeWayState report Down. It never drives ReceiveHello (adjacency/fsm.go) through adjacency creation; there Ze moves a new adjacency straight to Initializing (TestISISThreeWayProceduresEngagedByOption), and the state table is not modelled (RFC5303-3.2-11 {gap}).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L251) | unit/verify | unproven |
| positive | [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L247) | unit/verify | unproven |

### [`RFC5303-3.2-11`](#rfc5303-3.2-11)

If the action taken from section 8.2.4.2 a or b is "Up" or "Accept", the IS SHALL perform the action indicated by the new adjacency three-way state table below, based on the current adjacency three-way state and the received Adjacency Three-Way State value from the option. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-11, so no unit is bound to it.

### [`RFC5303-3.2-12`](#rfc5303-3.2-12)

If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-12, so no unit is bound to it.

### [`RFC5303-3.2-13`](#rfc5303-3.2-13)

If the new action is "Initialize", no event is generated and the adjacency three-way state SHALL be set to "Initializing". (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: on an Initialize action, generating an event or not setting Initializing. internal/plugins/isis/adjacency/fsm_test.go::TestISISThreeWayInitializeAction: from Down with received Down, tr.State != StateInitializing is Fatalf and SessionUp/SessionDown is Fatalf; negative: received Up with our echo goes Up with an event. The table yields Initialize in three cells (current Down, Initializing, Up with received Down); the Initializing and Up rows have no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L429) | unit/verify | unproven |
| positive | [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/fsm_test.go#L426) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5303.txt |
| Source fingerprint | 126dfb230a6e85de |
| Record | rfc/extraction/rfc5303.json |
| Mapped sentences | 16 |
| Declined as scope | 2 |
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
| `3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sending clause repeats the Section 3.1 obligation to include the option in every Point-to-Point IIH, which site 3.1:1 already maps to RFC5303-3.1-1. | The IS SHALL include the Point-to-Point Three-Way Adjacency option in the transmitted Point-to-Point IIH PDU. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF IPR boilerplate from the Intellectual Property notice; it invites disclosure of patent rights and states no protocol obligation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 5303, so its obligations are stated where they were written.
