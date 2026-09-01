# RFC 3623 - Graceful OSPF Restart

Experimental. Every requirement this repository extracted from RFC 3623, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 36.4% | 4 of 11 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 45.5% | 5 of 11 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.2% | 2 of 11 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 11 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 26 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 11 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3623.md` |
| Requirement shard | `rfc/requirements/rfc3623.md` |
| RFC text | `rfc/full/rfc3623.txt` |

## Enrolment

Enrolled: Graceful OSPF Restart (RFC 3623): restarter + helper roles; 4 MET (Grace-LSA mandatory TLVs, shared-media type-3, unplanned toggle) + 5 single-polarity positive + 2 gap (virtual-link V-bit, changed-retx-list refusal) + 2 not-applicable

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

- Restarter (planned + opt-in unplanned) and helper behavior
- Grace-LSA Opaque type-3 body codec shared with RFC 5187.


**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc3623.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc3623.md): the helper does not preserve the transit-area V-bit when helping over a virtual link ([`RFC3623-3-1`](#rfc3623-3-1)), and helper entry does not refuse on a changed retransmission-list LSA ([`RFC3623-3.1-1`](#rfc3623-3.1-1), onGraceReceived is hardcoded permissive, mitigated by the Section 3.2 strict-LSA-checking exit). Experimental pending deployment hardening.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC3623-A-2`](#rfc3623-a-2), [`RFC3623-A-3`](#rfc3623-a-3), [`RFC3623-A-4`](#rfc3623-a-4), [`RFC3623-5-1`](#rfc3623-5-1)

**Annotated instead of tested (9):** [`RFC3623-A-1`](#rfc3623-a-1), [`RFC3623-A-5`](#rfc3623-a-5), [`RFC3623-2.1-1`](#rfc3623-2.1-1), [`RFC3623-5-2`](#rfc3623-5-2), [`RFC3623-5-3`](#rfc3623-5-3), [`RFC3623-5-4`](#rfc3623-5-4), [`RFC3623-3-1`](#rfc3623-3-1), [`RFC3623-3.1-1`](#rfc3623-3.1-1), [`RFC3623-3.2-1`](#rfc3623-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3623-A-1` | Additional Grace-LSA TLVs must be described in an Internet Draft and subject to OSPF WG expert review (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates only the three RFC-defined TLVs (types 1/2/3) and adds none, and its decoder ignores unrecognized types, so the IETF-process obligation for additional TLVs binds no ze behavior (internal/plugins/ospf/packet/grace_lsa.go:44, :64) |
| `RFC3623-A-2` | Grace Period TLV (type 1) must always appear in a grace-LSA (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L20). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L75) |
| `RFC3623-A-3` | Graceful restart reason TLV (type 2) must always appear in a grace-LSA (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L24). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L72) |
| `RFC3623-A-4` | IP interface address TLV (type 3) required on broadcast, NBMA and Point-to-MultiPoint segments (§A, §3.1) | MUST | A | **positive:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L18). **negative:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L22) |
| `RFC3623-A-5` | DoNotAge is never set in a grace-LSA, even over a demand circuit (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62). **negative:** no negative test. **{single-polarity}:** Grace-LSAs originate through OriginateOpaque, whose input struct has no DoNotAge field and starts LS age at 0 with normal aging, so the bit is never set and no negative behavior exists to test (internal/plugins/ospf/lsdb/opaque_as.go:73, gr_restarter.go:314, lsdb/entry.go:87) |
| `RFC3623-2.1-1` | Before reload, ensure forwarding table(s) are up-to-date and remain in place across the restart (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91). **negative:** no negative test. **{single-polarity}:** prepareRestart raises gracefulStop so suppressInstall makes the ensuing engine stop skip RemoveAll and retain the pre-restart FIB; the meaningful assertion is retention, with no negative behavior the requirement forbids (internal/plugins/ospf/gr_restarter.go:49, gr.go:235) |
| `RFC3623-5-1` | An implementation providing recovery from unplanned outages must allow the operator to turn the option off (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L40). **negative:** `unit/verify` [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L16) |
| `RFC3623-5-2` | On unplanned restart, grace-LSAs must be originated and sent before any OSPF Hello packets; on broadcast networks flooded to AllSPFRouters 224.0.0.5 (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151). **negative:** no negative test. **{single-polarity}:** maybeUnplannedRestart enters in-restart then originates one Grace-LSA per interface before interface Hellos begin, and link-local LSAs flood to AllSPFRouters via the standard link-scope path (internal/plugins/ospf/gr_restarter.go:105, lsdb_flooding_test.go:47) |
| `RFC3623-5-3` | On unplanned restart, grace-LSAs are encapsulated in Link State Update packets and sent out all interfaces (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193). **negative:** no negative test. **{single-polarity}:** grOriginateGraceLSAs walks every active (non-passive) interface and floods each Grace-LSA via OriginateOpaque's standard LSU flooding (internal/plugins/ospf/gr_restarter.go:296, :315) |
| `RFC3623-5-4` | On unplanned restart, the restart reason in grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor) (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L58). **negative:** no negative test. **{single-polarity}:** grUnplannedReason returns the constant 3 (redundant control processor), an in-range unplanned reason, and there is no receive-side reason rejection to test negatively (internal/plugins/ospf/gr_restarter.go:88, gr.go:36) |
| `RFC3623-3-1` | When helping over a virtual link, the helper must continue to set bit V in its router-LSA for the transit area (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| `RFC3623-3.1-1` | Helper must refuse to enter helper mode if LSAs on X's retransmission list have changed content (not periodic refreshes) (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| `RFC3623-3.2-1` | If Y aggregated adjacencies on entering helper mode, it must exit helper mode for all adjacencies with X when any one exit event occurs (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze tracks one helper session per (interface, router) and never aggregates adjacencies across segments, so the aggregated exit-all obligation binds a mode ze does not play (internal/plugins/ospf/gr.go:71) |
| `RFC3623-2.1-2` | The grace period should not exceed LSRefreshTime (1800 seconds) (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-3` | Retransmit grace-LSAs until acknowledged (standard OSPF reliable flooding) (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-4` | After sending grace-LSAs, store the restart fact and grace-period length in non-volatile storage (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-1` | On exit, reoriginate router-LSAs for all attached areas (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-2` | On exit, reoriginate network-LSAs on all segments where it is Designated Router (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-3` | On exit, remove remnant pre-restart forwarding entries that are no longer valid (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-4` | On exit, flush received self-originated LSAs that are no longer valid (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-5` | On exit, flush any grace-LSAs the router originated (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-5` | Preserve cryptographic sequence numbers (or the clock generating them) in non-volatile storage across restarts (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-5-5` | Send grace-LSAs multiple times on unplanned restart to improve delivery (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-2` | Helper may disallow graceful restart with X on other network segments (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-3` | Helper may aggregate adjacencies, entering helper mode only when checks pass for all adjacencies with X (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.2-2` | Provide a configuration option to disable LSDB changes from terminating graceful restart (increases loop/black-hole risk) (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3623-A-1`](#rfc3623-a-1) Additional Grace-LSA TLVs must be described in an Internet Draft and subject to OSPF WG expert review (§A) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates only the three RFC-defined TLVs (types 1/2/3) and adds none, and its decoder ignores unrecognized types, so the IETF-process obligation for additional TLVs binds no ze behavior (internal/plugins/ospf/packet/grace_lsa.go:44, :64) |
| [`RFC3623-3-1`](#rfc3623-3-1) When helping over a virtual link, the helper must continue to set bit V in its router-LSA for the transit area (§3) | {gap}, no test | the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| [`RFC3623-3.1-1`](#rfc3623-3.1-1) Helper must refuse to enter helper mode if LSAs on X's retransmission list have changed content (not periodic refreshes) (§3.1) | {gap}, no test | the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| [`RFC3623-3.2-1`](#rfc3623-3.2-1) If Y aggregated adjacencies on entering helper mode, it must exit helper mode for all adjacencies with X when any one exit event occurs (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze tracks one helper session per (interface, router) and never aggregates adjacencies across segments, so the aggregated exit-all obligation binds a mode ze does not play (internal/plugins/ospf/gr.go:71) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3623-A-1`](#rfc3623-a-1)

Additional Grace-LSA TLVs must be described in an Internet Draft and subject to OSPF WG expert review (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-A-1, so no unit is bound to it.

### [`RFC3623-A-2`](#rfc3623-a-2)

Grace Period TLV (type 1) must always appear in a grace-LSA (§A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L75) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L20) | unit/verify | unproven |

### [`RFC3623-A-3`](#rfc3623-a-3)

Graceful restart reason TLV (type 2) must always appear in a grace-LSA (§A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L72) | unit/verify | unproven |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/grace_lsa_test.go#L24) | unit/verify | unproven |

### [`RFC3623-A-4`](#rfc3623-a-4)

IP interface address TLV (type 3) required on broadcast, NBMA and Point-to-MultiPoint segments (§A, §3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L22) | unit/verify | unproven |
| positive | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_lsa_test.go#L18) | unit/verify | unproven |

### [`RFC3623-A-5`](#rfc3623-a-5)

DoNotAge is never set in a grace-LSA, even over a demand circuit (§A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62) | unit/verify | unproven |

### [`RFC3623-2.1-1`](#rfc3623-2.1-1)

Before reload, ensure forwarding table(s) are up-to-date and remain in place across the restart (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91) | unit/verify | unproven |

### [`RFC3623-5-1`](#rfc3623-5-1)

An implementation providing recovery from unplanned outages must allow the operator to turn the option off (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L16) | unit/verify | unproven |
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L40) | unit/verify | unproven |

### [`RFC3623-5-2`](#rfc3623-5-2)

On unplanned restart, grace-LSAs must be originated and sent before any OSPF Hello packets; on broadcast networks flooded to AllSPFRouters 224.0.0.5 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L151) | unit/verify | unproven |

### [`RFC3623-5-3`](#rfc3623-5-3)

On unplanned restart, grace-LSAs are encapsulated in Link State Update packets and sent out all interfaces (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L193) | unit/verify | unproven |

### [`RFC3623-5-4`](#rfc3623-5-4)

On unplanned restart, the restart reason in grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor) (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/gr_unplanned_test.go#L58) | unit/verify | unproven |

### [`RFC3623-3-1`](#rfc3623-3-1)

When helping over a virtual link, the helper must continue to set bit V in its router-LSA for the transit area (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3-1, so no unit is bound to it.

### [`RFC3623-3.1-1`](#rfc3623-3.1-1)

Helper must refuse to enter helper mode if LSAs on X's retransmission list have changed content (not periodic refreshes) (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3.1-1, so no unit is bound to it.

### [`RFC3623-3.2-1`](#rfc3623-3.2-1)

If Y aggregated adjacencies on entering helper mode, it must exit helper mode for all adjacencies with X when any one exit event occurs (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3623-3.2-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 3623, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3623, so its obligations are stated where they were written.
