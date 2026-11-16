# RFC 6138 - LDP IGP Synchronization for Broadcast Networks

No entry in the public support list. Every requirement listed for RFC 6138, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 2 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 5 | 5 of 7 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 2 | of 2 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 2 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Included |
| Requirements | 2 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tests linked to requirements | 7 |
| Recorded audit verdicts | 2 |
| Fault test records | 5 |
| Summary | `rfc/short/rfc6138.md` |
| Requirement table | `rfc/requirements/rfc6138.md` |
| RFC text | `rfc/full/rfc6138.txt` |

## Automatic checks

Included in automatic checks: LDP IGP Synchronization for Broadcast Networks: two MUSTs, both tested with both polarities over ze's OSPF LDP-IGP-sync (internal/plugins/ospf/ldp_sync.go, spf/cutedge.go). RFC6138-4-1 (a cut-edge's LSA is not delayed by LDP): ldpSyncWithholdTransit returns false for a cut-edge even when not synchronized, while a non-cut-edge is withheld (TestLDPSyncTECostUntouched). RFC6138-x-1 (a pending SPF is executed before the cut-edge check): IsCutEdge flushes a scheduled-but-pending SPF and answers from the fresh graph, and the SPF does not run prematurely before that query (TestLDPSyncCutEdgeUsesFreshSPF). No gap, no ledger change.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6138.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC6138-4-1`](#rfc6138-4-1), [`RFC6138-x-1`](#rfc6138-x-1)

## Figures

- **MUST bar**: Ze verified 2
- **Overview bar**: Ze MUST verified 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze verified | 2 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6138.json](../../../data/rfc/rfc6138.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6138-4-1` | If the interface is a "cut-edge", then the updating of the LSA MUST NOT be delayed by LDP's operational state. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L375). **positive:** `unit/verify` [`TestRFC6138CutEdgeTransitLinkNotDelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L185). **negative:** `unit/verify` [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L378). **negative:** `unit/verify` [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L161) |
| `RFC6138-x-1` | If an SPF run was scheduled but is pending execution, that SPF MUST be executed immediately before any procedure checks whether an interface is a "cut-edge". (§A) | MUST | A | **positive:** `unit/verify` [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L57). **positive:** `unit/verify` [`TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L61). **negative:** `unit/verify` [`TestRFC6138PendingSPFNeverLeavesStaleCutEdge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L74) |

## Gaps and untested MUSTs

RFC 6138 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6138-4-1`](#rfc6138-4-1)

If the interface is a "cut-edge", then the updating of the LSA MUST NOT be delayed by LDP's operational state. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. Positive TestRFC6138CutEdgeTransitLinkNotDelayed: not synchronized, no alternate path, the originated LSA keeps exactly 1 transit link. Negative TestRFC5443BroadcastCostOutCoversWholeSegment: the same not-synchronized segment with an alternate path is withheld. Records on IsCutEdge. TestLDPSyncTECostUntouched also pins ldpSyncWithholdTransit for cut-edge and non-cut-edge directly.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L378) | unit/verify | not verified |
| negative | [`TestRFC5443BroadcastCostOutCoversWholeSegment`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestLDPSyncTECostUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ldp_sync_test.go#L375) | unit/verify | not verified |
| positive | [`TestRFC6138CutEdgeTransitLinkNotDelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6138_broadcast_origination_test.go#L185) | unit/verify | revert, verified |

### [`RFC6138-x-1`](#rfc6138-x-1)

If an SPF run was scheduled but is pending execution, that SPF MUST be executed immediately before any procedure checks whether an interface is a "cut-edge". (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. The precondition negative (SPF not yet run) is gone. Positive TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss: the P2P links are withdrawn with a one-hour SPF pending, IsCutEdge answers true from the pending graph; negative TestRFC6138PendingSPFNeverLeavesStaleCutEdge: the reverse change answers false, never the stale true. TestLDPSyncCutEdgeUsesFreshSPF: a nil graph would answer true. Records on flushPendingSPF.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC6138PendingSPFNeverLeavesStaleCutEdge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC6138PendingSPFRunBeforeCutEdgeAfterLinkLoss`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_cutedge_pending_spf_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestLDPSyncCutEdgeUsesFreshSPF`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc6138_ldp_sync_cutedge_test.go#L57) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 1 | reviewed | not stated |
| `B` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 6138 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 6138, so its obligations are stated where they were written.
