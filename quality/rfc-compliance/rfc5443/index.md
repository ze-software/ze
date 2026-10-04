# RFC 5443 - LDP IGP Synchronization

Experimental. Every requirement this repository extracted from RFC 5443, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 62.5% | 5 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 66.7% | 12 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 25.0% | 2 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.5% | 1 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 5 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc5443.md` |
| Requirement shard | `rfc/requirements/rfc5443.md` |
| RFC text | `rfc/full/rfc5443.txt` |

## Enrolment

Enrolled: LDP IGP Synchronization: eight MUST-level requirements. Five are met with positive+negative tags in internal/plugins/ospf (OSPF LDP-IGP sync state machine): 2-1 (advertise a link at maximum metric until LDP sync is achieved), 2-2 (the maximum metric value is LSInfinity 0xFFFF), 2-5 (do not declare sync until the LDP session is up and the hold-down estimate completes), 3-1 (the cost-out applies to the whole interface/segment, not per-neighbor), and 4-1 (only the IGP link metric is raised, not TE). 2-3 (IS-IS uses 2^24-2 until sync) is {gap}: ze implements LDP-IGP sync only in OSPF; IS-IS has no sync state machine. 2-4 (IS-IS must not use 2^24-1) and 2-6 (use End-of-LIB if implemented) are {not-applicable}: ze runs no IS-IS sync and its LDP has no End-of-LIB. Disclosed in the docs/features/rfc-status.md RFC 5443 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- OSPF LDP-IGP sync state machine ([`internal/plugins/ospf/ldp_sync.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync.go)): per-interface cost-out to LSInfinity (0xFFFF) while LDP is not fully operational, hold-down estimation of label-binding exchange, configured-cost restore, and whole-segment cost-out
- Section 4 raises only the IP link cost (ze originates no TE LSA). Tests bound per requirement in [`rfc/requirements/rfc5443.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5443.md).


**What the ledger says remains**

One MUST gap gated in [`rfc/short/rfc5443.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5443.md): ze has no IS-IS LDP-IGP sync, so the IS-IS 2^24-2 max-metric cost-out ([`RFC5443-2-3`](#rfc5443-2-3)) is unimplemented. End-of-LIB ([`RFC5443-2-6`](#rfc5443-2-6)) and the 2^24-1 misuse guard ([`RFC5443-2-4`](#rfc5443-2-4)) are not applicable (no IS-IS sync, no End-of-LIB).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5443-2-1`](#rfc5443-2-1), [`RFC5443-2-2`](#rfc5443-2-2), [`RFC5443-2-5`](#rfc5443-2-5), [`RFC5443-3-1`](#rfc5443-3-1), [`RFC5443-4-1`](#rfc5443-4-1)

**Annotated (including scoped evidence) (3):** [`RFC5443-2-3`](#rfc5443-2-3), [`RFC5443-2-4`](#rfc5443-2-4), [`RFC5443-2-6`](#rfc5443-2-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5443-2-1` | In detail: when LDP is not "fully operational" (see below) on a given link, the IGP will advertise the link with maximum cost to avoid any transit traffic over it. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L145). **positive:** `unit/verify` [`TestRFC5443NotOperationalLinkAdvertisedAtMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L145). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188). **negative:** `unit/verify` [`TestRFC5443OperationalLinkAdvertisedAtConfiguredCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L170) |
| `RFC5443-2-2` | In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L158). **positive:** `unit/verify` [`TestRFC5443NotOperationalLinkAdvertisedAtMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L148). **negative:** `unit/verify` [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L294). **negative:** `unit/verify` [`TestRFC5443MaxCostIsNotTheConfiguredNearMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L188) |
| `RFC5443-2-3` | In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE). (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements RFC 5443 LDP-IGP sync only in OSPF (internal/plugins/ospf/ldp_sync.go); IS-IS has no LDP-IGP sync state machine, so there is no IS-IS 2^24-2 max-metric cost-out producer (internal/plugins/isis defines only the generic MaxMetric topology-removal value) |
| `RFC5443-2-4` | Indeed, if a link is configured with 2^24-1 (the maximum link metric per [RFC5305]), then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort in case of massive failure. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no IS-IS LDP-IGP sync (internal/plugins/isis has no sync state machine), so it never originates an LDP-sync-driven IS-IS metric and cannot misuse the 2^24-1 value |
| `RFC5443-2-5` | LDP is considered fully operational on a link when an LDP hello adjacency exists on it, a suitable associated LDP session (matching the LDP Identifier of the hello adjacency) is established to the peer at the other end of the link, and all label bindings have been exchanged over the session. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L167). **positive:** `unit/verify` [`TestRFC5443SessionUpOnPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5443_sessionup_test.go#L144). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L190). **negative:** `unit/verify` [`TestRFC5443NoSessionUpBeforePeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5443_sessionup_test.go#L121) |
| `RFC5443-2-6` | The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the precondition is unmet -- ze's LDP (internal/plugins/ldp) implements no End-of-LIB notification, so ze uses the RFC 5443 hold-down-estimate alternative instead |
| `RFC5443-3-1` | On broadcast links with more than one IGP/LDP peer, the cost-out procedure can only be applied to the link as a whole and not to an individual peer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L366). **positive:** `unit/verify` [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L156). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L369). **negative:** `unit/verify` [`TestRFC5443BroadcastOnePeerUpKeepsSegmentCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L203) |
| `RFC5443-4-1` | The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5443CostOutRaisesIPCostNotDefaultTEMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L208). **negative:** `unit/verify` [`TestRFC5443CostOutLeavesExplicitTEMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L226) |
| `RFC5443-3-2` | Note however that non- optimal IP forwarding only occurs for a short time after a link comes up or when there is a genuine problem on a link. In the latter case, an implementation should issue network management alerts to report the error condition and enable the operator to address it. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-5-1` | These errors are considered general security issues and implementors should follow the current best security practice [MPLS-GMPLS-Sec]. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-2-7` | A simple implementation strategy is to use a configurable hold-down timer to allow LDP session establishment before declaring LDP fully operational. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-2-8` | When LDP End-of-LIB is implemented, the configurable hold-down timer is no longer needed. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-3-3` | So a policy decision has to be made whether the unavailability of LDP service to one peer should result in the traffic being diverted away from all the peers on the link. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-4-2` | Again, raising the IP cost of the tunnel while there is no operational LDP session will solve the problem. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5443-2-3`](#rfc5443-2-3) In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE). (§2) | {gap}, no test | ze implements RFC 5443 LDP-IGP sync only in OSPF (internal/plugins/ospf/ldp_sync.go); IS-IS has no LDP-IGP sync state machine, so there is no IS-IS 2^24-2 max-metric cost-out producer (internal/plugins/isis defines only the generic MaxMetric topology-removal value) |
| [`RFC5443-2-4`](#rfc5443-2-4) Indeed, if a link is configured with 2^24-1 (the maximum link metric per [RFC5305]), then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort in case of massive failure. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no IS-IS LDP-IGP sync (internal/plugins/isis has no sync state machine), so it never originates an LDP-sync-driven IS-IS metric and cannot misuse the 2^24-1 value |
| [`RFC5443-2-6`](#rfc5443-2-6) The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: the precondition is unmet -- ze's LDP (internal/plugins/ldp) implements no End-of-LIB notification, so ze uses the RFC 5443 hold-down-estimate alternative instead |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5443-2-1`](#rfc5443-2-1)

In detail: when LDP is not "fully operational" (see below) on a given link, the IGP will advertise the link with maximum cost to avoid any transit traffic over it. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 by an independent judge. TestRFC5443NotOperationalLinkAdvertisedAtMaxCost builds the engine from config, runs the production applyLDPSyncOverride, originates the Router-LSA and decodes it: the p2p link is exactly 0xFFFF before session-up and again during hold-down, while the subnet stub keeps 100 (only the link is costed out). TestRFC5443OperationalLinkAdvertisedAtConfiguredCost synchronizes (session-up, hold-down fired) and reads the configured 100, so an always-max stub fails. Both have observed-red revert records on applyLDPSyncOverride. The older helper-level tags (effectiveP2PCost, show path only) stay and add nothing. The 'fully operational' definition itself is RFC5443-2-5.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188) | unit/verify | unproven |
| negative | [`TestRFC5443OperationalLinkAdvertisedAtConfiguredCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L145) | unit/verify | unproven |
| positive | [`TestRFC5443NotOperationalLinkAdvertisedAtMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L145) | unit/verify | revert, verified |

### [`RFC5443-2-2`](#rfc5443-2-2)

In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. The positive reads exactly 0xFFFF from the originated, decoded Router-LSA. TestRFC5443MaxCostIsNotTheConfiguredNearMaxCost configures 0xFFFE: not-synchronized reads 0xFFFF (not the configured near-max value), synchronized returns 0xFFFE, so a cost-out to any value other than LSInfinity goes red. Revert records on lsdb routerLinks, which sets LSInfinity for LDPSyncMaxMetric. The older tags (constant check, unmanaged interface) stay and prove only neighbouring facts.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L294) | unit/verify | unproven |
| negative | [`TestRFC5443MaxCostIsNotTheConfiguredNearMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L188) | unit/verify | revert, verified |
| positive | [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L158) | unit/verify | unproven |
| positive | [`TestRFC5443NotOperationalLinkAdvertisedAtMaxCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L148) | unit/verify | revert, verified |

### [`RFC5443-2-3`](#rfc5443-2-3)

In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE). (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-3, so no unit is bound to it.

### [`RFC5443-2-4`](#rfc5443-2-4)

Indeed, if a link is configured with 2^24-1 (the maximum link metric per [RFC5305]), then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort in case of massive failure. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-4, so no unit is bound to it.

### [`RFC5443-2-5`](#rfc5443-2-5)

LDP is considered fully operational on a link when an LDP hello adjacency exists on it, a suitable associated LDP session (matching the LDP Identifier of the hello adjacency) is established to the peer at the other end of the link, and all label bindings have been exchanged over the session. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Three clauses: a hello adjacency exists, a session matching its LDP Identifier is established, all bindings exchanged. Established (RULINGS R4 fix, c29): TestRFC5443NoSessionUpBeforePeerKeepAlive drives runSession with a peer that completes TCP and the Initialization exchange but sends no KeepAlive, and requires zero SessionUp and state open-received; TestRFC5443SessionUpOnPeerKeepAlive sends the KeepAlive and requires exactly one SessionUp naming the adjacency's interface eth0 and LDP Identifier 10.0.0.2:0. Revert records (handleInit, keepaliveReceived) prove reach; author overlays with HEAD's OPENSENT->OPERATIONAL handleInit and a KeepAlive that makes no transition turned each red with the clause-specific message (judge read the job logs). Adjacency and LDP Identifier: SessionUp is published only by runSession for the adjacency startSessionForAdj dialed, from the operational transition, and an Initialization from another LSR ID or label space is refused before OPENREC (RFC5036-3.5.3-9 units), so no SessionUp is published for a non-matching session. Bindings exchanged (hold-down estimate): TestLDPSyncSubscribesSessionEvents and TestLDPSyncRestoresAfterHoldDown drive the real ldpSyncManager (SessionUp -> hold-down at LSInfinity, Synchronized only on timer expiry), with revert records on onSessionUp and onHoldDownExpiry added by the judge. Interop ospf-ldp-sync-frr passed with the fix (2026-10-02). Not this row: onDone emits SessionDown also for a session that never published SessionUp (journal state-read-after-teardown-reset-it.md).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5443NoSessionUpBeforePeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5443_sessionup_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L190) | unit/verify | revert, verified |
| positive | [`TestRFC5443SessionUpOnPeerKeepAlive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5443_sessionup_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L167) | unit/verify | revert, verified |

### [`RFC5443-2-6`](#rfc5443-2-6)

The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-6, so no unit is bound to it.

### [`RFC5443-3-1`](#rfc5443-3-1)

On broadcast links with more than one IGP/LDP peer, the cost-out procedure can only be applied to the link as a whole and not to an individual peer. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). TestRFC5443BroadcastCostOutCoversWholeSegment builds a real engine with broadcast eth1, two Full IGP peers, a real ospfspf.Computer over a three-router LSDB with an alternate path, runs production applyLDPSyncOverride and decodes the originated Router-LSA: not synchronized -> 0 transit and 0 point-to-point links (only the subnet stub), synchronized -> exactly 1 transit link to the pseudonode and 0 per-peer links, so the cost-out never has per-peer granularity. Negative TestRFC5443BroadcastOnePeerUpKeepsSegmentCostedOut: an LDP session up with hold-down pending still yields 0 transit and 0 per-peer links. Ze applies the RFC 6138 transit-link withhold on broadcast, which is the whole-link cost-out. Note: LDP session events are per interface, so the negative cannot name WHICH peer came up; it proves no partial cost-in. Revert records on applyLDPSyncOverride.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L369) | unit/verify | unproven |
| negative | [`TestRFC5443BroadcastOnePeerUpKeepsSegmentCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L366) | unit/verify | unproven |
| positive | [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L156) | unit/verify | revert, verified |

### [`RFC5443-4-1`](#rfc5443-4-1)

The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. Level: RFC 5443 section 4 reads lowercase 'should only be applied to the IP link cost'; under D-3 a lowercase keyword keeps the row's MUST level, so no correction is owed. TestRFC5443CostOutRaisesIPCostNotDefaultTEMetric originates both LSAs from one not-synchronized snapshot: the Router-LSA link is 0xFFFF and the TE Link LSA (teOriginateType1, decoded) keeps the TE metric defaulted from the configured IP cost, 100. TestRFC5443CostOutLeavesExplicitTEMetric covers the explicit te-metric branch (50 unchanged). Both branches of applyTELinkAttributes are covered and a TE metric raised by the cost-out goes red. The false 'Ze originates no TE LSA' tags were removed from TestLDPSyncTECostUntouched.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5443CostOutLeavesExplicitTEMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L226) | unit/verify | revert, verified |
| positive | [`TestRFC5443CostOutRaisesIPCostNotDefaultTEMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5443_origination_test.go#L208) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5443.txt |
| Source fingerprint | 2400d8b588b794a6 |
| Record | rfc/extraction/rfc5443.json |
| Mapped sentences | 0 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 problem statement, describing why an L2/L3 VPN service depends on an edge-to-edge LSP. The lowercase 'must have been exchanged' states a property of the network the document then sets out to protect, not an obligation on an implementation: the sentence is followed by 'If only one link along the IP shortest path is not covered by an LDP session, a blackhole exists'. Section 2 carries the obligations this document places on a router. | This means that all the links along the IP shortest path from one PE router to the other need to have operational LDP sessions, and the necessary label binding must have been exchanged over those sessions. |

## Superseded

No document obsoletes RFC 5443, so its obligations are stated where they were written.
