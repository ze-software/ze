# RFC 5305 - IS-IS Extensions for Traffic Engineering

Experimental. Every requirement this repository extracted from RFC 5305, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 63.6% | 7 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 31.2% | 5 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 4 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 36.4% | 4 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 5 | of 11 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 15 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 5 |
| Discrimination records | 5 |
| Summary | `rfc/short/rfc5305.md` |
| Requirement shard | `rfc/requirements/rfc5305.md` |
| RFC text | `rfc/full/rfc5305.txt` |

## Enrolment

Enrolled: IS-IS Extensions for Traffic Engineering (TLV 22 extended IS reachability, TLV 135 extended IP reachability): eight MUST-level requirements. Five are met with positive+negative tags in internal/plugins/isis: 3-1 (a maximum-metric link is not considered in normal SPF), 3-2 (SHALL clamp/saturate the wide path metric), 4-1 (do not install a route at or above MaxPathMetric), 4.1-1 (set the up/down bit correctly on a down-leak), 2-1 (retain unknown sub-TLVs without rejecting). 3.2-1, 3.2-2, 4.3-1 are {not-applicable}: ze implements no IS-IS TE sub-TLVs (6/8) or TLV 134.

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
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5305-3-1`](#rfc5305-3-1), [`RFC5305-3-2`](#rfc5305-3-2), [`RFC5305-4-1`](#rfc5305-4-1), [`RFC5305-4.1-1`](#rfc5305-4.1-1), [`RFC5305-4.1-2`](#rfc5305-4.1-2), [`RFC5305-4.2-1`](#rfc5305-4.2-1), [`RFC5305-2-1`](#rfc5305-2-1)

**Annotated instead of tested (4):** [`RFC5305-3.7-1`](#rfc5305-3.7-1), [`RFC5305-3.2-1`](#rfc5305-3.2-1), [`RFC5305-3.2-2`](#rfc5305-3.2-2), [`RFC5305-4.3-1`](#rfc5305-4.3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5305-3-1` | If a link is advertised with the maximum link metric (2^24 - 1), this link MUST NOT be considered during the normal SPF computation. (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L317). **negative:** `unit/verify` [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L318) |
| `RFC5305-3-2` | To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC. It is easiest to select MAX_PATH_METRIC such that MAX_PATH_METRIC plus a single link metric does not overflow the number of bits for internal metric calculation. We assume that this is 32 bits. Therefore, we have chosen MAX_PATH_METRIC to be 4,261,412,864 (0xFE000000, 2^32 - 2^25). (§3) | SHALL | 3 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L252). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L253) |
| `RFC5305-4-1` | If a prefix is advertised with a metric larger then MAX_PATH_METRIC (0xFE000000, see paragraph 3.0), this prefix MUST NOT be considered during the normal SPF computation. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L295). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L300) |
| `RFC5305-4.1-1` | If a prefix is advertised from a higher level to a lower level (e.g., level 2 to level 1), the bit MUST be set to 1, indicating that the prefix has traveled down the hierarchy. Prefixes that have the up/down bit set to 1 may only be advertised down the hierarchy, i.e., to lower levels. These semantics apply even if IS-IS is extended in the future to have additional levels. By ensuring that prefixes follow only the IS-IS hierarchy, we have ensured that the information does not loop, thereby ensuring that there are no persistent forwarding loops. If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L259). **negative:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L270). **negative:** `unit/verify` [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/consumer_test.go#L169) |
| `RFC5305-3.7-1` | If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep configured routes" for TE path computation. The conditional subject is "traffic engineering SPF calculations"; internal/plugins/rsvpte/register.go parseERO, routing.go resolveExplicitPath and frr.go selectBypass consume configured routes, native next-hop lookups and configured bypasses. internal/plugins/isis/spf/spf.go computes normal SPF. There is no CSPF/TE shortest-path consumer to apply this fallback. This exclusion does not cover native TE advertisement or wire duties, and BGP-LS export is not TE-SPF |
| `RFC5305-4.1-2` | The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS (Section 4.1) | SHALL | 4.1 | **positive:** `unit/verify` [`TestRFC5305FirstInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/inject_rfc5305_test.go#L15). **negative:** `unit/verify` [`TestRFC5305ConnectedInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/inject_rfc5305_test.go#L37) |
| `RFC5305-4.2-1` | If there are no sub-TLVs associated with a prefix, the bit indicating the presence of sub-TLVs SHALL be set to 0 (Section 4.2) | SHALL | 4.2 | **positive:** `unit/verify` [`TestRFC5305NoSubTLVsClearsPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc5305_test.go#L24). **negative:** `unit/verify` [`TestRFC5305SubTLVsSetPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc5305_test.go#L39) |
| `RFC5305-3.2-1` | Implementations MUST NOT inject a /32 prefix for the interface address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never decodes the RFC 5305 TE sub-TLVs (sub-TLV 6/8) or TLV 134 into an address; internal/plugins/isis/spf/graph.go:192-194 reads only edges and drops sub-TLVs, and route.go:155 installs only node.Prefixes, so there is no per-link /32 injection code path |
| `RFC5305-3.2-2` | If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| `RFC5305-4.3-1` | If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |
| `RFC5305-2-1` | Unknown sub-TLVs are to be ignored and skipped upon receipt. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L185). **positive:** `unit/verify` [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L85). **negative:** `unit/verify` [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L205) |
| `RFC5305-3.1-1` | This sub-TLV is OPTIONAL. This sub-TLV SHOULD appear once at most in each extended IS reachability TLV. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-2` | This TLV SHOULD not be included more than once in an LSP. (§4.3) | SHOULD NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-3.6-1` | For stability reasons, rapid changes in the values in this sub-TLV SHOULD NOT cause rapid generation of LSPs. (§3.6) | SHOULD NOT | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5305-4.3-3` | If a router advertises the Traffic Engineering router ID TLV in its LSP, and if it advertises prefixes via the Border Gateway Protocol (BGP) with the BGP next hop attribute set to the BGP router ID, the Traffic Engineering router ID SHOULD be the same as the BGP router ID. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5305-3.7-1`](#rfc5305-3.7-1) If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep configured routes" for TE path computation. The conditional subject is "traffic engineering SPF calculations"; internal/plugins/rsvpte/register.go parseERO, routing.go resolveExplicitPath and frr.go selectBypass consume configured routes, native next-hop lookups and configured bypasses. internal/plugins/isis/spf/spf.go computes normal SPF. There is no CSPF/TE shortest-path consumer to apply this fallback. This exclusion does not cover native TE advertisement or wire duties, and BGP-LS export is not TE-SPF |
| [`RFC5305-3.2-1`](#rfc5305-3.2-1) Implementations MUST NOT inject a /32 prefix for the interface address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never decodes the RFC 5305 TE sub-TLVs (sub-TLV 6/8) or TLV 134 into an address; internal/plugins/isis/spf/graph.go:192-194 reads only edges and drops sub-TLVs, and route.go:155 installs only node.Prefixes, so there is no per-link /32 injection code path |
| [`RFC5305-3.2-2`](#rfc5305-3.2-2) If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; ze originates no TLV 22 TE sub-TLVs (internal/plugins/isis/lsdb/encode.go:101-107 writes a zero sub-TLV length), so it does not implement the TE metric it would govern |
| [`RFC5305-4.3-1`](#rfc5305-4.3-1) If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation is conditional on implementing IS-IS TE; TLV 134 (TE Router ID) is absent from ze's IS-IS codec type set (internal/plugins/isis/packet/tlv.go:17-32), so ze originates and consumes no TLV 134 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5305-3-1`](#rfc5305-3-1)

If a link is advertised with the maximum link metric (2^24 - 1), this link MUST NOT be considered during the normal SPF computation. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: a link advertised at 2^24-1 being walked by normal SPF. TestISISSPFMaxLinkMetricExcluded builds A-B at 16777215 and t.Errorf fires if B or C appears in the SPF result; the positive half proves 16777214 is still relaxed (B at 16777214, C at 16777224).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L318) | unit/verify | mutant, verified |
| positive | [`TestISISSPFMaxLinkMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L317) | unit/verify | unproven |

### [`RFC5305-3-2`](#rfc5305-3-2)

To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC. It is easiest to select MAX_PATH_METRIC such that MAX_PATH_METRIC plus a single link metric does not overflow the number of bits for internal metric calculation. We assume that this is 32 bits. Therefore, we have chosen MAX_PATH_METRIC to be 4,261,412,864 (0xFE000000, 2^32 - 2^25). (§3)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The quoted sentence governs a traffic engineering SPF implementation: "To preclude overflow within a traffic engineering Shortest Path First (SPF) implementation, all metrics greater than or equal to MAX_PATH_METRIC SHALL be considered to have a metric of MAX_PATH_METRIC." Ze has no TE SPF (RFC5305-3.7-1 is not-applicable for that reason). TestISISMetricWidth asserts clampMetric saturation in the normal SPF, a neighbouring rule (RFC5308-5-2 tags the same unit), so the tag proves a different obligation from the one the row states.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L253) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L252) | unit/verify | unproven |

### [`RFC5305-4-1`](#rfc5305-4-1)

If a prefix is advertised with a metric larger then MAX_PATH_METRIC (0xFE000000, see paragraph 3.0), this prefix MUST NOT be considered during the normal SPF computation. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. §4 excludes a prefix "advertised with a metric larger then MAX_PATH_METRIC". Forbidden behaviour: such a prefix being considered in normal SPF. The tagged negative in TestISISMetricWidth advertises exactly MAX_PATH_METRIC (0xFE000000), which §4 does not exclude by itself, and the route drops out because the path total with edge 10 exceeds the ceiling. No input advertises a metric larger than MAX_PATH_METRIC, so the unit proves the path-total ceiling rather than §4's advertised-metric rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L300) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L295) | unit/verify | unproven |

### [`RFC5305-4.1-1`](#rfc5305-4.1-1)

If a prefix is advertised from a higher level to a lower level (e.g., level 2 to level 1), the bit MUST be set to 1, indicating that the prefix has traveled down the hierarchy. Prefixes that have the up/down bit set to 1 may only be advertised down the hierarchy, i.e., to lower levels. These semantics apply even if IS-IS is extended in the future to have additional levels. By ensuring that prefixes follow only the IS-IS hierarchy, we have ensured that the information does not loop, thereby ensuring that there are no persistent forwarding loops. If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote carries two clauses. Down the hierarchy: TestISISEngineLeakOrigination (lsdb_wiring_test.go) asserts the L2-to-L1 leak has up/down SET (t.Errorf on !up), and the negatives (the same test, and TestISISRedistConsumerConnected) assert an up leak and a first injection leave it clear. Same level, area to area: "If a prefix is advertised from one area to another at the same level, then the up/down bit SHALL be set to 1." No tagged unit advertises a prefix between areas at one level, so that clause has no assertion that goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L270) | unit/verify | unproven |
| negative | [`TestISISRedistConsumerConnected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/consumer_test.go#L169) | unit/verify | unproven |
| positive | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L259) | unit/verify | unproven |

### [`RFC5305-3.7-1`](#rfc5305-3.7-1)

If a link is advertised without this sub-TLV, traffic engineering SPF calculations MUST use the normal default metric of this link, which is advertised in the fixed part of the extended IS reachability TLV. (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.7-1, so no unit is bound to it.

### [`RFC5305-4.1-2`](#rfc5305-4.1-2)

The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305ConnectedInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/inject_rfc5305_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC5305FirstInjectionClearsUpDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/inject_rfc5305_test.go#L15) | unit/verify | revert, verified |

### [`RFC5305-4.2-1`](#rfc5305-4.2-1)

If there are no sub-TLVs associated with a prefix, the bit indicating the presence of sub-TLVs SHALL be set to 0 (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5305SubTLVsSetPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc5305_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC5305NoSubTLVsClearsPresenceBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc5305_test.go#L24) | unit/verify | revert, verified |

### [`RFC5305-3.2-1`](#rfc5305-3.2-1)

Implementations MUST NOT inject a /32 prefix for the interface address into their routing or forwarding table because this can lead to forwarding loops when interacting with systems that do not support this sub-TLV. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.2-1, so no unit is bound to it.

### [`RFC5305-3.2-2`](#rfc5305-3.2-2)

If a router implements traffic engineering, it MUST include this sub- TLV. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-3.2-2, so no unit is bound to it.

### [`RFC5305-4.3-1`](#rfc5305-4.3-1)

If a router implements traffic engineering, it MUST include this TLV in its LSP. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5305-4.3-1, so no unit is bound to it.

### [`RFC5305-2-1`](#rfc5305-2-1)

Unknown sub-TLVs are to be ignored and skipped upon receipt. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: rejecting a TLV that carries unknown sub-TLVs, or not skipping them so the next entry misparses. TestISISTLV22RoundTrip (tlv_core_test.go) keeps entry0's three uninterpreted sub-TLVs opaque and then asserts entry1's metric equals types.MaxMetric, which goes red on a wrong skip; TestISISTLVIPv4RoundTrip (tlv_ipv4_test.go, with-subtlv) goes red if a type-1 sub-TLV is rejected (t.Fatalf on decode error). Negative TestISISTLV22Truncated proves an overrunning length is not skipped as absent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISTLV22Truncated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L205) | unit/verify | unproven |
| positive | [`TestISISTLV22RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_core_test.go#L185) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L85) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5305.txt |
| Source fingerprint | 1bb1a875678e75b2 |
| Record | rfc/extraction/rfc5305.json |
| Mapped sentences | 15 |
| Declined as scope | 0 |
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

The walk over RFC 5305 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5305, so its obligations are stated where they were written.
