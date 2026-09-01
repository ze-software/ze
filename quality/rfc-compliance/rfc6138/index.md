# RFC 6138 - LDP IGP Synchronization for Broadcast Networks

No row in the public ledger. Every requirement this repository extracted from RFC 6138, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 2 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 2 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 2 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 2 |
| Gated MUST-level | 2 |
| Obligations that bind Ze | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC6138-4-1`](#rfc6138-4-1), [`RFC6138-x-1`](#rfc6138-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6138-4-1` | If the interface is a "cut-edge", updating of the LSA MUST NOT be delayed by LDP's operational state (the link is advertised immediately, regardless of LDP) (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L358). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L361) |
| `RFC6138-x-1` | If an SPF run was scheduled but is pending execution, that SPF must be executed immediately before any procedure checks whether an interface is a "cut-edge" (Appendix A) | MUST | x | **positive:** `unit/verify` [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/ldp_sync_cutedge_test.go#L57). **negative:** `unit/verify` [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/ldp_sync_cutedge_test.go#L60) |

## Gaps and untested MUSTs

RFC 6138 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6138-4-1`](#rfc6138-4-1)

If the interface is a "cut-edge", updating of the LSA MUST NOT be delayed by LDP's operational state (the link is advertised immediately, regardless of LDP) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L361) | unit/verify | unproven |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L358) | unit/verify | unproven |

### [`RFC6138-x-1`](#rfc6138-x-1)

If an SPF run was scheduled but is pending execution, that SPF must be executed immediately before any procedure checks whether an interface is a "cut-edge" (Appendix A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/ldp_sync_cutedge_test.go#L60) | unit/verify | unproven |
| positive | [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/ldp_sync_cutedge_test.go#L57) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 6138, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6138, so its obligations are stated where they were written.
