# RFC 2966 - Domain-wide Prefix Distribution with Two-Level IS-IS

Experimental. Every requirement this repository extracted from RFC 2966, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 6 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 70.0% | 14 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 6 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 8 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 6 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc2966.md` |
| Requirement shard | `rfc/requirements/rfc2966.md` |
| RFC text | `rfc/full/rfc2966.txt` |

## Enrolment

Enrolled: Domain-wide Prefix Distribution with Two-Level IS-IS (the RFC 2966 up/down bit): five MUST-level requirements after the 2026-09-21 extraction walk, which added RFC2966-3.2-1 (the six-level route preference order of Section 3.2) with no test and no annotation. The other four are over the pure inter-level leak producer LeakPrefixes/leakInto (internal/plugins/isis/spf/leak.go). RFC2966-2-1 (up/down bit set for L2->L1 prefixes, clear otherwise) both polarities via TestISISLeakOriginationL1L2: an L2-derived prefix leaks DOWN into L1 with up/down=true, an L1-native prefix leaks UP into L2 with up/down=false. RFC2966-2-2 (MUST NOT re-advertise up/down-set L1-learned prefixes back into L2) both polarities via TestISISLeakOriginationL1L2: a prefix already carrying the down bit is skipped (leakInto skips p.UpDown) while a clear-bit L1 prefix is still leaked. RFC2966-2-3 (L1L2 routers never advertise L2->L1 routes back into L2) both polarities via TestISISLeakFixpoint: a re-originated down-bit prefix is not leaked back up, while the same prefix without the down bit does leak down. RFC2966-x-1 (ignore internal-reachability-with-external-metric-type on receipt) is {not-applicable}: Ze does not decode the old narrow TLV 128/130 (only the wide TLV 135/236), so the internal/external-metric-type conflict never arises. No SHOULD/MAY requirements are gated. On 2026-09-30 the zero half of the §2 sender rule ("The bit must be set to zero for all other IP prefixes in L1 or L2 LSPs.") was given its own row, RFC2966-2-4, since the RFC2966-2-1 sentence states only the set-to-one half; it is proven on the decoded L2 LSPs of TestISISEngineLeakOrigination and TestRFC2966NarrowLeakUpDownBit, on the decoded L1 and L2 LSPs of TestRFC2966NativePrefixesGoOutWithBitZero (a connected and a redistributed prefix go out with the bit zero beside a down-leaked prefix that carries it) and, negatively, by TestISISLeakOriginationL1L2 (an L1 prefix already carrying the bit never reaches L2).

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Up/down bit retention and redistribution behavior.

**What the ledger says remains:**

Same IS-IS experimental status. [`RFC2966-3.2-1`](#rfc2966-3.2-1), the Section 3.2 route preference order, was added by the 2026-09-21 extraction walk; its tests in internal/plugins/isis cover every class member the order lists.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC2966-2-1`](#rfc2966-2-1), [`RFC2966-2-2`](#rfc2966-2-2), [`RFC2966-2-3`](#rfc2966-2-3), [`RFC2966-2-4`](#rfc2966-2-4), [`RFC2966-3.2-1`](#rfc2966-3.2-1), [`RFC2966-x-1`](#rfc2966-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2966-2-1` | L1L2 routers must set this bit to one for prefixes that are derived from L2 routing and are advertised into L1 LSPs. (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L259). **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52). **positive:** `unit/verify` [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L53). **negative:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L271). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56). **negative:** `unit/verify` [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L57) |
| `RFC2966-2-2` | Prefixes with the up/down bit set that are learned via L1 routing, must never be advertised by L1L2 routers back into L2. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) |
| `RFC2966-2-3` | However, to prevent routing-loops, L1L2 routers must never advertise L2->L1 inter-area routes that they learn via L1 routing, back into L2. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L139). **negative:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L143) |
| `RFC2966-2-4` | The bit must be set to zero for all other IP prefixes in L1 or L2 LSPs. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L272). **positive:** `unit/verify` [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L60). **positive:** `unit/verify` [`TestRFC2966NativePrefixesGoOutWithBitZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L129). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L65) |
| `RFC2966-3.2-1` | Some types of routes must always preferred over others, regardless of the costs that were computed in the Dijkstra calculation. One of the reasons for this is that inter-area routes can only be advertised with a maximum metric of 63. Another reason is that this maximum value of 63 does not mean infinity (e.g. like a hop count of 16 in RIP denotes unreachable). Introducing a value for infinity cost in IS-IS inter-area routes would introduce counting- to-infinity behavior via two or more L1L2 routers, which would have a bad impact on network stability. The order of preference of IP routes in IS-IS is based on a few assumptions. - RFC 1195 defines that routes derived from L1 routing are preferred over routes derived from L2 routing. - The note in RFC 1195 paragraph 3.10.2, item 2c) defines that internal routes with internal metric-type and external prefixes with internal metric-type have the same preference. - RFC 1195 defines that external routes with internal metric-type are preferred over external routes with external metric type. - Routes derived from L2 routing are preferred over L2->L1 routes derived from L1 routing. Based on these assumptions, this document defines the following route preferences. 1) L1 intra-area routes with internal metric L1 external routes with internal metric 2) L2 intra-area routes with internal metric L2 external routes with internal metric L1->L2 inter-area routes with internal metric L1->L2 inter-area external routes with internal metric 3) L2->L1 inter-area routes with internal metric L2->L1 inter-area external routes with internal metric 4) L1 external routes with external metric 5) L2 external routes with external metric L1->L2 inter-area external routes with external metric 6) L2->L1 inter-area external routes with external metric (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC2966EveryClassMemberOutranksNextClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L224). **positive:** `unit/verify` [`TestRFC2966SixPreferenceClasses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L186). **negative:** `unit/verify` [`TestRFC2966EqualClassMembersCompareByMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L241). **negative:** `unit/verify` [`TestRFC2966ExternalL1DoesNotOverrideDownInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L209) |
| `RFC2966-x-1` | Upon receipt of an IP prefix with this combination, routers must ignore this prefix. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L219). **negative:** `unit/verify` [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L220) |
| `RFC2966-3.3-1` | Therefore, it is recommended that implementations ignore the up/down bit in L2 LSPs, and accept the prefixes in L2 LSPs regardless whether the up/down bit is set. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2966-x-2` | For this reason it is recommended that implementations by default do not advertise any L2 routes into L1. Implementations should force the network administrator to manually configure L1L2 routers to advertise any L2 routes into L1. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2966 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2966-2-1`](#rfc2966-2-1)

L1L2 routers must set this bit to one for prefixes that are derived from L2 routing and are advertised into L1 LSPs. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestISISEngineLeakOrigination, TestRFC2966NarrowLeakUpDownBit and spf TestISISLeakOriginationL1L2 gained only RFC2966-2-4 tag comments; bodies byte-identical. Positive: TestRFC2966NarrowLeakUpDownBit requires 0x80 on the default-metric octet of the L2->L1 internal (TLV 128) and external (TLV 130) narrow leaks in the originated L1 LSP, TestISISEngineLeakOrigination the TLV 135 bit, spf leakInto sets UpDown only for IntoL1. Negative: the L1-derived prefix leaked into L2 goes out with the bit clear on both encodings.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L271) | unit/verify | revert, verified |
| negative | [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L259) | unit/verify | revert, verified |
| positive | [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52) | unit/verify | revert, verified |

### [`RFC2966-2-2`](#rfc2966-2-2)

Prefixes with the up/down bit set that are learned via L1 routing, must never be advertised by L1L2 routers back into L2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestISISLeakOriginationL1L2 gained only RFC2966-2-4 tag comments; body byte-identical. Assertion 3 (hasPrefix(leak.IntoL2, alreadyDown) -> t.Errorf) goes red if leakInto stops skipping p.UpDown; assertion 1 keeps a clear-bit L1 prefix leaking up, so a blanket drop also goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) | unit/verify | unproven |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59) | unit/verify | unproven |

### [`RFC2966-2-3`](#rfc2966-2-3)

However, to prevent routing-loops, L1L2 routers must never advertise L2->L1 inter-area routes that they learn via L1 routing, back into L2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: an L1L2 router advertising back into L2 an L2->L1 inter-area route it learned via L1. TestISISLeakFixpoint round 2 re-injects the leaked-down prefix as an L1 advertisement with UpDown set and fails on hasPrefix(r2.IntoL2, l2Derived); round 1 (negative) requires the same L2 prefix to leak down, so suppressing all leaking goes red too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L143) | unit/verify | unproven |
| positive | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L139) | unit/verify | unproven |

### [`RFC2966-2-4`](#rfc2966-2-4)

The bit must be set to zero for all other IP prefixes in L1 or L2 LSPs. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the L1 half was added. Positive: TestRFC2966NativePrefixesGoOutWithBitZero runs a started engine, stores a connected prefix (ConnectedPrefixInfos) and a redistributed one (Consumer.InjectRoute) at both levels, down-leaks 10.2/24 into L1, then decodes TLV 135 from the stored L1 AND L2 LSPs: the down-leaked prefix carries the bit, the connected and redistributed prefixes carry it zero in both LSPs, so an originator setting the bit on every L1 entry or on own prefixes goes red (per-prefix, read from the wire). The L2 half stays on TestISISEngineLeakOrigination (TLV 135) and TestRFC2966NarrowLeakUpDownBit (TLV 128 bit 8). Negative: spf TestISISLeakOriginationL1L2 keeps the down-bit alreadyDown out of IntoL2 while the clear-bit L1 prefix leaks up with the bit false. Each cover holds an observed-red revert record (ConnectedPrefixInfos, narrowIPReachEntryBytes, leakedToPrefixInfos, leakInto).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L65) | unit/verify | revert, verified |
| positive | [`TestISISEngineLeakOrigination`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L272) | unit/verify | revert, verified |
| positive | [`TestRFC2966NarrowLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC2966NativePrefixesGoOutWithBitZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc2966_narrow_updown_test.go#L129) | unit/verify | revert, verified |

### [`RFC2966-3.2-1`](#rfc2966-3.2-1)

Some types of routes must always preferred over others, regardless of the costs that were computed in the Dijkstra calculation. One of the reasons for this is that inter-area routes can only be advertised with a maximum metric of 63. Another reason is that this maximum value of 63 does not mean infinity (e.g. like a hop count of 16 in RIP denotes unreachable). Introducing a value for infinity cost in IS-IS inter-area routes would introduce counting- to-infinity behavior via two or more L1L2 routers, which would have a bad impact on network stability. The order of preference of IP routes in IS-IS is based on a few assumptions. - RFC 1195 defines that routes derived from L1 routing are preferred over routes derived from L2 routing. - The note in RFC 1195 paragraph 3.10.2, item 2c) defines that internal routes with internal metric-type and external prefixes with internal metric-type have the same preference. - RFC 1195 defines that external routes with internal metric-type are preferred over external routes with external metric type. - Routes derived from L2 routing are preferred over L2->L1 routes derived from L1 routing. Based on these assumptions, this document defines the following route preferences. 1) L1 intra-area routes with internal metric L1 external routes with internal metric 2) L2 intra-area routes with internal metric L2 external routes with internal metric L1->L2 inter-area routes with internal metric L1->L2 inter-area external routes with internal metric 3) L2->L1 inter-area routes with internal metric L2->L1 inter-area external routes with internal metric 4) L1 external routes with external metric 5) L2 external routes with external metric L1->L2 inter-area external routes with external metric 6) L2->L1 inter-area external routes with external metric (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a less preferred class selected for a lower metric, or two members of one class ranked against each other. TestRFC2966EveryClassMemberOutranksNextClass drives every member of each class the fixture can express (L1 internal and external-with-internal-metric; L2 internal and external-with-internal-metric, which is also the L1->L2 inter-area member to the receiver; L2->L1 internal and external-with-internal-metric; L1 external-metric; L2 external-metric; L2->L1 external-metric) against every member of the next class, preferred one at cost 100/metric 60 against 1/1. TestRFC2966EqualClassMembersCompareByMetric requires the lower metric to win between two members of one class in both orders, so a member ranked apart from its class goes red. Recorded red on preferenceRank and better; the older adjacent-pair tests remain.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2966EqualClassMembersCompareByMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L241) | unit/verify | revert, verified |
| negative | [`TestRFC2966ExternalL1DoesNotOverrideDownInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L209) | unit/verify | revert, verified |
| positive | [`TestRFC2966EveryClassMemberOutranksNextClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L224) | unit/verify | revert, verified |
| positive | [`TestRFC2966SixPreferenceClasses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L186) | unit/verify | revert, verified |

### [`RFC2966-x-1`](#rfc2966-x-1)

Upon receipt of an IP prefix with this combination, routers must ignore this prefix. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: accepting an IP Internal Reachability (TLV 128) prefix whose metric-type bit says external. TestRFC2966InvalidInternalExternalMetricExcluded sets bit 0x40 on the TLV 128 entry of the cheaper route and requires metricWinner to pick the dearer valid route (id 2, 160); clearing the bit requires the cheap route back (id 3, 2), so both an accept-anyway and a drop-everything implementation go red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L219) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2966.txt |
| Source fingerprint | 69c5c5a8b8142a19 |
| Record | rfc/extraction/rfc2966.json |
| Mapped sentences | 6 |
| Declined as scope | 14 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 1 | walked | not stated |
| `2` | not stated | 7 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 3 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | 'It must also be noted' is the author addressing the reader, not an obligation on an implementation. The sentence records that domain-wide prefix distribution leaves the link state database boundary untouched. | It must also be noted that the domain-wide distribution of prefixes has no effect whatsoever on the first aspect of scalability, namely the existence of areas and the limitation of the distribution of the link state database. |
| `2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 1195, which the sentence names: 'RFC 1195 defines that these L1->L2 inter-area routes must be advertised in L2 LSPs in the IP Internal Reachability Information TLV (TLV 128).' RFC 2966 reports it as the starting point for the up/down bit. | RFC 1195 defines that these L1->L2 inter-area routes must be advertised in L2 LSPs in the "IP Internal Reachability Information" TLV (TLV 128). |
| `2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | States the design need that motivates the up/down bit, and the next sentences supply the mechanism: 'Draft-ietf-isis-traffic-01.txt defines the up/down bit for this purpose.' The obligations are sites 2:5, 2:6 and 2:7. | Therefore, there must be a way to distinguish L2->L1 inter-area routes from L1 intra-area routes. |
| `2:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1195's rule for the bit it reserved, introduced two sentences earlier as 'RFC 1195 defines TLVs 128 and 130 to contain IP routes... the default metric, has the high-order bit reserved (bit 8).' RFC 2966 then supersedes it: 'This document redefines this high-order bit in the default metric field in TLVs 128 and 130 to be the up/down bit.' | Routers must set this bit to zero on transmission, and ignore it on receipt. |
| `2.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The preference rule belongs to RFC 1195, which the same paragraph cites and this document declines to change: 'RFC 1195 states this quite clearly in the note in paragraph 3.10.2, item 2c). This document does not alter this rule of preference.' | To prevent confusion, this document states again that when a router computes IP routes, it must give the same preference to IP routes advertised in an "IP Internal Reachability Information" TLV and IP routes advertised in an "IP External Reachability Information" TLV. |
| `2.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Also an RFC 1195 rule, which section 3.2 of this document attributes: 'RFC 1195 defines that external routes with internal metric-type are preferred over external routes with external metric type.' Section 2.1 is titled 'Clarification of external route-type and external metric-type' and restates it. | However, IP routes advertised in "IP External Reachability Information" with external metric-type must be given less preference than the same IP routes advertised with internal-metric type, regardless of the value of the metrics. |
| `2.1:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The must-not clause restates the same RFC 1195 rule the paragraph already attributed to 'the note in paragraph 3.10.2, item 2c)'. What this sentence adds is a permission, not an obligation: routers running several IGPs 'are free to use this distinction' when filling their global RIB. | While IS-IS routers must not give different preference to IP prefixes learned via "IP Internal Reachability Information" and "IP External Reachability Information" when executing the Dijkstra calculation, routers that implement multiple IGPs are free to use this distinction between internal and external routes when comparing routes derived from different IGPs for inclusion in their global RIB. |
| `2.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 1195, which the sentence names: 'RFC 1195 defines that IP routes learned via L1 routing must always be advertised in L2 LSPs in a IP Internal Reachability Information TLV.' | RFC 1195 defines that IP routes learned via L1 routing must always be advertised in L2 LSPs in a "IP Internal Reachability Information" TLV. |
| `2.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the up/down bit sender rule for the external-prefix case of section 2.2. RFC2966-2-1 already requires the bit set to one for prefixes derived from L2 routing and advertised into L1 LSPs. | Of course in this case also the up/down bit must be set. |
| `2.2:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 1195, which the sentence names: 'RFC 1195 defines that if a router sees the same external prefix advertised by two or more routers with the same external metric, it must select the route that is advertised by the router that is closest to itself.' | RFC 1195 defines that if a router sees the same external prefix advertised by two or more routers with the same external metric, it must select the route that is advertised by the router that is closest to itself. |
| `4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens 'As has been stated before', and restates the section 2 loop-prevention rule that RFC2966-2-3 already carries: an L1L2 router never advertises an L2->L1 inter-area route learned via L1 routing back into L2. | As has been stated before, L1L2 routers must never advertise L2->L1 inter-area routes back into L2. |
| `4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A precondition on the deployed network rather than on an implementation: every L1L2 router in the area has to understand the up/down bit before L2 routes are leaked down. The same paragraph turns it into the implementation obligation the summary carries as RFC2966-x-2: 'it is recommended that implementations by default do not advertise any L2 routes into L1. Implementations should force the network administrator to manually configure L1L2 routers to advertise any L2 routes into L1.' | Therefore, if L2 routes are advertised down into L1 area, it is required that all L1L2 routers in that area run software that understands the new up/down bit. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 5 compares this document with the proposal it calls [3], and the sentence describes what deploying [3] costs a network administrator. RFC 2966 places no obligation here; the section closes by offering its own solution as the alternative that 'requires only an upgrade of L1L2 routers in selected areas'. | To make full use of the wider metric space, network administrators must deploy both new TLVs at the same time. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The ISOC full copyright statement in the back matter. It governs republication of the document text and carries no protocol obligation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2966, so its obligations are stated where they were written.
