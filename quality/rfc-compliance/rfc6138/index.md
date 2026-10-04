# RFC 6138 - LDP IGP Synchronization for Broadcast Networks

No row in the public ledger. Every requirement this repository extracted from RFC 6138, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 50.0% | 4 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 2 | of 2 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 2 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 2 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc6138.md` |
| Requirement shard | `rfc/requirements/rfc6138.md` |
| RFC text | `rfc/full/rfc6138.txt` |

## Enrolment

Enrolled: LDP IGP Synchronization for Broadcast Networks: two MUSTs, both tested with both polarities over ze's OSPF LDP-IGP-sync (internal/plugins/ospf/ldp_sync.go, spf/cutedge.go). RFC6138-4-1 (a cut-edge's LSA is not delayed by LDP): ldpSyncWithholdTransit returns false for a cut-edge even when not synchronized, while a non-cut-edge is withheld (TestLDPSyncTECostUntouched). RFC6138-x-1 (a pending SPF is executed before the cut-edge check): IsCutEdge flushes a scheduled-but-pending SPF and answers from the fresh graph, and the SPF does not run prematurely before that query (TestLDPSyncCutEdgeUsesFreshSPF). No gap, no ledger change.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6138.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC6138-4-1`](#rfc6138-4-1), [`RFC6138-x-1`](#rfc6138-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6138-4-1` | If the interface is a "cut-edge", then the updating of the LSA MUST NOT be delayed by LDP's operational state. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L360). **positive:** `unit/verify` [`TestRFC6138CutEdgeTransitLinkNotDelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L185). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L363). **negative:** `unit/verify` [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L161) |
| `RFC6138-x-1` | If an SPF run was scheduled but is pending execution, that SPF MUST be executed immediately before any procedure checks whether an interface is a "cut-edge". (§A) | MUST | A | **positive:** `unit/verify` [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L57). **positive:** `unit/verify` [`TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L61). **negative:** `unit/verify` [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L60). **negative:** `unit/verify` [`TestRFC6138PendingSPFNeverLeavesStaleCutEdge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L74) |

## Gaps and untested MUSTs

RFC 6138 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6138-4-1`](#rfc6138-4-1)

If the interface is a "cut-edge", then the updating of the LSA MUST NOT be delayed by LDP's operational state. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). The decision now reaches the originated LSA through the real SPF graph. Positive TestRFC6138CutEdgeTransitLinkNotDelayed: not synchronized, no alternate path in the SPF computer (cut-edge) -> production applyLDPSyncOverride plus OriginateRouter keep exactly 1 transit link. Negative TestRFC5443BroadcastCostOutCoversWholeSegment: the same not-synchronized segment with an alternate path (not a cut-edge) has its transit link withheld, so the cut-edge answer and not LDP alone decides. Revert records on spf IsCutEdge, observed red. Test race in the rfc6138SPF helper (field swap under the engine's own SPF goroutine) fixed by stopping the engine computer first; not a product race.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L363) | unit/verify | unproven |
| negative | [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L360) | unit/verify | unproven |
| positive | [`TestRFC6138CutEdgeTransitLinkNotDelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L185) | unit/verify | revert, verified |

### [`RFC6138-x-1`](#rfc6138-x-1)

If an SPF run was scheduled but is pending execution, that SPF MUST be executed immediately before any procedure checks whether an interface is a "cut-edge". (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c28). New spf/rfc6138_cutedge_pending_spf_test.go drives the Computer over a real LSDB: SPF run, the topology changed with newer Router-LSAs, a one-hour SPF armed through TriggerArea, then IsCutEdge on LAN 10.0.0.254. + TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss: parallel p2p withdrawn while pending -> true (stale graph says false); - TestRFC6138PendingSPFNeverLeavesStaleCutEdge: p2p pair added while pending -> false (stale graph says true), so a stale answer fails in each direction. Judge overlay: IsCutEdge reads lastGraphs before flushing -> both new units red with the stale answer (and the HEAD unit). Revert records on flushPendingSPF observed. The HEAD 'negative' tag on TestLDPSyncCutEdgeUsesFreshSPF#2 is that unit's precondition (snapshot empty before the query) and violates nothing; it is supplementary, the genuine negative is the new unit. The arming precondition in the new helper compares SPF state counts, which are equal in both topologies, so it is not load-bearing; the overlays prove the run stayed pending.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6138PendingSPFNeverLeavesStaleCutEdge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L74) | unit/verify | revert, verified |
| negative | [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L60) | unit/verify | unproven |
| positive | [`TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L57) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6138.txt |
| Source fingerprint | 5a82958400e2e887 |
| Record | rfc/extraction/rfc6138.json |
| Mapped sentences | 2 |
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
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 6138 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 6138, so its obligations are stated where they were written.
