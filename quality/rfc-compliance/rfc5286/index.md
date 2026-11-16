# RFC 5286 - Basic Specification for IP Fast Reroute: Loop-Free Alternates

Loop-Free Alternate FRR. Every requirement listed for RFC 5286, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 8 | 8 of 12 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 3 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 30 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Loop-Free Alternate FRR |
| Automatic checks | Included |
| Requirements | 30 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tests linked to requirements | 12 |
| Recorded audit verdicts | 3 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc5286.md` |
| Requirement table | `rfc/requirements/rfc5286.md` |
| RFC text | `rfc/full/rfc5286.txt` |

## Automatic checks

Included in automatic checks: IP Fast Reroute: Loop-Free Alternates (LFA): six MUST-level requirements, computed in OSPF only (internal/plugins/ospf/spf/lfa.go). x-1 (Inequality 1 strict loop-free criterion), x-2 (no alternate over a link with forward/reverse cost LSInfinity), and x-3 (OSPF: exclude a neighbor whose every reverse link is LSInfinity) each carry positive+negative tags. x-4 (IS-IS overload-bit exclusion) is {gap}: ze has no IS-IS LFA code path. x-5 (alternate only for shortest-path traffic) and x-6 (bound the alternate's lifetime) are {gap}: the backup is attached to every SPF route with no address-family guard so an OSPFv3 multicast AF route inherits it, and ze programs no alternate into the Linux kernel, so none is used after a primary failure (only SPF reconvergence), and has no RFC 5286 Section 4.1 hold-down/termination timer. Disclosed in the docs/features/rfc-status.md RFC 5286 row.

## Published support status

**Area:** Loop-Free Alternate FRR

**Test verification:** 2 of 6 MUST proven

**Evidence:** unit 2

**Unmet:** RFC5286-x-4, RFC5286-x-5, RFC5286-x-6


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 3 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC5286-x-1`](#rfc5286-x-1), [`RFC5286-x-2`](#rfc5286-x-2), [`RFC5286-x-3`](#rfc5286-x-3)

**Annotated (including scoped evidence) (3):** [`RFC5286-x-4`](#rfc5286-x-4), [`RFC5286-x-5`](#rfc5286-x-5), [`RFC5286-x-6`](#rfc5286-x-6)

## Figures

- **MUST bar**: Ze verified 2, Tests awaiting verification 1, Not built 3
- **Overview bar**: Ze MUST verified 2, Tests awaiting verification 1, Not assessed 24, Not built 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 2 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 3 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 24 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 24 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5286.json](../../../data/rfc/rfc5286.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5286-x-1` | Alternate next hops used by implementations following this specification MUST conform to at least the loop-freeness condition stated above in Inequality 1. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22). **negative:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) |
| `RFC5286-x-2` | For computing an alternate, a router MUST NOT use an alternate next- hop that is along a link whose cost or reverse cost is LSInfinity (for OSPF) or the maximum cost (for IS-IS) or that has the overload bit set (for IS-IS). (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60). **positive:** `unit/verify` [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L26). **negative:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66). **negative:** `unit/verify` [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L32) |
| `RFC5286-x-3` | In the case of OSPF, if all links from router S to a neighbor N_i have a reverse cost of LSInfinity, then router S MUST NOT use N_i as an alternate. (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L59). **positive:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99). **positive:** `unit/verify` [`TestRFC5286TransitNeighborFiniteReverseIsAlternate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_transit_reverse_test.go#L70). **negative:** `unit/verify` [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L72). **negative:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114). **negative:** `unit/verify` [`TestRFC5286TransitNeighborCostedOutReverseNotAlternate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_transit_reverse_test.go#L85) |
| `RFC5286-x-4` | Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5) | MUST NOT | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go) and implements no IS-IS LFA, with no owner decision declining it, so no IS-IS computation excludes an overloaded neighbor |
| `RFC5286-x-5` | The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| `RFC5286-x-6` | A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze computes the alternate (internal/plugins/ospf/spf/lfa.go) and carries it on the Loc-RIB Path through sysrib to fib-kernel's own state, but fib-kernel does not program it into the Linux kernel (buildRichRoute in internal/plugins/fib/kernel/nexthop_linux.go sends the primary next-hops only), so no alternate is ever in use after a primary failure and traffic recovers only through SPF reconvergence (internal/plugins/ospf/spf/install.go). There is nothing to time-limit yet: the requirement becomes meaningful once a backup is activated on primary failure, and ze then also owes a Section 4.1 hold-down or termination condition. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
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
| [`RFC5286-x-4`](#rfc5286-x-4) Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5) | {gap}, no test | ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go) and implements no IS-IS LFA, with no owner decision declining it, so no IS-IS computation excludes an overloaded neighbor |
| [`RFC5286-x-5`](#rfc5286-x-5) The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4) | {gap}, no test | ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| [`RFC5286-x-6`](#rfc5286-x-6) A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1) | {gap}, no test | ze computes the alternate (internal/plugins/ospf/spf/lfa.go) and carries it on the Loc-RIB Path through sysrib to fib-kernel's own state, but fib-kernel does not program it into the Linux kernel (buildRichRoute in internal/plugins/fib/kernel/nexthop_linux.go sends the primary next-hops only), so no alternate is ever in use after a primary failure and traffic recovers only through SPF reconvergence (internal/plugins/ospf/spf/install.go). There is nothing to time-limit yet: the requirement becomes meaningful once a backup is activated on primary failure, and ze then also owes a Section 4.1 hold-down or termination condition. Disclosed in the docs/features/rfc-status.md RFC 5286 row |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5286-x-1`](#rfc5286-x-1)

Alternate next hops used by implementations following this specification MUST conform to at least the loop-freeness condition stated above in Inequality 1. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an alternate N with D_opt(N,D) >= D_opt(N,S)+D_opt(S,D). TestRFC5286LoopFreeInequality1 negative builds the equality case D(N,D)=21 == 10+11 with finite forward and reverse cost, and the `if _, ok := runSelect(...); ok { t.Fatalf(...) }` goes red on any selectLFA that accepts it (a <= or a missing check). Positive 11 < 21 asserts the backup is returned with next-hop N. Isolated: no other gate trips on either input. Unit-level on selectLFA (lfa.go), which is the production LFA selector.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) | unit/verify | not verified |
| positive | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22) | unit/verify | not verified |

### [`RFC5286-x-2`](#rfc5286-x-2)

For computing an alternate, a router MUST NOT use an alternate next- hop that is along a link whose cost or reverse cost is LSInfinity (for OSPF) or the maximum cost (for IS-IS) or that has the overload bit set (for IS-IS). (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an alternate over an OSPF link whose forward or reverse cost is costed out. A Router-LSA link metric is 16 bits, so the costed-out value is MaxLinkMetric 0xffff (RFC 6987 Section 3). selectLFA (spf/lfa.go) now gates forwardCost and reverseCost on spf.MaxLinkMetric = 0xffff. TestRFC5286CostedOutLinkMetric drives selectLFA with forward cost 0xffff and reverse cost 0xffff (each must return no backup) and with 0xfffe on both (must return one), so a gate on the 24-bit LSInfinity or a missing gate goes red. The older TestRFC5286CostReverseCostGate still injects 0x00ffffff, a value no link carries; it stays true and adds nothing.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66) | unit/verify | not verified |
| positive | [`TestRFC5286CostedOutLinkMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60) | unit/verify | not verified |

### [`RFC5286-x-3`](#rfc5286-x-3)

In the case of OSPF, if all links from router S to a neighbor N_i have a reverse cost of LSInfinity, then router S MUST NOT use N_i as an alternate. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (independent judge, ospf c25). Forbidden: N_i as an alternate when every link from S to N_i has reverse cost LSInfinity. Both link kinds lfa.go enumerates are now driven. Point-to-point: TestRFC5286AllReverseLinksCostedOut and TestRFC5286ReverseCostAllInfinite (reverseP2PCost over parallel links, all 0xffff refused, one finite accepted). Transit: TestRFC5286TransitNeighborCostedOutReverseNotAlternate builds a graph where N's only link back to S is its transit link onto the shared LAN at 0xffff, runs the real neighborLinks enumeration (reverseTransitCost) and selectLFA over its candidates, and requires the candidate's reverse cost 0xffff and no alternate although N is otherwise loop-free (10 < 10+15); TestRFC5286TransitNeighborFiniteReverseIsAlternate is the same graph at 10 and requires N selected. Overlays on reverseTransitCost (return 10, return 0xffff) each redden only the matching new unit (author o2, o3); observed-red records on reverseTransitCost and selectLFA, both polarities. The gate is per candidate link, which never uses N when all links are costed out. IS-IS computes no LFA in ze, so the row binds OSPF only.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L72) | unit/verify | revert, verified |
| negative | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestRFC5286TransitNeighborCostedOutReverseNotAlternate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_transit_reverse_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestRFC5286AllReverseLinksCostedOut`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_costed_out_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC5286TransitNeighborFiniteReverseIsAlternate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_transit_reverse_test.go#L70) | unit/verify | revert, verified |

### [`RFC5286-x-4`](#rfc5286-x-4)

Similarly in the case of IS-IS, if N_i has the overload bit set, then S MUST NOT consider using N_i as an alternate. (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5286-x-4, so there is no linked test.

### [`RFC5286-x-5`](#rfc5286-x-5)

The alternate next-hop MUST be used only for traffic types that are routed according to the shortest path. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5286-x-5, so there is no linked test.

### [`RFC5286-x-6`](#rfc5286-x-6)

A router MUST limit the amount of time an alternate next-hop is used after the primary next-hop has become unavailable. (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5286-x-6, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 1 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 3 | reviewed | not stated |
| `3.5.1` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 0 | reviewed | not stated |
| `3.7` | not stated | 0 | reviewed | not stated |
| `3.8` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `6.3.1` | not stated | 0 | reviewed | not stated |
| `6.3.2` | not stated | 0 | reviewed | not stated |
| `6.4` | not stated | 0 | reviewed | not stated |
| `6.5` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 5286 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 5286, so its obligations are stated where they were written.
