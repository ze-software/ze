# RFC 5286 - Basic Specification for IP Fast Reroute: Loop-Free Alternates

Experimental. Every requirement this repository extracted from RFC 5286, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 3 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 40.0% | 4 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 16.7% | 1 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 2 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 3 | of 6 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 30 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 10 |
| Recorded audit verdicts | 3 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc5286.md` |
| Requirement shard | `rfc/requirements/rfc5286.md` |
| RFC text | `rfc/full/rfc5286.txt` |

## Enrolment

Enrolled: IP Fast Reroute: Loop-Free Alternates (LFA): six MUST-level requirements, computed in OSPF only (internal/plugins/ospf/spf/lfa.go). x-1 (Inequality 1 strict loop-free criterion), x-2 (no alternate over a link with forward/reverse cost LSInfinity), and x-3 (OSPF: exclude a neighbor whose every reverse link is LSInfinity) each carry positive+negative tags. x-4 (IS-IS overload-bit exclusion) is {not-applicable}: ze has no IS-IS LFA code path. x-5 (alternate only for shortest-path traffic) and x-6 (bound the alternate's lifetime) are {gap}: the backup is attached to every SPF route with no address-family guard so an OSPFv3 multicast AF route inherits it, and ze has no explicit RFC 5286 Section 4.1 hold-down/termination timer (only SPF reconvergence). Disclosed in the docs/features/rfc-status.md RFC 5286 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

LFA and TI-LFA fast reroute: per-neighbor SPFs, loop-free / node-protecting / downstream backup selection, SR repair lists, multi-area suppression.

**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc5286.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5286.md): the LFA backup is attached to every SPF route regardless of address family, so an OSPFv3 multicast AF (RFC 5838) route inherits it (RFC5286-x-5); and no explicit Section 4.1 hold-down timer bounds how long an alternate stays active, only SPF reconvergence (RFC5286-x-6). IS-IS has no LFA.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5286-x-1`](#rfc5286-x-1), [`RFC5286-x-2`](#rfc5286-x-2), [`RFC5286-x-3`](#rfc5286-x-3)

**Annotated instead of tested (3):** [`RFC5286-x-4`](#rfc5286-x-4), [`RFC5286-x-5`](#rfc5286-x-5), [`RFC5286-x-6`](#rfc5286-x-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5286-x-1` | Alternate next hops used by implementations following this specification MUST conform to at least the loop-freeness condition stated above in Inequality 1. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22). **negative:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) |
| `RFC5286-x-2` | For computing an alternate, a router MUST NOT use an alternate next- hop that is along a link whose cost or reverse cost is LSInfinity (for OSPF) or the maximum cost (for IS-IS) or that has the overload bit set (for IS-IS). (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60). **positive:** `unit/verify` [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L26). **negative:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66). **negative:** `unit/verify` [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L32) |
| `RFC5286-x-3` | In the case of OSPF, if all links from router S to a neighbor N_i have a reverse cost of LSInfinity, then router S MUST NOT use N_i as an alternate. (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L59). **positive:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99). **negative:** `unit/verify` [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L72). **negative:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114) |
| `RFC5286-x-4` | Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go); IS-IS has no LFA/backup-next-hop computation code path, so the IS-IS overload-bit exclusion has nothing to apply to |
| `RFC5286-x-5` | The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| `RFC5286-x-6` | A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze relies on SPF reconvergence to replace a route with a stale alternate (internal/plugins/ospf/spf/install.go:171-195) and the kernel RTNH_F_LINKDOWN flag (internal/plugins/fib/kernel/nexthop_linux.go:113), but implements no explicit RFC 5286 Section 4.1 hold-down timer or termination-condition bounding how long an alternate stays active. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| `RFC5286-x-7` | Since the functionality of link-and-node-protecting LFAs is greater than that of link-protecting downstream paths, a router SHOULD select a link-and-node-protecting LFA over a link-protecting downstream path. (§1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-8` | Therefore, it SHOULD be assumed that an alternate next-hop does not offer node protection if Inequality 3 is not met. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-9` | If the primary next-hop uses a broadcast link, then an alternate SHOULD be loop-free with respect to that link's pseudo-node (PN) to provide link protection. This requirement is described in Inequality 4 below. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-10` | A router SHOULD NOT specify the "local protection available" flag as a result of having LFAs. (§3.5.1) | SHOULD NOT | 3.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-11` | A router SHOULD NOT use an alternate next-hop that is along a link for which the link has been advertised with the attribute "link excluded from local protection path" or with the attribute "local maintenance required". (§3.5.1) | SHOULD NOT | 3.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-12` | A router supporting this specification SHOULD attempt to select at least one loop-free alternate next-hop for each primary next-hop used for a given prefix. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-13` | S SHOULD select a loop-free node-protecting alternate next-hop, if one is available. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-14` | If S has a choice between a loop-free link-and-node-protecting alternate and a loop-free node-protecting alternate that is not link-protecting, S SHOULD select a loop-free link-and-node- protecting alternate. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-15` | If S has multiple primary next-hops, then S SHOULD select as a loop-free alternate either one of the other primary next-hops or a loop-free node-protecting alternate if available. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-16` | If no loop- free node-protecting alternate is available and no other primary next-hop can provide link-protection, then S SHOULD select a loop-free link-protecting alternate. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-17` | Implementations SHOULD support a mode where other primary next- hops satisfying the basic loop-free condition and providing at least link or node protection are preferred over any non-primary alternates. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-18` | When a next-hop failure is detected via a local interface failure or other failure detection mechanisms (see [FRAMEWORK]), the router SHOULD: 1. Remove the primary next-hop associated with the failure. 2. Install the loop-free alternate calculated for the failed next- hop if it is not already installed (e.g., the alternate is also a primary next-hop). (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-19` | A router that implements [MICROLOOP] SHOULD follow the rules given there for terminating the use of an alternate. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-20` | A router that implements [ORDERED-FIB] SHOULD follow the rules given there for terminating the use of an alternate. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-21` | An implementation SHOULD continue to use the alternate next-hops for packet forwarding even after the new routing information is available based on the new network topology. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-22` | The use of the alternate next- hops for packet forwarding SHOULD terminate: a. if the new primary next-hop was loop-free prior to the topology change, or b. if a configured hold-down, which represents a worst-case bound on the length of the network convergence transition, has expired, or c. if notification of an unrelated topological change in the network is received. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-23` | A router SHOULD compute the alternate next-hop for an IGP multi-homed prefix by considering alternate paths via all routers that have announced that prefix. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-24` | When a forwarding address is set in an OSPF AS-external Link State Advertisement (LSA), all routers in the network calculate their next- hops for the external prefix by doing a lookup for the forwarding address in the routing table, rather than using the next-hops calculated for the ASBR. In this case, the alternate next-hops SHOULD be computed by selecting among the alternate paths to the forwarding link(s) instead of among alternate paths to the ASBR. (§6.3.1) | SHOULD | 6.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-25` | The alternate next-hops SHOULD NOT be used for multicast Reverse Path Forwarding (RPF) checks. (§6.5) | SHOULD NOT | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-26` | A router MAY decide to not use an available loop-free alternate next-hop. A reason for such a decision might be that the loop-free alternate next-hop does not provide protection for the failure scenario of interest. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-27` | If no loop-free node-protecting alternate is available, then S MAY select a loop-free link-protecting alternate. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-28` | Implementations considering SRLGs MAY use SRLG protection to determine that a node-protecting or link-protecting alternate is not available for use. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-29` | Note that the router MAY remove other next-hops if it believes (via SRLG analysis) that they may have been affected by the same failure, even if it is not visible at the time of failure detection. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-30` | In all cases, a router MAY safely simplify the multi-homed prefix (MHP) calculation by assuming that the MHP is solely attached to the router that was its pre-failure optimal point of attachment. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5286-x-4`](#rfc5286-x-4) Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go); IS-IS has no LFA/backup-next-hop computation code path, so the IS-IS overload-bit exclusion has nothing to apply to |
| [`RFC5286-x-5`](#rfc5286-x-5) The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4) | {gap}, no test | ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| [`RFC5286-x-6`](#rfc5286-x-6) A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1) | {gap}, no test | ze relies on SPF reconvergence to replace a route with a stale alternate (internal/plugins/ospf/spf/install.go:171-195) and the kernel RTNH_F_LINKDOWN flag (internal/plugins/fib/kernel/nexthop_linux.go:113), but implements no explicit RFC 5286 Section 4.1 hold-down timer or termination-condition bounding how long an alternate stays active. Disclosed in the docs/features/rfc-status.md RFC 5286 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5286-x-1`](#rfc5286-x-1)

Alternate next hops used by implementations following this specification MUST conform to at least the loop-freeness condition stated above in Inequality 1. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an alternate N with D_opt(N,D) >= D_opt(N,S)+D_opt(S,D). TestRFC5286LoopFreeInequality1 negative builds the equality case D(N,D)=21 == 10+11 with finite forward and reverse cost, and the `if _, ok := runSelect(...); ok { t.Fatalf(...) }` goes red on any selectLFA that accepts it (a <= or a missing check). Positive 11 < 21 asserts the backup is returned with next-hop N. Isolated: no other gate trips on either input. Unit-level on selectLFA (lfa.go), which is the production LFA selector.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22) | unit/verify | unproven |

### [`RFC5286-x-2`](#rfc5286-x-2)

For computing an alternate, a router MUST NOT use an alternate next- hop that is along a link whose cost or reverse cost is LSInfinity (for OSPF) or the maximum cost (for IS-IS) or that has the overload bit set (for IS-IS). (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an alternate over an OSPF link whose forward or reverse cost is costed out. A Router-LSA link metric is 16 bits, so the costed-out value is MaxLinkMetric 0xffff (RFC 6987 Section 3). selectLFA (spf/lfa.go) now gates forwardCost and reverseCost on spf.MaxLinkMetric = 0xffff. TestRFC5286CostedOutLinkMetric drives selectLFA with forward cost 0xffff and reverse cost 0xffff (each must return no backup) and with 0xfffe on both (must return one), so a gate on the 24-bit LSInfinity or a missing gate goes red. The older TestRFC5286CostReverseCostGate still injects 0x00ffffff, a value no link carries; it stays true and adds nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60) | unit/verify | unproven |

### [`RFC5286-x-3`](#rfc5286-x-3)

In the case of OSPF, if all links from router S to a neighbor N_i have a reverse cost of LSInfinity, then router S MUST NOT use N_i as an alternate. (§3.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27. Forbidden: N_i as an alternate when every link from S to N_i has reverse cost LSInfinity. Asserted for point-to-point links only: TestRFC5286AllReverseLinksCostedOut and TestRFC5286ReverseCostAllInfinite derive the reverse cost with reverseP2PCost (spf/lfa.go) over parallel P2P links back to S, all 0xffff or none, and require no backup, with one finite link requiring the backup. A neighbour reached over a transit (broadcast) network takes its reverse cost from reverseTransitCost, which no tagged unit drives at 0xffff, so 'all links' is proven for one of the two link kinds lfa.go enumerates. IS-IS computes no LFA in ze, so the row binds OSPF only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L72) | unit/verify | revert, verified |
| negative | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114) | unit/verify | unproven |
| positive | [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99) | unit/verify | unproven |

### [`RFC5286-x-4`](#rfc5286-x-4)

Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-4, so no unit is bound to it.

### [`RFC5286-x-5`](#rfc5286-x-5)

The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-5, so no unit is bound to it.

### [`RFC5286-x-6`](#rfc5286-x-6)

A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-6, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5286.txt |
| Source fingerprint | 65181f8363c53cdf |
| Record | rfc/extraction/rfc5286.json |
| Mapped sentences | 6 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 3 | walked | not stated |
| `3.5.1` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `3.8` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.3.1` | not stated | 0 | walked | not stated |
| `6.3.2` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5286 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5286, so its obligations are stated where they were written.
