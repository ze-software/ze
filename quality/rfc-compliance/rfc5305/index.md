# RFC 5305 - IS-IS Extensions for Traffic Engineering

Experimental. Every requirement this repository extracted from RFC 5305, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 64.3% | 9 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 83.3% | 20 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 22 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 35.7% | 5 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 22 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 9 |
| Discrimination records | 20 |
| Summary | `rfc/short/rfc5305.md` |
| Requirement shard | `rfc/requirements/rfc5305.md` |
| RFC text | `rfc/full/rfc5305.txt` |

## Enrolment

Enrolled: IS-IS Extensions for Traffic Engineering (TLV 22 extended IS reachability, TLV 135 extended IP reachability): eight MUST-level requirements. Five are met with positive+negative tags in internal/plugins/isis: 3-1 (a maximum-metric link is not considered in normal SPF), 4-1 (do not install a route at or above MaxPathMetric), 4.1-1 (set the up/down bit correctly on a down-leak), 2-1 (retain unknown sub-TLVs without rejecting). 3.2-2, 4.3-1 are {not-applicable}: ze originates no IS-IS TE sub-TLVs (6/8) or TLV 134. On 2026-09-30 the {not-applicable} markers came off 3.2-1, 3.3-1 and 4.3-4: those rows ban injecting a /32 for an address received in sub-TLV 6, sub-TLV 8 or TLV 134, Ze fills that receiving role (the BGP-LS export decodes all three), and TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes and TestRFC5305TEAddressesInjectNoHostRoute prove from a received LSP that only the reachability prefix is installed and no /32 for any of the three addresses reaches the Loc-RIB or a BGP-LS Prefix NLRI.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

- Extended IS reachability (TLV 22) and extended IPv4 reachability (TLV 135) with wide metrics and the up/down bit
- unknown sub-TLVs retained
- tests bound per requirement in [`rfc/requirements/rfc5305.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5305.md).


**What the ledger says remains:**

TE sub-TLVs (6/8) and TLV 134 (TE Router ID) are not implemented (no IS-IS TE).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC5305-3-1`](#rfc5305-3-1), [`RFC5305-4-1`](#rfc5305-4-1), [`RFC5305-4.1-1`](#rfc5305-4.1-1), [`RFC5305-4.1-2`](#rfc5305-4.1-2), [`RFC5305-4.2-1`](#rfc5305-4.2-1), [`RFC5305-3.2-1`](#rfc5305-3.2-1), [`RFC5305-3.3-1`](#rfc5305-3.3-1), [`RFC5305-4.3-4`](#rfc5305-4.3-4), [`RFC5305-2-1`](#rfc5305-2-1)

**Annotated (including scoped evidence) (5):** [`RFC5305-4.1-3`](#rfc5305-4.1-3), [`RFC5305-3.7-1`](#rfc5305-3.7-1), [`RFC5305-3.2-2`](#rfc5305-3.2-2), [`RFC5305-3.3-2`](#rfc5305-3.3-2), [`RFC5305-4.3-1`](#rfc5305-4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5305-3-1` | If a link is advertised with the maximum link metric (2^24 - 1), this link MUST NOT be considered during the normal SPF computation. (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L315). **negative:** `unit/verify` [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L316) |
| `RFC5305-4-1` | If a prefix is advertised with a metric larger then MAX_PATH_METRIC (0xFE000000, see paragraph 3.0), this prefix MUST NOT be considered during the normal SPF computation. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L293). **positive:** `unit/verify` [`TestRFC5305PrefixAboveMaxPathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L78). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L298). **negative:** `unit/verify` [`TestRFC5305PrefixAboveMaxPathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L80) |
| `RFC5305-4.1-1` | If a prefix is advertised from a higher level to a lower level (e.g., level 2 to level 1), the bit MUST be set to 1, indicating that the prefix has traveled down the hierarchy. Prefixes that have the up/down bit set to 1 may only be advertised down the hierarchy, i.e., to lower levels. These semantics apply even if IS-IS is extended in the future to have additional levels. By ensuring that prefixes follow only the IS-IS hierarchy, we have ensured that the information does not loop, thereby ensuring that there are no persistent forwarding loops. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L258). **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L69). **negative:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L270). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L72). **negative:** `unit/verify` [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_consumer_test.go#L169) |
| `RFC5305-4.1-3` | If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1. (§4.1) | SHALL | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the condition never holds in Ze. A Ze IS-IS instance belongs to one Level 1 area, and internal/plugins/isis/spf/leak.go LeakPrefixes moves prefixes only between levels (IntoL1 from Level 2, IntoL2 from Level 1); no producer advertises a prefix from one area to another at the same level |
| `RFC5305-3.7-1` | If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep configured routes" for TE path computation. The conditional subject is "traffic engineering SPF calculations"; internal/plugins/rsvpte/register.go parseERO, routing.go resolveExplicitPath and frr.go selectBypass consume configured routes, native next-hop lookups and configured bypasses. internal/plugins/isis/spf/spf.go computes normal SPF. There is no CSPF/TE shortest-path consumer to apply this fallback. This exclusion does not cover native TE advertisement or wire duties, and BGP-LS export is not TE-SPF |
| `RFC5305-4.1-2` | The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC5305FirstInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_inject_test.go#L15). **negative:** `unit/verify` [`TestRFC5305ConnectedInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_inject_test.go#L37) |
| `RFC5305-4.2-1` | If there are no sub-TLVs associated with a prefix, the bit indicating the presence of sub-TLVs SHALL be set to 0 (Section 4.2) | SHALL | 4.2 | **positive:** `unit/verify` [`TestRFC5305NoSubTLVsClearsPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_presence_test.go#L24). **negative:** `unit/verify` [`TestRFC5305SubTLVsSetPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_presence_test.go#L39) |
| `RFC5305-3.2-1` | Implementations MUST NOT inject a /32 prefix for the interface address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L83). **negative:** `unit/verify` [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L105) |
| `RFC5305-3.2-2` | If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| `RFC5305-3.3-1` | Implementations MUST NOT inject a /32 prefix for the neighbor address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L84). **negative:** `unit/verify` [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L106) |
| `RFC5305-3.3-2` | If a router implements traffic engineering, it MUST include this sub- TLV on point-to-point adjacencies. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| `RFC5305-4.3-4` | Implementations MUST NOT inject a /32 prefix for the router ID into their forwarding table because this can lead to forwarding loops when interacting with systems that do not support this TLV. (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L85). **negative:** `unit/verify` [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L107) |
| `RFC5305-3.4-1` | This sub-TLV is optional. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-3.5-1` | This sub-TLV is optional. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-3.6-2` | This sub-TLV is optional. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-3.7-2` | This sub-TLV is optional. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-1` | If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |
| `RFC5305-2-1` | Unknown sub-TLVs are to be ignored and skipped upon receipt. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_core_test.go#L185). **positive:** `unit/verify` [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_test.go#L81). **negative:** `unit/verify` [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_core_test.go#L205) |
| `RFC5305-3.1-1` | This sub-TLV is OPTIONAL. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-2` | This TLV SHOULD not be included more than once in an LSP. (§4.3) | SHOULD NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-3.6-1` | For stability reasons, rapid changes in the values in this sub-TLV SHOULD NOT cause rapid generation of LSPs. (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-3` | If a router advertises the Traffic Engineering router ID TLV in its LSP, and if it advertises prefixes via the Border Gateway Protocol (BGP) with the BGP next hop attribute set to the BGP router ID, the Traffic Engineering router ID SHOULD be the same as the BGP router ID. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5305-4.1-3`](#rfc5305-4.1-3) If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the condition never holds in Ze. A Ze IS-IS instance belongs to one Level 1 area, and internal/plugins/isis/spf/leak.go LeakPrefixes moves prefixes only between levels (IntoL1 from Level 2, IntoL2 from Level 1); no producer advertises a prefix from one area to another at the same level |
| [`RFC5305-3.7-1`](#rfc5305-3.7-1) If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep configured routes" for TE path computation. The conditional subject is "traffic engineering SPF calculations"; internal/plugins/rsvpte/register.go parseERO, routing.go resolveExplicitPath and frr.go selectBypass consume configured routes, native next-hop lookups and configured bypasses. internal/plugins/isis/spf/spf.go computes normal SPF. There is no CSPF/TE shortest-path consumer to apply this fallback. This exclusion does not cover native TE advertisement or wire duties, and BGP-LS export is not TE-SPF |
| [`RFC5305-3.2-2`](#rfc5305-3.2-2) If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| [`RFC5305-3.3-2`](#rfc5305-3.3-2) If a router implements traffic engineering, it MUST include this sub- TLV on point-to-point adjacencies. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| [`RFC5305-4.3-1`](#rfc5305-4.3-1) If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5305-3-1`](#rfc5305-3-1)

If a link is advertised with the maximum link metric (2^24 - 1), this link MUST NOT be considered during the normal SPF computation. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: a link advertised at 2^24-1 being walked by normal SPF. TestISISSPFMaxLinkMetricExcluded builds A-B at 16777215 and t.Errorf fires if B or C appears in the SPF result; the positive half proves 16777214 is still relaxed (B at 16777214, C at 16777224).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L316) | unit/verify | mutant, verified |
| positive | [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L315) | unit/verify | unproven |

### [`RFC5305-4-1`](#rfc5305-4-1)

If a prefix is advertised with a metric larger then MAX_PATH_METRIC (0xFE000000, see paragraph 3.0), this prefix MUST NOT be considered during the normal SPF computation. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. §4 excludes a prefix 'advertised with a metric larger then MAX_PATH_METRIC'. spf TestRFC5305PrefixAboveMaxPathMetricNotConsidered drives BuildRoutes: node 3 (distance 10) advertises 10.9.0.0/24 at 50 (positive, installed at 60 via node 3 alone); node 2 (distance 10) advertises the same prefix at 0xFE000001 and 10.8.0.0/24 only at 0xFFFFFFFF (negative, strictly larger than MAX_PATH_METRIC): one next hop at 60 and the lone prefix absent. The path ceiling in route.go (total >= MaxPathMetric after clampMetric) is the guard; recorded +/- on BuildRoutes. Closure rejudge 2026-10-02: TestISISMetricWidth's positive claim was narrowed to what its body asserts (node B at edge 10 stays in the Compute result while its over-ceiling prefix drops; 'installable' dropped) and re-recorded on spf.go::Compute; its negative (prefix at exactly MAX_PATH_METRIC absent from BuildRoutes) keeps its record on BuildRoutes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305PrefixAboveMaxPathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L80) | unit/verify | revert, verified |
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L298) | unit/verify | revert, verified |
| positive | [`TestRFC5305PrefixAboveMaxPathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L293) | unit/verify | revert, verified |

### [`RFC5305-4.1-1`](#rfc5305-4.1-1)

If a prefix is advertised from a higher level to a lower level (e.g., level 2 to level 1), the bit MUST be set to 1, indicating that the prefix has traveled down the hierarchy. Prefixes that have the up/down bit set to 1 may only be advertised down the hierarchy, i.e., to lower levels. These semantics apply even if IS-IS is extended in the future to have additional levels. By ensuring that prefixes follow only the IS-IS hierarchy, we have ensured that the information does not loop, thereby ensuring that there are no persistent forwarding loops. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestISISEngineLeakOrigination and spf TestISISLeakOriginationL1L2 gained only RFC2966-2-4 tag comments; bodies byte-identical. Clause 1: l2Derived enters L2 with UpDown false and hasLeak(IntoL1, l2Derived, true) is required; the engine L1 LSP decodes it with the bit set. Clause 2: alreadyDown (UpDown true in L1) must be absent from IntoL2, isolated by l1Only from the same node leaking up. Recorded red on spf/leak.go leakInto.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L270) | unit/verify | revert, verified |
| negative | [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_consumer_test.go#L169) | unit/verify | revert, verified |
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L69) | unit/verify | revert, verified |

### [`RFC5305-4.1-3`](#rfc5305-4.1-3)

If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1. (§4.1)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. The condition, a prefix advertised from one area to another at the same level, never arises in Ze.

No test carries RFC5305-4.1-3, so no unit is bound to it.

### [`RFC5305-3.7-1`](#rfc5305-3.7-1)

If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.7-1, so no unit is bound to it.

### [`RFC5305-4.1-2`](#rfc5305-4.1-2)

The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305ConnectedInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_inject_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC5305FirstInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5305_inject_test.go#L15) | unit/verify | revert, verified |

### [`RFC5305-4.2-1`](#rfc5305-4.2-1)

If there are no sub-TLVs associated with a prefix, the bit indicating the presence of sub-TLVs SHALL be set to 0 (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305SubTLVsSetPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_presence_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC5305NoSubTLVsClearsPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_presence_test.go#L24) | unit/verify | revert, verified |

### [`RFC5305-3.2-1`](#rfc5305-3.2-1)

Implementations MUST NOT inject a /32 prefix for the interface address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30 under OWNER RULING 2 (a receive-side ban: the positive is Ze processing the LSP that carries the address, the negative is Ze's handling of the address it knows: no /32). Setup: a peer L1 LSP with TLV 22 to the root carrying sub-TLV 6 (203.0.113.1), sub-TLV 8, TLV 134 and a TLV 128 prefix goes through packet.DecodePDU and lsdb.Receive, SPF runs into a real Loc-RIB. Positive TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes: 198.51.100.0/24 installed via the neighbor and Loc-RIB Len == 1 (a /32 for the interface address would make it 2). Negative TestRFC5305TEAddressesInjectNoHostRoute: the BGP-LS builder decodes 203.0.113.1 as a link local address (so Ze holds the address the ban names), yet no 203.0.113.1/32 is in the Loc-RIB and no BGP-LS Prefix NLRI contains it. LIMIT, stated honestly: no Ze code path injects a /32 from sub-TLV 6 (spf/graph.go drops sub-TLVs), so no break of existing code can make the ban assertions red; the negative's observed red (linkAttribute disabled) comes from its decode precondition, and the positive's (BuildRoutes disabled) from the missing reachability route. The ban assertions are load-bearing only against future code that adds an injection. Accepted as enforced under R1(b): the input is forced to the exact trigger of the ban and the output complies.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L83) | unit/verify | revert, verified |

### [`RFC5305-3.2-2`](#rfc5305-3.2-2)

If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.2-2, so no unit is bound to it.

### [`RFC5305-3.3-1`](#rfc5305-3.3-1)

Implementations MUST NOT inject a /32 prefix for the neighbor address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30 under OWNER RULING 2, same setup as RFC5305-3.2-1. Positive TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes: with sub-TLV 8 (IPv4 neighbor address 203.0.113.2) on the received adjacency the Loc-RIB holds exactly 198.51.100.0/24 via the neighbor (Len == 1). Negative TestRFC5305TEAddressesInjectNoHostRoute: the BGP-LS builder decodes 203.0.113.2 as a link remote address, yet no 203.0.113.2/32 is in the Loc-RIB and no BGP-LS Prefix NLRI contains it. LIMIT: no Ze code path injects a /32 from sub-TLV 8, so no break of existing code reddens the ban assertions; the negative's observed red (linkAttribute disabled) comes from the decode precondition. Enforced under R1(b): input forced to the ban's trigger, output complies.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L84) | unit/verify | revert, verified |

### [`RFC5305-3.3-2`](#rfc5305-3.3-2)

If a router implements traffic engineering, it MUST include this sub- TLV on point-to-point adjacencies. (§3.3)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Judged 2026-09-30. Accepted as mirroring the HEAD siblings; under ruling R23 the stricter form for all three TE-conditional rows is an extraction exclusion feature-out-of-scope quoting the 'If a router implements traffic engineering' condition.

No test carries RFC5305-3.3-2, so no unit is bound to it.

### [`RFC5305-4.3-4`](#rfc5305-4.3-4)

Implementations MUST NOT inject a /32 prefix for the router ID into their forwarding table because this can lead to forwarding loops when interacting with systems that do not support this TLV. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30 under OWNER RULING 2, same setup as RFC5305-3.2-1. Positive TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes: with a TLV 134 router ID (203.0.113.9) in the received LSP the Loc-RIB holds exactly 198.51.100.0/24 (Len == 1). Negative TestRFC5305TEAddressesInjectNoHostRoute: bgplsBuilder.lsp exports 203.0.113.9 as node attribute 1028, yet no 203.0.113.9/32 is in the Loc-RIB (Ze's forwarding feed) and no BGP-LS Prefix NLRI contains it. LIMIT: no Ze code path injects a /32 from TLV 134, so no break of existing code reddens the ban assertions; the negative's observed red (bgplsBuilder.lsp disabled) comes from the export precondition. Enforced under R1(b): input forced to the ban's trigger, output complies.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305TEAddressesInjectNoHostRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestRFC5305TEAddressesLeaveOnlyReachabilityRoutes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5305_no_host_route_test.go#L85) | unit/verify | revert, verified |

### [`RFC5305-4.3-1`](#rfc5305-4.3-1)

If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-4.3-1, so no unit is bound to it.

### [`RFC5305-2-1`](#rfc5305-2-1)

Unknown sub-TLVs are to be ignored and skipped upon receipt. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Unchanged on re-read: TestISISTLVIPv4RoundTrip lost only its RFC1195-5.2-4 comment tag. Forbidden behaviour: rejecting a TLV that carries unknown sub-TLVs, or not skipping them so the next entry misparses. TestISISTLV22RoundTrip (tlv_core_test.go) keeps entry0's three uninterpreted sub-TLVs opaque and then asserts entry1's metric equals types.MaxMetric, which goes red on a wrong skip; TestISISTLVIPv4RoundTrip (with-subtlv) goes red if a type-1 sub-TLV is rejected. Negative TestISISTLV22Truncated proves an overrunning length is not skipped as absent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_core_test.go#L205) | unit/verify | unproven |
| positive | [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_core_test.go#L185) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5305_tlv_ipv4_test.go#L81) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc5305.txt |
| Source fingerprint | 1bb1a875678e75b2 |
| Record | rfc/extraction/rfc5305.json |
| Mapped sentences | 13 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `3.3` | not stated | 2 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 2 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on a traffic engineering SPF implementation, a feature Ze does not provide. RFC 5305 Section 3: 'To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC.' Section 3.7 repeats it for the TE Default metric sub-TLV. Ze runs no TE SPF: owner decision 2026-09-21 'Keep configured routes' for TE path computation (the same scope decision RFC5305-3.7-1 records), so RSVP-TE paths come from configured EROs, native next-hop lookups and configured bypasses (internal/plugins/rsvpte parseERO, resolveExplicitPath, selectBypass), and Ze decodes no TE metric sub-TLV. Ze's normal SPF saturates path sums at MAX_PATH_METRIC anyway (spf.go clampMetric), which RFC 5308 Section 5 obliges and RFC5308-5-2 carries. This is a SCOPE DECISION: TE SPF is an implementation gap, not a conformance gap. | To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC. |
| `3.7:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Conditional on a traffic engineering SPF implementation, a feature Ze does not provide. RFC 5305 Section 3: 'To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC.' Section 3.7 repeats it for the TE Default metric sub-TLV. Ze runs no TE SPF: owner decision 2026-09-21 'Keep configured routes' for TE path computation (the same scope decision RFC5305-3.7-1 records), so RSVP-TE paths come from configured EROs, native next-hop lookups and configured bypasses (internal/plugins/rsvpte parseERO, resolveExplicitPath, selectBypass), and Ze decodes no TE metric sub-TLV. Ze's normal SPF saturates path sums at MAX_PATH_METRIC anyway (spf.go clampMetric), which RFC 5308 Section 5 obliges and RFC5308-5-2 carries. This is a SCOPE DECISION: TE SPF is an implementation gap, not a conformance gap. | To preclude overflow within a traffic engineering SPF implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC. |

## Superseded

No document obsoletes RFC 5305, so its obligations are stated where they were written.
