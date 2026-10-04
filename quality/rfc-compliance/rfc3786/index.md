# RFC 3786 - Extending the Number of Intermediate System to Intermediate System (IS-IS) Link State PDU (LSP) Fragments Beyond the 256 Limit

No row in the public ledger. Every requirement this repository extracted from RFC 3786, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 22.2% | 2 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 60.0% | 3 of 5 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 77.8% | 7 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 5 |
| Tagged units | 5 |
| Recorded audit verdicts | 4 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc3786.md` |
| Requirement shard | `rfc/requirements/rfc3786.md` |
| RFC text | `rfc/full/rfc3786.txt` |

## Enrolment

Enrolled: Extending the Number of IS-IS LSP Fragments Beyond the 256 Limit: seven MUST-level requirements after the 2026-09-21 extraction walk, eight after the 2026-09-28 split of RFC3786-5-1. Four are {not-applicable} to Ze, five after that split; the three the walk added carry no test and no annotation: RFC3786-1.3-1 (all LSPs in one SPF instance use the same Mode, §1.3), RFC3786-3.2-1 (a Mode 1 Extended LSP names the Normal system-id as a neighbor, §3.2) and RFC3786-6-1 (an IS uses the system-id of the LSP that will include a neighbor when forming an adjacency with it, §6). Ze does not implement the RFC 3786 extended-LSP-fragment mechanism -- its IS-IS codec recognizes no IS Alias ID TLV (type 24) and originates no extended LSP sets or virtual system (recognized TLV set 1/2/6/8/9/10/22/129/132/135/137/232/236/240 in internal/plugins/isis/packet/tlv.go), running only standard ISO/IEC 10589 LSPs bounded by the 256-fragment limit. RFC3786-2-1 (IS Alias ID TLV in fragment 0) and RFC3786-3.1-1 (generate extended fragment zero) have no extended-LSP origination path; RFC3786-5-2 (SPF exclusion of an expired extended fragment 0 set) governs extended sets Ze does not have (its SPF operates only on standard LSPs), while RFC3786-5-1 (the Original set is excluded when its fragment 0 is missing or expired) applies to Ze's standard fragment sets and is tested; RFC3786-x-2 (Mode 1 Originating-to-Virtual adjacency zero metric) governs a virtual-system model Ze does not implement. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 3786.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC3786-5-1`](#rfc3786-5-1), [`RFC3786-6-1`](#rfc3786-6-1)

**Annotated (including scoped evidence) (7):** [`RFC3786-2-1`](#rfc3786-2-1), [`RFC3786-3.1-1`](#rfc3786-3.1-1), [`RFC3786-5-2`](#rfc3786-5-2), [`RFC3786-x-2`](#rfc3786-x-2), [`RFC3786-3.2.1-1`](#rfc3786-3.2.1-1), [`RFC3786-1.3-1`](#rfc3786-1.3-1), [`RFC3786-3.2-1`](#rfc3786-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3786-2-1` | This TLV MUST be included in fragment 0 of every LSP set belonging to an Originating System running in either Mode 1 or Mode 2. (Section 2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/lsdb/origination.go maxFragments is 256 and no IS Alias ID producer or mode configuration exists; the "in either mode" condition is absent. |
| `RFC3786-3.1-1` | An extended LSP fragment zero MUST be generated for every extended LSP set, to allow a router's SPF calculation to consider those fragments in that set. (Section 3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 leaves both extension modes disabled without explicit configuration. internal/plugins/isis/lsdb/origination.go produces one standard set per source with maxFragments=256 and no additional-system-ID or IS Alias ID producer, so no extended set is generated. |
| `RFC3786-5-1` | If LSP fragment 0 of the Original LSP set is missing or its RemainingLifetime is zero, all of the LSPs generated by that Originating System (Extended as well) MUST NOT be considered in the SPF. That is, the large logical LSP is not considered in the SPF. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC1195StandardFragmentsRequireFragmentZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L384). **negative:** `unit/verify` [`TestRFC1195StandardFragmentsRequireFragmentZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L385). **negative:** `unit/verify` [`TestRFC3786ExpiredFragmentZeroExcludesSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L262) |
| `RFC3786-5-2` | If an LSP fragment 0 of an extended LSP set is missing or its RemainingLifetime is zero, only that LSP set MUST NOT be considered in the SPF. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/lsdb/origination.go maxFragments is 256 and no IS Alias ID TLV (type 24) is decoded or originated, so Ze holds no extended LSP set for SPF to exclude; internal/plugins/isis/spf_wiring.go Records reads standard source sets only. |
| `RFC3786-x-1` | The Attached (ATT) bits SHOULD be set to zero for all four metric types, on all Extended LSPs. (§3.1.1) | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3786-3.1.4-1` | The overload bit should be set consistently across all LSPs, original and extended, belonging to an Originating System, and should reflect the Originating System's overload state. (§3.1.4) | SHOULD | 3.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3786-x-2` | Under Operation Mode 1, adjacencies from the Originating System to its Virtual Systems are advertised using the standard neighbor TLVs. The metric for these connections MUST be zero, since the cost of reaching a Virtual System is the same as the cost of reaching its Originating System. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 requires explicit configuration to enable either mode; no Mode 1 configuration, virtual-system adjacency, or extended-LSP producer exists in internal/plugins/isis/config.go or lsdb/origination.go. |
| `RFC3786-3.2.1-1` | Other neighbors MUST NOT be specified in an Extended LSP, because those other neighbors would only specify the Originating System and not the Virtual System, and hence would not satisfy the bi- directionality check in the SPF computation. (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 requires explicit configuration to enable either mode; no Mode 1 configuration, virtual-system adjacency, or extended-LSP producer exists in internal/plugins/isis/config.go or lsdb/origination.go. |
| `RFC3786-3.1.2-1` | The Partition Repair (P) bit SHOULD be set to zero on all extended LSPs. (§3.1.2) | SHOULD | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3786-1.3-1` | That is, all LSPs considered in the same SPF instance MUST use the same Mode. (Section 1.3) | MUST | 1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/config.go has no extension-mode selection; lsdb/origination.go maxFragments=256 and spf_wiring.go reads standard source sets only. There are no Mode 1/Mode 2 inputs to mix in one SPF instance. |
| `RFC3786-3.2-1` | Furthermore, the Extended LSP MUST specify the Normal system-id as a neighbor. The metric SHOULD be set to MaxLinkMetric - 1 (this is only for uniformity purpose, any metric greater than zero is acceptable). This in order to satisfy the two-way connectivity check on other routers. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 leaves Mode 1 disabled without explicit configuration; internal/plugins/isis/lsdb/origination.go has no extended-set or virtual-system producer. There is no Mode 1 extended LSP requiring a backlink. |
| `RFC3786-6-1` | It should be noted, that an IS MUST use the system-id of the LSP that will include a neighbor, when forming an adjacency with that neighbor. That is, if a neighbor is to be included in extended LSP S', then S' should be used as the system-id in IS Hellos [3] and IS- IS Hellos when forming an adjacency with that neighbor. This is regardless of the Operational Mode. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC3786NormalHelloMatchesContainingLSP`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3786_hello_identity_test.go#L91). **negative:** `unit/verify` [`TestRFC3786NormalHelloIdentityReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3786_hello_identity_test.go#L98) |
| `RFC3786-7-1` | Implementations SHOULD support a configuration parameter controlling the LSP origination behavior. The default value of this parameter SHOULD correspond to the behavior described in [ISIS-ISO], i.e., neither of the two modes described in this document should be enabled without explicit configuration when the router software is upgraded with this extension. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3786-2-1`](#rfc3786-2-1) This TLV MUST be included in fragment 0 of every LSP set belonging to an Originating System running in either Mode 1 or Mode 2. (Section 2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/lsdb/origination.go maxFragments is 256 and no IS Alias ID producer or mode configuration exists; the "in either mode" condition is absent. |
| [`RFC3786-3.1-1`](#rfc3786-3.1-1) An extended LSP fragment zero MUST be generated for every extended LSP set, to allow a router's SPF calculation to consider those fragments in that set. (Section 3.1) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 leaves both extension modes disabled without explicit configuration. internal/plugins/isis/lsdb/origination.go produces one standard set per source with maxFragments=256 and no additional-system-ID or IS Alias ID producer, so no extended set is generated. |
| [`RFC3786-5-2`](#rfc3786-5-2) If an LSP fragment 0 of an extended LSP set is missing or its RemainingLifetime is zero, only that LSP set MUST NOT be considered in the SPF. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/lsdb/origination.go maxFragments is 256 and no IS Alias ID TLV (type 24) is decoded or originated, so Ze holds no extended LSP set for SPF to exclude; internal/plugins/isis/spf_wiring.go Records reads standard source sets only. |
| [`RFC3786-x-2`](#rfc3786-x-2) Under Operation Mode 1, adjacencies from the Originating System to its Virtual Systems are advertised using the standard neighbor TLVs. The metric for these connections MUST be zero, since the cost of reaching a Virtual System is the same as the cost of reaching its Originating System. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 requires explicit configuration to enable either mode; no Mode 1 configuration, virtual-system adjacency, or extended-LSP producer exists in internal/plugins/isis/config.go or lsdb/origination.go. |
| [`RFC3786-3.2.1-1`](#rfc3786-3.2.1-1) Other neighbors MUST NOT be specified in an Extended LSP, because those other neighbors would only specify the Originating System and not the Virtual System, and hence would not satisfy the bi- directionality check in the SPF computation. (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 requires explicit configuration to enable either mode; no Mode 1 configuration, virtual-system adjacency, or extended-LSP producer exists in internal/plugins/isis/config.go or lsdb/origination.go. |
| [`RFC3786-1.3-1`](#rfc3786-1.3-1) That is, all LSPs considered in the same SPF instance MUST use the same Mode. (Section 1.3) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 says neither mode should be enabled without explicit configuration. internal/plugins/isis/config.go has no extension-mode selection; lsdb/origination.go maxFragments=256 and spf_wiring.go reads standard source sets only. There are no Mode 1/Mode 2 inputs to mix in one SPF instance. |
| [`RFC3786-3.2-1`](#rfc3786-3.2-1) Furthermore, the Extended LSP MUST specify the Normal system-id as a neighbor. The metric SHOULD be set to MaxLinkMetric - 1 (this is only for uniformity purpose, any metric greater than zero is acceptable). This in order to satisfy the two-way connectivity check on other routers. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep standard fragment sets". Section 7 leaves Mode 1 disabled without explicit configuration; internal/plugins/isis/lsdb/origination.go has no extended-set or virtual-system producer. There is no Mode 1 extended LSP requiring a backlink. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3786-2-1`](#rfc3786-2-1)

This TLV MUST be included in fragment 0 of every LSP set belonging to an Originating System running in either Mode 1 or Mode 2. (Section 2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-2-1, so no unit is bound to it.

### [`RFC3786-3.1-1`](#rfc3786-3.1-1)

An extended LSP fragment zero MUST be generated for every extended LSP set, to allow a router's SPF calculation to consider those fragments in that set. (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-3.1-1, so no unit is bound to it.

### [`RFC3786-5-1`](#rfc3786-5-1)

If LSP fragment 0 of the Original LSP set is missing or its RemainingLifetime is zero, all of the LSPs generated by that Originating System (Extended as well) MUST NOT be considered in the SPF. That is, the large logical LSP is not considered in the SPF. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row narrowed to the Original-set sentences. Missing fragment 0: TestRFC1195StandardFragmentsRequireFragmentZero requires the fragment-1 route while fragment 0 lives (2, 15) and no route after fragment 0 is deleted. RemainingLifetime zero: TestRFC3786ExpiredFragmentZeroExcludesSet stores a zero-lifetime fragment 0, asserts it is still held by the LSDB, and requires no route although fragment 1 still carries the link back and the prefix. Whole-stack: RawSnapshot drops zero-lifetime entries and Records admits no fragment of a source without a live fragment 0. The extended-set clause is RFC3786-5-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3786ExpiredFragmentZeroExcludesSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L262) | unit/verify | revert, verified |
| negative | [`TestRFC1195StandardFragmentsRequireFragmentZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC1195StandardFragmentsRequireFragmentZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L384) | unit/verify | revert, verified |

### [`RFC3786-5-2`](#rfc3786-5-2)

If an LSP fragment 0 of an extended LSP set is missing or its RemainingLifetime is zero, only that LSP set MUST NOT be considered in the SPF. (§5)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Binds an extended LSP set, which Ze neither originates nor recognizes (owner decision 2026-09-21, standard fragment sets).

No test carries RFC3786-5-2, so no unit is bound to it.

### [`RFC3786-x-2`](#rfc3786-x-2)

Under Operation Mode 1, adjacencies from the Originating System to its Virtual Systems are advertised using the standard neighbor TLVs. The metric for these connections MUST be zero, since the cost of reaching a Virtual System is the same as the cost of reaching its Originating System. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-x-2, so no unit is bound to it.

### [`RFC3786-3.2.1-1`](#rfc3786-3.2.1-1)

Other neighbors MUST NOT be specified in an Extended LSP, because those other neighbors would only specify the Originating System and not the Virtual System, and hence would not satisfy the bi- directionality check in the SPF computation. (§3.2.1)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Judged 2026-09-30. {not-applicable} accepted; the child spec constraint records x-2's split as a row-text decision.

No test carries RFC3786-3.2.1-1, so no unit is bound to it.

### [`RFC3786-1.3-1`](#rfc3786-1.3-1)

That is, all LSPs considered in the same SPF instance MUST use the same Mode. (Section 1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-1.3-1, so no unit is bound to it.

### [`RFC3786-3.2-1`](#rfc3786-3.2-1)

Furthermore, the Extended LSP MUST specify the Normal system-id as a neighbor. The metric SHOULD be set to MaxLinkMetric - 1 (this is only for uniformity purpose, any metric greater than zero is acceptable). This in order to satisfy the two-way connectivity check on other routers. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3786-3.2-1, so no unit is bound to it.

### [`RFC3786-6-1`](#rfc3786-6-1)

It should be noted, that an IS MUST use the system-id of the LSP that will include a neighbor, when forming an adjacency with that neighbor. That is, if a neighbor is to be included in extended LSP S', then S' should be used as the system-id in IS Hellos [3] and IS- IS Hellos when forming an adjacency with that neighbor. This is regardless of the Operational Mode. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: sending IS Hellos (ISH) or IS-IS Hellos with a system-id other than that of the LSP that contains the neighbor. assertNormalHelloIdentity first requires the neighbor in the node's normal LSP, then t.Fatalf when a captured ISH NET system-id or a P2P IIH SystemID differs from lsp.LSPID.SystemID(), and requires both to be seen. TestRFC3786NormalHelloMatchesContainingLSP is the positive; TestRFC3786NormalHelloIdentityReload reloads the NET twice and requires both hellos to follow, so a circuit keeping its old identity goes red. The S' (extended LSP) clause has no producer in Ze (no extended LSP sets), so the normal system-id is the only identity the MUST can require here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3786NormalHelloIdentityReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3786_hello_identity_test.go#L98) | unit/verify | unproven |
| positive | [`TestRFC3786NormalHelloMatchesContainingLSP`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc3786_hello_identity_test.go#L91) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3786.txt |
| Source fingerprint | cd5d17df37dc7f90 |
| Record | rfc/extraction/rfc3786.json |
| Mapped sentences | 9 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 1 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.1.3` | not stated | 0 | walked | not stated |
| `3.1.4` | not stated | 0 | walked | not stated |
| `3.1.5` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `3.2.1` | not stated | 1 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1 states the binding obligation in the abstract and the same paragraph names the carrier: 'Under both modes, the Originating System MUST include information binding the Original LSP and the Extended ones. ... This binding is advertised via a new IS Alias ID TLV, which is advertised in all fragment 0 of Original and Extended LSPs.' Site 2:1 states that same obligation concretely, 'This TLV MUST be included in fragment 0 of every LSP set belonging to an Originating System running in either Mode 1 or Mode 2', and maps to RFC3786-2-1. | Under both modes, the Originating System MUST include information binding the Original LSP and the Extended ones. |
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates, for the moment an Extended LSP is first created, the zero-metric Originating-to-Virtual adjacency that site 3.2:1 states in its own right: 'The metric for these connections MUST be zero' (Section 3.2) against 'When an Extended LSP belonging to Additional system-id S' is first created, the Original LSP MUST specify S' as a neighbor, with metric set to zero.' Both sentences describe the one adjacency RFC3786-x-2 carries, and the sentence after this one gives the same reason: 'This is in order to consider the cost of reaching the Virtual System S' the same as the cost of reaching its Originating System.' | When an Extended LSP belonging to Additional system-id S' is first created, the Original LSP MUST specify S' as a neighbor, with metric set to zero. |

## Superseded

No document obsoletes RFC 3786, so its obligations are stated where they were written.
