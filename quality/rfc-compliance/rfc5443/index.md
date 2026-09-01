# RFC 5443 - LDP IGP Synchronization

Experimental. Every requirement this repository extracted from RFC 5443, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 83.3% | 5 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 16.7% | 1 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 14 |
| Gated MUST-level | 8 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 0 |
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
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5443-2-1`](#rfc5443-2-1), [`RFC5443-2-2`](#rfc5443-2-2), [`RFC5443-2-5`](#rfc5443-2-5), [`RFC5443-3-1`](#rfc5443-3-1), [`RFC5443-4-1`](#rfc5443-4-1)

**Annotated instead of tested (3):** [`RFC5443-2-3`](#rfc5443-2-3), [`RFC5443-2-4`](#rfc5443-2-4), [`RFC5443-2-6`](#rfc5443-2-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5443-2-1` | While LDP is not fully operational on a link, the IGP advertises that link with maximum cost to avoid transit traffic ("when LDP is not 'fully operational' ... on a given link, the IGP will advertise the link with maximum cost to avoid any transit traffic over it") (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L143). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L186) |
| `RFC5443-2-2` | In OSPF, the maximum cost advertised is LSInfinity, the 16-bit value `0xFFFF` ("In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]") (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L156). **negative:** `unit/verify` [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L292) |
| `RFC5443-2-3` | In IS-IS, the maximum metric advertised is `2^24-2` (`0xFFFFFE`) ("In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE)") (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements RFC 5443 LDP-IGP sync only in OSPF (internal/plugins/ospf/ldp_sync.go); IS-IS has no LDP-IGP sync state machine, so there is no IS-IS 2^24-2 max-metric cost-out producer (internal/plugins/isis defines only the generic MaxMetric topology-removal value) |
| `RFC5443-2-4` | Do not advertise the IS-IS link at `2^24-1` (the per-RFC-5305 maximum link metric), because that removes the link from the topology and loses the last-resort IP path ("if a link is configured with 2^24-1 ... then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort") (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no IS-IS LDP-IGP sync (internal/plugins/isis has no sync state machine), so it never originates an LDP-sync-driven IS-IS metric and cannot misuse the 2^24-1 value |
| `RFC5443-2-5` | Treat LDP as fully operational on a link only when all three conditions hold: an LDP hello adjacency exists, a suitable associated LDP session matching the hello adjacency's LDP Identifier is established to the peer at the other end of the link, and all label bindings have been exchanged over the session ("LDP is considered fully operational on a link when an LDP hello adjacency exists on it, a suitable associated LDP session ... is established ... and all label bindings have been exchanged over the session") (§2) | MUST | 2 | **positive:** `unit/verify` [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L165). **negative:** `unit/verify` [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188) |
| `RFC5443-2-6` | When LDP End-of-LIB is implemented, consider the neighbor LDP session fully operational only upon receipt of the End-of-LIB notification message ("The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received") (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the precondition is unmet -- ze's LDP (internal/plugins/ldp) implements no End-of-LIB notification, so ze uses the RFC 5443 hold-down-estimate alternative instead |
| `RFC5443-3-1` | On broadcast links with more than one IGP/LDP peer, apply the cost-out procedure to the link as a whole, not to an individual peer ("the cost-out procedure can only be applied to the link as a whole and not to an individual peer") (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L370). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L373) |
| `RFC5443-4-1` | Apply the cost-raising mechanism only to the IP link cost, not the TE link cost ("The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes") (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L364). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L367) |
| `RFC5443-3-2` | When a genuine link problem (not merely link bring-up) causes the cost-out, the implementation should issue network management alerts so the operator can address the condition ("an implementation should issue network management alerts to report the error condition and enable the operator to address it") (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-5-1` | Follow current best security practice for MPLS/GMPLS networks ("implementors should follow the current best security practice [MPLS-GMPLS-Sec]") (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-2-7` | Use a configurable hold-down timer after LDP session establishment as the estimation strategy for "all label bindings exchanged" when End-of-LIB is not available ("A simple implementation strategy is to use a configurable hold-down timer to allow LDP session establishment before declaring LDP fully operational") (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-2-8` | Omit the hold-down timer entirely when LDP End-of-LIB is implemented ("When LDP End-of-LIB is implemented, the configurable hold-down timer is no longer needed") (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-3-3` | As a policy decision on broadcast links, divert traffic away from all peers on the link when LDP service to one peer is unavailable ("a policy decision has to be made whether the unavailability of LDP service to one peer should result in the traffic being diverted away from all the peers on the link") (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5443-4-2` | Raise the IP cost of a TE tunnel while there is no operational targeted LDP session between tunnel endpoints ("raising the IP cost of the tunnel while there is no operational LDP session will solve the problem") (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5443-2-3`](#rfc5443-2-3) In IS-IS, the maximum metric advertised is `2^24-2` (`0xFFFFFE`) ("In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE)") (§2) | {gap}, no test | ze implements RFC 5443 LDP-IGP sync only in OSPF (internal/plugins/ospf/ldp_sync.go); IS-IS has no LDP-IGP sync state machine, so there is no IS-IS 2^24-2 max-metric cost-out producer (internal/plugins/isis defines only the generic MaxMetric topology-removal value) |
| [`RFC5443-2-4`](#rfc5443-2-4) Do not advertise the IS-IS link at `2^24-1` (the per-RFC-5305 maximum link metric), because that removes the link from the topology and loses the last-resort IP path ("if a link is configured with 2^24-1 ... then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort") (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no IS-IS LDP-IGP sync (internal/plugins/isis has no sync state machine), so it never originates an LDP-sync-driven IS-IS metric and cannot misuse the 2^24-1 value |
| [`RFC5443-2-6`](#rfc5443-2-6) When LDP End-of-LIB is implemented, consider the neighbor LDP session fully operational only upon receipt of the End-of-LIB notification message ("The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received") (§2) | no test | no test carries this requirement id; annotated {not-applicable}: the precondition is unmet -- ze's LDP (internal/plugins/ldp) implements no End-of-LIB notification, so ze uses the RFC 5443 hold-down-estimate alternative instead |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5443-2-1`](#rfc5443-2-1)

While LDP is not fully operational on a link, the IGP advertises that link with maximum cost to avoid transit traffic ("when LDP is not 'fully operational' ... on a given link, the IGP will advertise the link with maximum cost to avoid any transit traffic over it") (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L186) | unit/verify | unproven |
| positive | [`TestLDPSyncForcesMaxMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L143) | unit/verify | unproven |

### [`RFC5443-2-2`](#rfc5443-2-2)

In OSPF, the maximum cost advertised is LSInfinity, the 16-bit value `0xFFFF` ("In the case of OSPF, this cost is LSInfinity (16-bit value 0xFFFF), as proposed in [RFC3137]") (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncDisabledIsNoOp`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L292) | unit/verify | unproven |
| positive | [`TestLDPSyncMaxMetricValue`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L156) | unit/verify | unproven |

### [`RFC5443-2-3`](#rfc5443-2-3)

In IS-IS, the maximum metric advertised is `2^24-2` (`0xFFFFFE`) ("In the case of ISIS, the maximum metric value is 2^24-2 (0xFFFFFE)") (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-3, so no unit is bound to it.

### [`RFC5443-2-4`](#rfc5443-2-4)

Do not advertise the IS-IS link at `2^24-1` (the per-RFC-5305 maximum link metric), because that removes the link from the topology and loses the last-resort IP path ("if a link is configured with 2^24-1 ... then this link is not advertised in the topology. It is important to keep the link in the topology to allow IP traffic to use the link as a last resort") (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-4, so no unit is bound to it.

### [`RFC5443-2-5`](#rfc5443-2-5)

Treat LDP as fully operational on a link only when all three conditions hold: an LDP hello adjacency exists, a suitable associated LDP session matching the hello adjacency's LDP Identifier is established to the peer at the other end of the link, and all label bindings have been exchanged over the session ("LDP is considered fully operational on a link when an LDP hello adjacency exists on it, a suitable associated LDP session ... is established ... and all label bindings have been exchanged over the session") (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncRestoresAfterHoldDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L188) | unit/verify | unproven |
| positive | [`TestLDPSyncSubscribesSessionEvents`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L165) | unit/verify | unproven |

### [`RFC5443-2-6`](#rfc5443-2-6)

When LDP End-of-LIB is implemented, consider the neighbor LDP session fully operational only upon receipt of the End-of-LIB notification message ("The neighbor LDP session is considered fully operational when the End-of-LIB notification message is received") (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5443-2-6, so no unit is bound to it.

### [`RFC5443-3-1`](#rfc5443-3-1)

On broadcast links with more than one IGP/LDP peer, apply the cost-out procedure to the link as a whole, not to an individual peer ("the cost-out procedure can only be applied to the link as a whole and not to an individual peer") (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L373) | unit/verify | unproven |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L370) | unit/verify | unproven |

### [`RFC5443-4-1`](#rfc5443-4-1)

Apply the cost-raising mechanism only to the IP link cost, not the TE link cost ("The mechanism described in this document should only be applied to the IP link cost to prevent unnecessary TE tunnel reroutes") (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L367) | unit/verify | unproven |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L364) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5443, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5443, so its obligations are stated where they were written.
