# RFC 5286 - Basic Specification for IP Fast Reroute: Loop-Free Alternates

Experimental. Every requirement this repository extracted from RFC 5286, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.0% | 3 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 40.0% | 2 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 30 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 7 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5286-x-1`](#rfc5286-x-1), [`RFC5286-x-2`](#rfc5286-x-2), [`RFC5286-x-3`](#rfc5286-x-3)

**Annotated instead of tested (3):** [`RFC5286-x-4`](#rfc5286-x-4), [`RFC5286-x-5`](#rfc5286-x-5), [`RFC5286-x-6`](#rfc5286-x-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5286-x-1` | Alternate next-hops MUST conform to at least the loop-freeness | MUST | x | **positive:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22). **negative:** `unit/verify` [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) |
| `RFC5286-x-2` | For computing an alternate, a router MUST NOT use an alternate | MUST NOT | x | **positive:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60). **negative:** `unit/verify` [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66) |
| `RFC5286-x-3` | In OSPF, if all links from S to a neighbor N_i have reverse | MUST NOT | x | **positive:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99). **negative:** `unit/verify` [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114) |
| `RFC5286-x-4` | In IS-IS, if N_i has the overload bit set, S MUST NOT consider using N_i as an alternate | MUST NOT | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go); IS-IS has no LFA/backup-next-hop computation code path, so the IS-IS overload-bit exclusion has nothing to apply to |
| `RFC5286-x-5` | The alternate next-hop MUST be used only for traffic types routed according to the shortest path | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| `RFC5286-x-6` | A router MUST limit the amount of time an alternate next-hop is used after the primary becomes unavailable | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze relies on SPF reconvergence to replace a route with a stale alternate (internal/plugins/ospf/spf/install.go:171-195) and the kernel RTNH_F_LINKDOWN flag (internal/plugins/fib/kernel/nexthop_linux.go:113), but implements no explicit RFC 5286 Section 4.1 hold-down timer or termination-condition bounding how long an alternate stays active. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| `RFC5286-x-7` | A router SHOULD select a link-and-node-protecting LFA over a | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-8` | It SHOULD be assumed that an alternate offers no node protection | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-9` | If the primary next-hop uses a broadcast link, an alternate | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-10` | A router SHOULD NOT specify the "local protection available" | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-11` | A router SHOULD NOT use an alternate next-hop along a link | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-12` | A router supporting this specification SHOULD attempt to select | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-13` | S SHOULD select a loop-free node-protecting alternate if one is | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-14` | Given a choice between a link-and-node-protecting alternate and a | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-15` | With multiple primary next-hops, S SHOULD select as alternate | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-16` | If no node-protecting alternate and no other primary can provide | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-17` | Implementations SHOULD support a mode preferring other primary | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-18` | On primary next-hop failure the router SHOULD remove the failed | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-19` | A router implementing [MICROLOOP] SHOULD follow that document's | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-20` | A router implementing [ORDERED-FIB] SHOULD follow that document's | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-21` | An implementation SHOULD continue to use the alternate next-hops | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-22` | Use of the alternate next-hops SHOULD terminate when (a) the new | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-23` | A router SHOULD compute the alternate next-hop for an IGP | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-24` | For OSPF external routes with a forwarding address set, alternate | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-25` | Alternate next-hops SHOULD NOT be used for multicast Reverse | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-26` | A router MAY decide not to use an available loop-free alternate | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-27` | If no node-protecting alternate is available, S MAY select a | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-28` | Implementations considering SRLGs MAY use SRLG protection to | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-29` | The router MAY remove other next-hops it believes (via SRLG | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC5286-x-30` | A router MAY safely simplify the multi-homed-prefix calculation by | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5286-x-4`](#rfc5286-x-4) In IS-IS, if N_i has the overload bit set, S MUST NOT consider using N_i as an alternate | no test | no test carries this requirement id; annotated {not-applicable}: ze computes Loop-Free Alternates only in OSPF (internal/plugins/ospf/spf/lfa.go); IS-IS has no LFA/backup-next-hop computation code path, so the IS-IS overload-bit exclusion has nothing to apply to |
| [`RFC5286-x-5`](#rfc5286-x-5) The alternate next-hop MUST be used only for traffic types routed according to the shortest path | {gap}, no test | ze attaches the RFC 5286 alternate to a route's Loc-RIB Path unconditionally in Installer.insert (internal/plugins/ospf/spf/install.go:213-229) with no guard confining it to unicast shortest-path forwarding; an OSPFv3 multicast address-family engine (family.IPv4Multicast / family.IPv6Multicast, internal/plugins/ospf/multiaf.go:102-115) installs through the same NewInstallerFamily path (internal/plugins/ospf/spf_wiring.go:34) and inherits fast-reroute config (internal/plugins/ospf/config.go:709-711), so a multicast-AF route receives the same backup next-hop, which Section 4 confines to shortest-path traffic and Section 6.5 excludes from multicast RPF. Disclosed in the docs/features/rfc-status.md RFC 5286 row |
| [`RFC5286-x-6`](#rfc5286-x-6) A router MUST limit the amount of time an alternate next-hop is used after the primary becomes unavailable | {gap}, no test | ze relies on SPF reconvergence to replace a route with a stale alternate (internal/plugins/ospf/spf/install.go:171-195) and the kernel RTNH_F_LINKDOWN flag (internal/plugins/fib/kernel/nexthop_linux.go:113), but implements no explicit RFC 5286 Section 4.1 hold-down timer or termination-condition bounding how long an alternate stays active. Disclosed in the docs/features/rfc-status.md RFC 5286 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5286-x-1`](#rfc5286-x-1)

Alternate next-hops MUST conform to at least the loop-freeness

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC5286LoopFreeInequality1`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L22) | unit/verify | unproven |

### [`RFC5286-x-2`](#rfc5286-x-2)

For computing an alternate, a router MUST NOT use an alternate

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC5286CostReverseCostGate`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L60) | unit/verify | unproven |

### [`RFC5286-x-3`](#rfc5286-x-3)

In OSPF, if all links from S to a neighbor N_i have reverse

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L114) | unit/verify | unproven |
| positive | [`TestRFC5286ReverseCostAllInfinite`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/spf/rfc5286_lfa_test.go#L99) | unit/verify | unproven |

### [`RFC5286-x-4`](#rfc5286-x-4)

In IS-IS, if N_i has the overload bit set, S MUST NOT consider using N_i as an alternate

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-4, so no unit is bound to it.

### [`RFC5286-x-5`](#rfc5286-x-5)

The alternate next-hop MUST be used only for traffic types routed according to the shortest path

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-5, so no unit is bound to it.

### [`RFC5286-x-6`](#rfc5286-x-6)

A router MUST limit the amount of time an alternate next-hop is used after the primary becomes unavailable

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5286-x-6, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5286, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5286, so its obligations are stated where they were written.
