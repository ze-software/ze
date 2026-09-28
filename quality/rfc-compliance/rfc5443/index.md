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
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Audit verdicts | 5 | of 8 gated MUSTs judged | 5 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 5 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5443-2-1`](#rfc5443-2-1), [`RFC5443-2-2`](#rfc5443-2-2), [`RFC5443-2-5`](#rfc5443-2-5), [`RFC5443-3-1`](#rfc5443-3-1), [`RFC5443-4-1`](#rfc5443-4-1)

**Annotated instead of tested (3):** [`RFC5443-2-3`](#rfc5443-2-3), [`RFC5443-2-4`](#rfc5443-2-4), [`RFC5443-2-6`](#rfc5443-2-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5443-2-1` | In detail: when LDP is not "fully operational" (see below) on a given link, the IGP will advertise the link with maximum cost to avoid any transit traffic over it. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L145). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188) |
| `RFC5443-2-2` | In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L158). **negative:** `unit/verify` [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L294) |
| `RFC5443-2-3` | In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE). (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements RFC 5443 LDP-IGP sync only in OSPF (internal/plugins/ospf/ldp_sync.go); IS-IS has no LDP-IGP sync state machine, so there is no IS-IS 2^24-2 max-metric cost-out producer (internal/plugins/isis defines only the generic MaxMetric topology-removal value) |
| `RFC5443-2-4` | Indeed, if a link is configured with 2^24-1 (the maximum link metric per [RFC5305]), then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort in case of massive failure. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no IS-IS LDP-IGP sync (internal/plugins/isis has no sync state machine), so it never originates an LDP-sync-driven IS-IS metric and cannot misuse the 2^24-1 value |
| `RFC5443-2-5` | LDP is considered fully operational on a link when an LDP hello adjacency exists on it, a suitable associated LDP session (matching the LDP Identifier of the hello adjacency) is established to the peer at the other end of the link, and all label bindings have been exchanged over the session. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L167). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L190) |
| `RFC5443-2-6` | The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the precondition is unmet -- ze's LDP (internal/plugins/ldp) implements no End-of-LIB notification, so ze uses the RFC 5443 hold-down-estimate alternative instead |
| `RFC5443-3-1` | On broadcast links with more than one IGP/LDP peer, the cost-out procedure can only be applied to the link as a whole and not to an individual peer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L372). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L375) |
| `RFC5443-4-1` | The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L366). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L369) |
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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Non-compliant: the originated router-LSA keeps the configured cost while LDP is not fully operational. No tagged assertion reads an originated LSA: TestLDPSyncForcesMaxMetric and TestLDPSyncRestoresAfterHoldDown assert effectiveP2PCost, whose only non-test caller is the show snapshot (ldp_sync.go:664); the advertised cost comes from applyLDPSyncOverride setting InterfaceInfo.LDPSyncMaxMetric and lsdb/origination.go:245/279 reading it. Deleting that flag assignment leaves every tag green. Broadcast links are withheld (RFC 6138 style), not costed out, and no tag covers that for this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188) | unit/verify | unproven |
| positive | [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L145) | unit/verify | unproven |

### [`RFC5443-2-2`](#rfc5443-2-2)

In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]. (§2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Non-compliant: advertising a max cost other than 0xFFFF. TestLDPSyncMaxMetricValue asserts only the constant ospflsdb.LSInfinity == 0xFFFF; no tagged assertion reads the metric the origination emits for an LDPSyncMaxMetric link. The negative (TestLDPSyncDisabledIsNoOp) proves an unmanaged interface keeps its cost, a neighbouring rule, not the value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L294) | unit/verify | unproven |
| positive | [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L158) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Three clauses: hello adjacency exists, a session matching the adjacency's LDP Identifier is established, all bindings exchanged. Tagged units (TestLDPSyncSubscribesSessionEvents, TestLDPSyncRestoresAfterHoldDown) drive the real ldpSyncManager: SessionUp -> hold-down, timer -> synchronized, so a machine declaring sync on SessionUp goes red. No assertion covers the hello-adjacency or the LDP-Identifier-match clause: a session-up for a session not matching the link's adjacency is never exercised.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L190) | unit/verify | unproven |
| positive | [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L167) | unit/verify | unproven |

### [`RFC5443-2-6`](#rfc5443-2-6)

The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-6, so no unit is bound to it.

### [`RFC5443-3-1`](#rfc5443-3-1)

On broadcast links with more than one IGP/LDP peer, the cost-out procedure can only be applied to the link as a whole and not to an individual peer. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Non-compliant: a per-peer cost-out on a broadcast link. TestLDPSyncTECostUntouched asserts ldpSyncWithholdTransit(notSynced, managed, cutEdge=false) is true and cut-edge false; the function takes no peer input, so the whole-link granularity is structural and no assertion would go red on a per-neighbour withhold introduced in applyLDPSyncOverride. The negative/positive both test RFC 6138 cut-edge logic rather than peer granularity.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L375) | unit/verify | unproven |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L372) | unit/verify | unproven |

### [`RFC5443-4-1`](#rfc5443-4-1)

The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes. (§4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The row forbids raising the TE link cost. TestLDPSyncTECostUntouched asserts effectiveP2PCost == LSInfinity (the IP cost override, RFC5443-2-1) and the cut-edge withhold (RFC 6138 section 4); no assertion reads any TE metric. The tag prose says ze originates no TE LSA, yet interface_cost_test.go has TestDerivedCostReachesLDPSyncAndTEMetric, so that claim needs checking. Both tags prove neighbouring rules.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L369) | unit/verify | unproven |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L366) | unit/verify | unproven |

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
