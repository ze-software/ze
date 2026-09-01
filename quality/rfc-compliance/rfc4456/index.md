# RFC 4456 - BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP)

Supported. Every requirement this repository extracted from RFC 4456, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 83.3% | 5 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 1 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4456.md` |
| Requirement shard | `rfc/requirements/rfc4456.md` |
| RFC text | `rfc/full/rfc4456.txt` |

## Enrolment

Enrolled: BGP Route Reflection: six MUST-level requirements over the reactor RS-fast-path route-reflection code (internal/component/bgp/reactor/forward_rs.go, filter_delta_handlers.go). Five have both polarities: RFC4456-8-1 (set ORIGINATOR_ID if absent) and RFC4456-8-4 (preserve if present) via originatorIDHandler; RFC4456-8-2 (prepend CLUSTER_ID) via clusterListHandler AttrModPrepend; RFC4456-8-3 (ORIGINATOR_ID not fabricated -- it is the originator's id, not the RR's, and a present one is not replaced); RFC4456-x-2 (a non-client route is not reflected to a non-client but is to a client). RFC4456-x-1 (must not modify NEXT_HOP/AS_PATH/LOCAL_PREF/MED) is {single-polarity: positive}: on reflection only ORIGINATOR_ID/CLUSTER_LIST ops are emitted, so those four ride through unchanged. Tests: forward_rr_test.go (TestReactorForwardRRInjects/PreservesOriginator/NonClientRule). The 8-5/8-6 SHOULDs are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

ORIGINATOR_ID, CLUSTER_LIST, route-reflector plugin behavior, cluster checks.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC4456-8-1`](#rfc4456-8-1), [`RFC4456-8-2`](#rfc4456-8-2), [`RFC4456-8-3`](#rfc4456-8-3), [`RFC4456-8-4`](#rfc4456-8-4), [`RFC4456-x-2`](#rfc4456-x-2)

**Annotated instead of tested (1):** [`RFC4456-x-1`](#rfc4456-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4456-x-1` | An RR MUST NOT modify the NEXT_HOP, AS_PATH, LOCAL_PREF, or MED attributes of a reflected route (Route Reflection Rules) | MUST | x | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L157). **negative:** no negative test. **{single-polarity}:** on reflection the RR forwarding path emits only ORIGINATOR_ID and CLUSTER_LIST modifications (internal/component/bgp/reactor/forward_rs.go:337-339) and never a NEXT_HOP/AS_PATH/LOCAL_PREF/MED op, so those four are always carried through in the verbatim wire; there is no RR scenario that modifies them to assert as a negative. The positive is proven byte-identical in TestReactorForwardRRInjects |
| `RFC4456-8-1` | When an RR reflects a route from a client to a non-client or to another client, it MUST set the ORIGINATOR_ID to the BGP Identifier of the originator if not already present (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L149). **negative:** `unit/verify` [`TestForwardReflectionLeavesAWithdrawalUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_withdraw_shape_test.go#L436). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L180). **positive:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L131). **negative:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L132) |
| `RFC4456-8-2` | When an RR reflects a route, it MUST prepend its local CLUSTER_ID to the CLUSTER_LIST (creating one if absent) (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L151). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L184). **positive:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L133). **negative:** `interop/nightly` [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L134) |
| `RFC4456-8-3` | ORIGINATOR_ID MUST NOT be created by a speaker that did not originate the route within the local AS (§8) | MUST NOT | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L153). **negative:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L182) |
| `RFC4456-8-4` | ORIGINATOR_ID value MUST be preserved unchanged through the reflection chain (§8) | MUST | 8 - Avoiding Routing Information Loops | **positive:** `unit/verify` [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L178). **negative:** `unit/verify` [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L155) |
| `RFC4456-x-2` | A non-client peer route MUST NOT be reflected to other non-client peers (Route Reflection Rules) | MUST | x | **positive:** `unit/verify` [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L211). **negative:** `unit/verify` [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L213) |
| `RFC4456-8-5` | A router that recognizes ORIGINATOR_ID SHOULD ignore a route received with its own BGP Identifier as the ORIGINATOR_ID (§8) | SHOULD | 8 - Avoiding Routing Information Loops | **positive:** no positive test. **negative:** no negative test |
| `RFC4456-8-6` | If the local CLUSTER_ID is found in the CLUSTER_LIST, the advertisement SHOULD be ignored (§8) | SHOULD | 8 - Avoiding Routing Information Loops | **positive:** no positive test. **negative:** no negative test |
| `RFC4456-9-1` | A BGP Speaker SHOULD prefer a route with the shorter CLUSTER_LIST length; the length is zero when the route carries no CLUSTER_LIST attribute, and the rule is inserted between RFC 4271 Section 9.1.2.2 Steps f) and g) (§9) | SHOULD | 9 - Impact on Route Selection | **positive:** `unit/verify` [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1207). **positive:** `unit/verify` [`TestClusterListEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1302). **negative:** `unit/verify` [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1213). **positive:** `interop/nightly` [`checkClusterListLengthTieBreak`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc4456.go#L47) |

## Gaps and untested MUSTs

RFC 4456 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4456-x-1`](#rfc4456-x-1)

An RR MUST NOT modify the NEXT_HOP, AS_PATH, LOCAL_PREF, or MED attributes of a reflected route (Route Reflection Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L157) | unit/verify | unproven |

### [`RFC4456-8-1`](#rfc4456-8-1)

When an RR reflects a route from a client to a non-client or to another client, it MUST set the ORIGINATOR_ID to the BGP Identifier of the originator if not already present (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestForwardReflectionLeavesAWithdrawalUntouched`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_withdraw_shape_test.go#L436) | unit/verify | unproven |
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L180) | unit/verify | unproven |
| negative | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L132) | interop/nightly | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L149) | unit/verify | unproven |
| positive | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L131) | interop/nightly | unproven |

### [`RFC4456-8-2`](#rfc4456-8-2)

When an RR reflects a route, it MUST prepend its local CLUSTER_ID to the CLUSTER_LIST (creating one if absent) (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L184) | unit/verify | unproven |
| negative | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L134) | interop/nightly | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L151) | unit/verify | unproven |
| positive | [`checkReflectorWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_special.go#L133) | interop/nightly | unproven |

### [`RFC4456-8-3`](#rfc4456-8-3)

ORIGINATOR_ID MUST NOT be created by a speaker that did not originate the route within the local AS (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L182) | unit/verify | unproven |
| positive | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L153) | unit/verify | unproven |

### [`RFC4456-8-4`](#rfc4456-8-4)

ORIGINATOR_ID value MUST be preserved unchanged through the reflection chain (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRInjects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L155) | unit/verify | unproven |
| positive | [`TestReactorForwardRRPreservesOriginator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L178) | unit/verify | unproven |

### [`RFC4456-x-2`](#rfc4456-x-2)

A non-client peer route MUST NOT be reflected to other non-client peers (Route Reflection Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L213) | unit/verify | unproven |
| positive | [`TestReactorForwardRRNonClientRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L211) | unit/verify | unproven |

### [`RFC4456-9-1`](#rfc4456-9-1)

A BGP Speaker SHOULD prefer a route with the shorter CLUSTER_LIST length; the length is zero when the route carries no CLUSTER_LIST attribute, and the rule is inserted between RFC 4271 Section 9.1.2.2 Steps f) and g) (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1213) | unit/verify | unproven |
| positive | [`TestBestPath_ClusterListLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1207) | unit/verify | unproven |
| positive | [`TestClusterListEntries`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/bestpath_test.go#L1302) | unit/verify | unproven |
| positive | [`checkClusterListLengthTieBreak`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc4456.go#L47) | interop/nightly | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc4456 |
| Signed off | 2026-08-31 |
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
| `14.2` | not stated | 1 | walked | Informative References (RFC 4223, RFC 3065, RFC 1966, RFC 2385, RFC 2796 and RFC 2119), and, because no numbered heading follows it, the whole tail of the document: Appendix A, Appendix B, Authors' Addresses, the Full Copyright Statement, the Intellectual Property notice and the RFC Editor funding acknowledgement. The section is recorded as walked rather than skipped because that span holds prose beyond a reference list. Appendix A and Appendix B are non-normative comparisons with RFC 2796 and RFC 1966 that record what changed, including that the CLUSTER_ID addition moved from 'append' to 'prepend' to match deployed code, which is stated as an obligation by section 8 and captured there. The one site is the Intellectual Property notice, excluded below. |

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
| `14.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Intellectual Property notice from the RFC's closing boilerplate, which the section splitter attributes to 14.2 because no numbered heading follows the informative reference list. It invites interested parties to tell the IETF about patents and directs no protocol behaviour. It is boilerplate the extractor did not strip. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4456, so its obligations are stated where they were written.
