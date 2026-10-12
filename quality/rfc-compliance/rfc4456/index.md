# RFC 4456 - BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP)

Supported. Every requirement this repository extracted from RFC 4456, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 9.1% | 2 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 4 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 7 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc4456.md` |
| Requirement shard | `rfc/requirements/rfc4456.md` |
| RFC text | `rfc/full/rfc4456.txt` |

## Enrolment

Enrolled: BGP Route Reflection: four MUST-level requirements over the reactor RS-fast-path route-reflection code (internal/component/bgp/reactor/forward_rs.go, filter_delta_handlers.go), each with both polarities: RFC4456-8-1 (ORIGINATOR_ID created on reflection) and RFC4456-8-4 (it carries the originator's identifier, kept when present) via originatorIDHandler; RFC4456-8-2 (prepend CLUSTER_ID, create the list if empty) via clusterListHandler AttrModPrepend; RFC4456-x-2 (a non-client route is reflected to clients, not to a non-client). RFC4456-8-3 (do not create an ORIGINATOR_ID when one exists) and RFC4456-x-1 (do not modify NEXT_HOP/AS_PATH/LOCAL_PREF/MED) are SHOULD NOT in the RFC (rfc/corrections/rfc4456.md) and still carry their tests. Tests: forward_rr_test.go (TestReactorForwardRRInjects/PreservesOriginator/NonClientRule). The 8-5/8-6/9-1 SHOULDs are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

ORIGINATOR_ID, CLUSTER_LIST, route-reflector plugin behavior, cluster checks.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC4456-8-1`](#rfc4456-8-1), [`RFC4456-8-2`](#rfc4456-8-2), [`RFC4456-8-4`](#rfc4456-8-4), [`RFC4456-x-2`](#rfc4456-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4456-x-1` | In addition, when a RR reflects a route, it SHOULD NOT modify the following path attributes: NEXT_HOP, AS_PATH, LOCAL_PREF, and MED. (§10) | SHOULD NOT | 10 - Implementation Considerations | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L178). **negative:** no negative test. **{single-polarity}:** on reflection the RR forwarding path emits only ORIGINATOR_ID and CLUSTER_LIST modifications (internal/component/bgp/reactor/forward_rs.go:337-339) and never a NEXT_HOP/AS_PATH/LOCAL_PREF/MED op, so those four are always carried through in the verbatim wire; there is no RR scenario that modifies them to assert as a negative. The positive is proven byte-identical in TestReactorForwardRRInjects |
| `RFC4456-8-1` | This attribute is 4 bytes long and it will be created by an RR in reflecting a route. (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L170). **negative:** `unit/verify` [`TestForwardReflectionLeavesAWithdrawalUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_forward_build_withdraw_shape_test.go#L436). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L201). **positive:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L272). **negative:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L273) |
| `RFC4456-8-2` | When an RR reflects a route, it MUST prepend the local CLUSTER_ID to the CLUSTER_LIST.  If the CLUSTER_LIST is empty, it MUST create a new one. (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L172). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L205). **positive:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L274). **negative:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L275) |
| `RFC4456-8-3` | A BGP speaker SHOULD NOT create an ORIGINATOR_ID attribute if one already exists. (§8) | SHOULD NOT | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L174). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L203) |
| `RFC4456-8-4` | This attribute will carry the BGP Identifier of the originator of the route in the local AS. (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_originator_id_test.go#L30). **positive:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L199). **negative:** `unit/verify` [`TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_originator_id_test.go#L31). **negative:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L176) |
| `RFC4456-x-2` | After the best path is selected, it must do the following depending on the type of peer it is receiving the best path from 1) A route from a Non-Client IBGP peer: Reflect to all the Clients. (§6) | MUST | 6 - Operation | **positive:** `unit/verify` [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L232). **negative:** `unit/verify` [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L234) |
| `RFC4456-8-5` | A router that recognizes the ORIGINATOR_ID attribute SHOULD ignore a route received with its BGP Identifier as the ORIGINATOR_ID. (§8) | SHOULD | 8 - Avoiding Routing Information Loops | **positive:** no positive test. **negative:** no negative test |
| `RFC4456-8-6` | If the local CLUSTER_ID is found in the CLUSTER_LIST, the advertisement received SHOULD be ignored. (§8) | SHOULD | 8 - Avoiding Routing Information Loops | **positive:** no positive test. **negative:** no negative test |
| `RFC4456-9-1` | In addition, the following rule SHOULD be inserted between Steps f) and g): a BGP Speaker SHOULD prefer a route with the shorter CLUSTER_LIST length.  The CLUSTER_LIST length is zero if a route does not carry the CLUSTER_LIST attribute. (§9) | SHOULD | 9 - Impact on Route Selection | **positive:** `unit/verify` [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1215). **positive:** `unit/verify` [`TestClusterListEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1310). **negative:** `unit/verify` [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1221). **positive:** `interop/nightly` [`checkClusterListLengthTieBreak`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc4456.go#L47) |

## Gaps and untested MUSTs

RFC 4456 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4456-x-1`](#rfc4456-x-1)

In addition, when a RR reflects a route, it SHOULD NOT modify the following path attributes: NEXT_HOP, AS_PATH, LOCAL_PREF, and MED. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestReactorForwardRRInjects pins AS_PATH, NEXT_HOP, MED and LOCAL_PREF byte-identical after reflection, so any modification fails it; single-polarity annotated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L178) | unit/verify | unproven |

### [`RFC4456-8-1`](#rfc4456-8-1)

This attribute is 4 bytes long and it will be created by an RR in reflecting a route. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent supplemental source rejudgment, 2026-10-06. RFC4456 Section 8: "ORIGINATOR_ID is a new optional, non-transitive BGP attribute of Type code 9. This attribute is 4 bytes long and it will be created by an RR in reflecting a route. This attribute will carry the BGP Identifier of the originator of the route in the local AS. A BGP speaker SHOULD NOT create an ORIGINATOR_ID attribute if one already exists." All five covers read: forward_rr_test.go TestReactorForwardRRInjects and TestReactorForwardRRPreservesOriginator, rfc4456_forward_build_withdraw_shape_test.go TestForwardReflectionLeavesAWithdrawalUntouched, and both checkReflectorWithdrawal tags. The real RS forward helper captures dispatched bodies and decodes bounded attribute fields: absent originator becomes exactly four bytes 10.0.0.1; present 9.9.9.9 remains byte-exact. General-forward withdrawal and EOR cases remain byte-exact, with an advertisement control requiring both RR attributes. Producers forwardUpdateSection/reactorForwardRSSection record Set(9); attrModHandlersWithDefaults explicitly registers originatorIDHandler, whose absent-source branch emits only a four-byte Set and existing-source branch keeps it; planAttr prevents creation without reachable NLRI. Interop carrier requires exact attribute tokens, refuses an early withdrawal, requests FRR withdrawal, then requires its exact wire shape and established session. It is registered in specialCheckers. No current diff changes checkReflectorWithdrawal itself (only unrelated waitZePeerState in its file); therefore this stale-unit is preexisting judgment debt relative to HEAD, not this mechanical edit. Supplemental interop log matching is not promoted to independent executed proof: live named-peer run remains pending. Existing enforced verdict retained on complementary precise unit assertions; no new defect found.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L201) | unit/verify | unproven |
| negative | [`TestForwardReflectionLeavesAWithdrawalUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_forward_build_withdraw_shape_test.go#L436) | unit/verify | unproven |
| negative | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L273) | interop/nightly | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L170) | unit/verify | unproven |
| positive | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L272) | interop/nightly | unproven |

### [`RFC4456-8-2`](#rfc4456-8-2)

When an RR reflects a route, it MUST prepend the local CLUSTER_ID to the CLUSTER_LIST.  If the CLUSTER_LIST is empty, it MUST create a new one. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent supplemental source rejudgment, 2026-10-06. RFC4456 Section 8: "When an RR reflects a route, it MUST prepend the local CLUSTER_ID to the CLUSTER_LIST. If the CLUSTER_LIST is empty, it MUST create a new one." Read all four covers: forward_rr_test.go TestReactorForwardRRInjects, TestReactorForwardRRPreservesOriginator, and both checkReflectorWithdrawal tags. With an otherwise identical valid iBGP route, the first requires exact CLUSTER_LIST [1.2.3.2] and the second exact [1.2.3.2,5.5.5.5]; replacement, append, omission and wrong local ID all fail. Existing-list input isolates the prepend obligation, while absent-list input proves creation. Traced both forward producers recording Prepend(10) through attrModHandlersWithDefaults to clusterListHandler, which emits each four-byte prepend before the retained source value. planAttr blocks creation on withdrawals/EOR, and interop requires the reflected attribute token followed by an explicitly requested attribute-free withdrawal. The interop unit has no current diff against HEAD, so its stale-unit is preexisting judgment debt rather than current mechanical change; sibling waitZePeerState edits do not alter it. Source verdict retained; named-peer execution remains pending, and no canonical producer-halt record is taken as semantic correctness. No new defect found.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L205) | unit/verify | unproven |
| negative | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L275) | interop/nightly | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L172) | unit/verify | unproven |
| positive | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L274) | interop/nightly | unproven |

### [`RFC4456-8-3`](#rfc4456-8-3)

A BGP speaker SHOULD NOT create an ORIGINATOR_ID attribute if one already exists. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. negative asserts an existing ORIGINATOR_ID 9.9.9.9 survives reflection unchanged, which is the SHOULD NOT itself; positive shows creation when none exists

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L203) | unit/verify | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L174) | unit/verify | unproven |

### [`RFC4456-8-4`](#rfc4456-8-4)

This attribute will carry the BGP Identifier of the originator of the route in the local AS. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sentence: ORIGINATOR_ID carries the BGP Identifier of the originator. TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier separates the source address (10.0.0.1) from its BGP Identifier (10.11.12.13): the created ORIGINATOR_ID equals 10.11.12.13 exactly and is neither the address nor the reflector's id 1.2.3.2; an existing 9.9.9.9 is kept and not replaced by the neighbor's Identifier. The address/Identifier confusion the earlier verdict named would now go red. Observed-red records on reactorForwardRS both polarities. Old negative tag prose on TestReactorForwardRRInjects still describes 8-1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L176) | unit/verify | unproven |
| negative | [`TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_originator_id_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L199) | unit/verify | unproven |
| positive | [`TestRFC4456OriginatorIDIsTheOriginatorsBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4456_originator_id_test.go#L30) | unit/verify | revert, verified |

### [`RFC4456-x-2`](#rfc4456-x-2)

After the best path is selected, it must do the following depending on the type of peer it is receiving the best path from 1) A route from a Non-Client IBGP peer: Reflect to all the Clients. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. asserts nil output for non-client to non-client and non-nil for non-client to client; tag polarity labels are inverted against the new quote, whose conforming case is reflection to a client

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L234) | unit/verify | unproven |
| positive | [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L232) | unit/verify | unproven |

### [`RFC4456-9-1`](#rfc4456-9-1)

In addition, the following rule SHOULD be inserted between Steps f) and g): a BGP Speaker SHOULD prefer a route with the shorter CLUSTER_LIST length.  The CLUSTER_LIST length is zero if a route does not carry the CLUSTER_LIST attribute. (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. SelectBest and SelectBestExplain asserted on shorter-list win, absent-as-zero win, step f precedence and fall-through to step g; clusterListEntries pins absent and empty as zero

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1221) | unit/verify | unproven |
| positive | [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1215) | unit/verify | unproven |
| positive | [`TestClusterListEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1310) | unit/verify | unproven |
| positive | [`checkClusterListLengthTieBreak`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc4456.go#L47) | interop/nightly | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc4456.txt |
| Source fingerprint | efe974c45b13bec4 |
| Record | rfc/extraction/rfc4456.json |
| Mapped sentences | 2 |
| Declined as scope | 10 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract states the scaling problem this document alleviates and announces that it obsoletes RFC 2796 and RFC 1966. Its one site is the lowercase restatement of the existing full-mesh model, excluded below. |
| `1` | Introduction | 1 | walked | Introduction. Indicative prose: n*(n-1)/2 iBGP sessions do not scale, other proposals exist, this document proposes route reflection, and it adds two new optional non-transitive attributes to prevent loops. The one site repeats the Abstract's description of the existing model and directs no speaker. |
| `2` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph, which lists the key words in upper case. It binds no speaker, and it is what makes every lowercase 'must' in sections 1, 3, 4, 5 and 6 a description of the scheme rather than a directive. |
| `3` | Design Criteria | 3 | walked | Design Criteria. Three criteria route reflection 'was designed to satisfy': simplicity, easy transition and compatibility with noncompliant iBGP peers. Three sites, all excluded below: they measure a design against a goal and name no act a speaker performs on the wire. |
| `4` | Route Reflection | 1 | walked | Route Reflection. The Figure 1 and Figure 2 worked example that motivates the scheme, ending in 'The route reflection scheme is based upon this basic principle'. Its one site describes what RTR-A does under the EXISTING BGP model, so it is excluded below. |
| `5` | Terminology and Concepts | 1 | walked | Terminology and Concepts. Defines route reflection, route reflector, reflected route, client and non-client peers, and cluster, with Figure 3. Its one site is the topology constraint that non-clients stay fully meshed, which binds the operator and is excluded below. |
| `6` | Operation | 2 | walked | Operation. The distribution rule an RR applies after it selects the best path, in two cases: a route from a non-client is reflected to all clients, and a route from a client is reflected to all non-clients and to the other clients. Site 6:1 is the sentence that introduces both cases and is mapped below to RFC4456-x-2, the gated row rfc/short/rfc4456.md derives from its Route Reflection Rules table. The remainder of the section is indicative: an AS can have many RRs, an RR treats other RRs as ordinary internal speakers, and conventional speakers that do not understand route reflection can coexist in either group, which is a migration statement and not a directive. |
| `7` | Redundant RRs | 0 | walked | Redundant RRs. Value and configuration statement: a single-RR cluster is identified by the BGP Identifier of the RR, and to remove that single point of failure all RRs in one cluster can be configured with a 4-byte CLUSTER_ID so that an RR can discard routes from other RRs in the same cluster. It is permissive ('can be configured') and states no obligation; the discard itself is section 8's SHOULD, declared as RFC4456-8-6. Ze reads the configured cluster-id and falls back to the Router ID for exactly this default (internal/component/bgp/reactor/filter.LoopIngress). |
| `8` | Avoiding Routing Information Loops | 2 | walked | Avoiding Routing Information Loops. Defines ORIGINATOR_ID (optional non-transitive, type code 9, 4 bytes) and CLUSTER_LIST (optional non-transitive, type code 10, a sequence of CLUSTER_ID values). Its two capitalised MUSTs are the CLUSTER_LIST prepend and the create-if-empty clause, mapped below. Five further obligations are stated in shapes a MUST-level site scan cannot see, so they are the unsourced ids here: the attribute 'will be created by an RR in reflecting a route' (RFC4456-8-1) and 'will carry the BGP Identifier of the originator of the route in the local AS' (RFC4456-8-4, the value preserved through the chain), 'A BGP speaker SHOULD NOT create an ORIGINATOR_ID attribute if one already exists' (RFC4456-8-3), 'A router that recognizes the ORIGINATOR_ID attribute SHOULD ignore a route received with its BGP Identifier as the ORIGINATOR_ID' (RFC4456-8-5), and 'If the local CLUSTER_ID is found in the CLUSTER_LIST, the advertisement received SHOULD be ignored' (RFC4456-8-6). rfc/short/rfc4456.md declares RFC4456-8-3 at MUST NOT where the source says SHOULD NOT: the summary is STRICTER than the source, which conformance permits, and Ze meets the stricter reading. |
| `9` | Impact on Route Selection | 0 | walked | Impact on Route Selection. Two modifications to the RFC 4271 tie-breaking rules, both SHOULD, neither visible to a MUST-level site scan. Ze implements both. The first, 'If a route carries the ORIGINATOR_ID attribute, then in Step f) the ORIGINATOR_ID SHOULD be treated as the BGP Identifier of the BGP speaker that has advertised the route', is the OriginatorIP field of rib.Candidate, filled from the ORIGINATOR_ID attribute and falling back to the peer's Router ID (the candidate build in rib_commands.go), compared at the Router ID step of rib.comparePair. It has no declared id. The second, 'a BGP Speaker SHOULD prefer a route with the shorter CLUSTER_LIST length', was NOT implemented when this walk was performed and was reported to the owner rather than recorded as satisfied. He ruled it be implemented unconditionally, without a configuration option, and it landed the same day: Candidate.ClusterListEntries, filled beside OriginatorIP, compared between the Router ID and peer address steps of both rib.comparePair and rib.comparePairWithReason, and declared as RFC4456-9-1 [SHOULD] in rfc/short/rfc4456.md with tests in both polarities. Corrected 2026-08-31: this reason previously said Ze did not implement it, which was true at sign-off and false within hours. The exclusion count is unchanged by the correction, so no resign-reason is owed. |
| `10` | Implementation Considerations | 0 | walked | Implementation Considerations. Two directives, both invisible to a MUST-level site scan. 'Care should be taken to make sure that none of the BGP path attributes defined above can be modified through configuration when exchanging internal routing information between RRs and Clients and Non-Clients' constrains what configuration may offer, and 'when a RR reflects a route, it SHOULD NOT modify the following path attributes: NEXT_HOP, AS_PATH, LOCAL_PREF, and MED' is the source of RFC4456-x-1, the unsourced id recorded here. rfc/short/rfc4456.md declares that row at MUST where the source says SHOULD NOT, so the summary is stricter than the source and Ze meets the stricter reading. |
| `11` | Configuration and Deployment Considerations | 0 | walked | Configuration and Deployment Considerations. Operator guidance throughout: a client cannot identify itself dynamically so it is configured by hand, incomparable MEDs and differing IGP metrics can make reflection select a different route than a full mesh would, and three ways to avoid that (local preference set at the border router, distinct AS-path lengths, community-based policy), then POP-based topology advice. Every sentence directs the person designing the reflection topology, in lowercase 'may' and 'should', and none directs a speaker. |
| `12` | Security Considerations | 0 | walked | Security Considerations. One sentence: this extension to BGP does not change the underlying security issues inherent in the existing iBGP. No countermeasure is directed at a speaker. |
| `13` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `14` | References | 0 | skipped (references) | References. The heading over the two reference lists below it. |
| `14.1` | Normative References: RFC 4271 | 0 | skipped (references) | Normative References: RFC 4271. |
| `14.2` | not stated | 0 | walked | Informative References (RFC 4223, RFC 3065, RFC 1966, RFC 2385, RFC 2796 and RFC 2119). A reference list with no site. Appendix A and Appendix B follow as their own sections: non-normative comparisons with RFC 2796 and RFC 1966 that record what changed, including that the CLUSTER_ID addition moved from 'append' to 'prepend' to match deployed code, which is stated as an obligation by section 8 and captured there. |
| `A` | Appendix A | 0 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `B` | not stated | 1 | walked | Appendix B, the comparison with RFC 1966, and, because no heading follows it, the tail of the document: Authors' Addresses, the Full Copyright Statement, the Intellectual Property notice and the RFC Editor funding acknowledgement. The section is recorded as walked because that span holds prose beyond a reference list. The one site is the Intellectual Property notice, excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Abstract. Lowercase 'must', twice, describing the property of the EXISTING BGP model that creates the scaling problem: speakers are typically fully meshed, so external routing information is re-distributed to every other router in the AS. Section 2 lists the key words in upper case, so this is not one of them, and the sentence states no obligation this document creates. | Typically, all BGP speakers within a single AS must be fully meshed so that any external routing information must be re-distributed to all other routers within that Autonomous System (AS). |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction. The same description of the existing full-mesh model as the Abstract, in the body text. The paragraph's next sentence draws the consequence ('This "full mesh" requirement clearly does not scale'), which is the problem statement this document answers rather than a directive it issues. | Typically, all BGP speakers within a single AS must be fully meshed and any external routing information must be re-distributed to all other routers within that AS. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Design Criteria, bullet 'Simplicity'. The section opens 'Route reflection was designed to satisfy the following criteria', so the sentence measures a design against a goal in the past tense. Being simple to configure and easy to understand is not an act a speaker performs, and no code path can carry it. | Any alternative must be simple to configure and easy to understand. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Design Criteria, bullet 'Easy Transition'. Same frame as 3:1: a criterion the design had to meet, namely that an operator can transition from a full mesh without changing topology or AS. The sentence then contrasts the technique of RFC 3065 as unfortunate management overhead, which is commentary, not a requirement. | It must be possible to transition from a full-mesh configuration without the need to change either topology or AS. |
| `3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Design Criteria, bullet 'Compatibility'. Same frame as 3:1: a criterion the design had to meet, that noncompliant iBGP peers can stay in the AS without losing routing information. The mechanism that delivers it is section 6's statement that conventional speakers can be members of either group, which is itself indicative. | It must be possible for noncompliant IBGP peers to continue to be part of the original AS or domain without any loss of BGP routing information. |
| `4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Route Reflection. A worked example of the Figure 1 full mesh, and it says so in its own opening clause: 'With the existing BGP model'. It describes what RTR-A does BEFORE the rule this document relaxes, and the next sentence then relaxes it. Nothing here binds an RR. | With the existing BGP model, if RTR-A receives an external route and it is selected as the best path it must advertise the external route to both RTR-B and RTR-C. |
| `5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Terminology and Concepts. The role is the operator who designs the AS's iBGP topology. The sentence states which sessions have to exist, not what a speaker does with a message: non-clients stay fully meshed among themselves while clients need not be. A route reflector cannot create or police its non-clients' sessions with each other, and Ze reads client versus non-client from configuration. The corresponding wire behaviour, that a non-client's route is not reflected to another non-client, is section 6's rule and is mapped at site 6:1. The role is a person designing a topology, so no producer could act as it. Ze CONSUMES the topology the operator built: the route reflector plugin (`internal/component/bgp/plugins/rr/register.go`) reflects over the sessions it is configured with and designs none. | The Non-Client peer must be fully meshed but the Client peers need not be fully meshed. |
| `6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Operation, case 2. A parenthetical drawing the consequence of the rule above it: because a client's route is reflected to the other clients, the clients are not required to be fully meshed. The site scan sees it for the word 'required', and it states that a requirement does NOT apply rather than imposing one. | (Hence the Client peers are not required to be fully meshed.) |
| `8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Avoiding Routing Information Loops, CLUSTER_LIST. The second half of the same obligation: the sentence before it requires the prepend, and this one says what to do when there is nothing to prepend to. rfc/short/rfc4456.md carries both in one row, 'MUST prepend its local CLUSTER_ID to the CLUSTER_LIST (creating one if absent)', which site 8:1 maps. | If the CLUSTER_LIST is empty, it MUST create a new one. |
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Intellectual Property notice from the RFC's closing boilerplate, which falls in Appendix B because no heading follows it. It invites interested parties to tell the IETF about patents and directs no protocol behaviour. It is boilerplate the extractor did not strip. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4456, so its obligations are stated where they were written.
