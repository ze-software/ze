# RFC 2966 - Domain-wide Prefix Distribution with Two-Level IS-IS

Experimental. Every requirement this repository extracted from RFC 2966, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 5 of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 5 | of 5 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 5 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 7 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 5 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2966.md` |
| Requirement shard | `rfc/requirements/rfc2966.md` |
| RFC text | `rfc/full/rfc2966.txt` |

## Enrolment

Enrolled: Domain-wide Prefix Distribution with Two-Level IS-IS (the RFC 2966 up/down bit): five MUST-level requirements after the 2026-09-21 extraction walk, which added RFC2966-3.2-1 (the six-level route preference order of Section 3.2) with no test and no annotation. The other four are over the pure inter-level leak producer LeakPrefixes/leakInto (internal/plugins/isis/spf/leak.go). RFC2966-2-1 (up/down bit set for L2->L1 prefixes, clear otherwise) both polarities via TestISISLeakOriginationL1L2: an L2-derived prefix leaks DOWN into L1 with up/down=true, an L1-native prefix leaks UP into L2 with up/down=false. RFC2966-2-2 (MUST NOT re-advertise up/down-set L1-learned prefixes back into L2) both polarities via TestISISLeakOriginationL1L2: a prefix already carrying the down bit is skipped (leakInto skips p.UpDown) while a clear-bit L1 prefix is still leaked. RFC2966-2-3 (L1L2 routers never advertise L2->L1 routes back into L2) both polarities via TestISISLeakFixpoint: a re-originated down-bit prefix is not leaked back up, while the same prefix without the down bit does leak down. RFC2966-x-1 (ignore internal-reachability-with-external-metric-type on receipt) is {not-applicable}: Ze does not decode the old narrow TLV 128/130 (only the wide TLV 135/236), so the internal/external-metric-type conflict never arises. No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Up/down bit retention and redistribution behavior.

**What the ledger says remains:**

Same IS-IS experimental status. [`RFC2966-3.2-1`](#rfc2966-3.2-1), the Section 3.2 route preference order, was added by the 2026-09-21 extraction walk and carries no test.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC2966-2-1`](#rfc2966-2-1), [`RFC2966-2-2`](#rfc2966-2-2), [`RFC2966-2-3`](#rfc2966-2-3), [`RFC2966-3.2-1`](#rfc2966-3.2-1), [`RFC2966-x-1`](#rfc2966-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2966-2-1` | L1L2 routers must set this bit to one for prefixes that are derived from L2 routing and are advertised into L1 LSPs. (Section 2) | MUST | 2 | **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56) |
| `RFC2966-2-2` | Prefixes with the up/down bit set that are learned via L1 routing, must never be advertised by L1L2 routers back into L2. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59). **negative:** `unit/verify` [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) |
| `RFC2966-2-3` | However, to prevent routing-loops, L1L2 routers must never advertise L2->L1 inter-area routes that they learn via L1 routing, back into L2. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L129). **negative:** `unit/verify` [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L133) |
| `RFC2966-3.2-1` | Some types of routes must always preferred over others, regardless of the costs that were computed in the Dijkstra calculation. One of the reasons for this is that inter-area routes can only be advertised with a maximum metric of 63. Another reason is that this maximum value of 63 does not mean infinity (e.g. like a hop count of 16 in RIP denotes unreachable). Introducing a value for infinity cost in IS-IS inter-area routes would introduce counting- to-infinity behavior via two or more L1L2 routers, which would have a bad impact on network stability. The order of preference of IP routes in IS-IS is based on a few assumptions. - RFC 1195 defines that routes derived from L1 routing are preferred over routes derived from L2 routing. - The note in RFC 1195 paragraph 3.10.2, item 2c) defines that internal routes with internal metric-type and external prefixes with internal metric-type have the same preference. - RFC 1195 defines that external routes with internal metric-type are preferred over external routes with external metric type. - Routes derived from L2 routing are preferred over L2->L1 routes derived from L1 routing. Based on these assumptions, this document defines the following route preferences. 1) L1 intra-area routes with internal metric L1 external routes with internal metric 2) L2 intra-area routes with internal metric L2 external routes with internal metric L1->L2 inter-area routes with internal metric L1->L2 inter-area external routes with internal metric 3) L2->L1 inter-area routes with internal metric L2->L1 inter-area external routes with internal metric 4) L1 external routes with external metric 5) L2 external routes with external metric L1->L2 inter-area external routes with external metric 6) L2->L1 inter-area external routes with external metric (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC2966SixPreferenceClasses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L187). **negative:** `unit/verify` [`TestRFC2966ExternalL1DoesNotOverrideDownInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L210) |
| `RFC2966-x-1` | Upon receipt of an IP prefix with this combination, routers must ignore this prefix. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L220). **negative:** `unit/verify` [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L221) |
| `RFC2966-3.3-1` | Therefore, it is recommended that implementations ignore the up/down bit in L2 LSPs, and accept the prefixes in L2 LSPs regardless whether the up/down bit is set. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2966-x-2` | For this reason it is recommended that implementations by default do not advertise any L2 routes into L1. Implementations should force the network administrator to manually configure L1L2 routers to advertise any L2 routes into L1. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2966 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2966-2-1`](#rfc2966-2-1)

L1L2 routers must set this bit to one for prefixes that are derived from L2 routing and are advertised into L1 LSPs. (Section 2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: an L1L2 router originating an L2-derived prefix in its L1 LSP with bit 8 of the default metric clear. TestISISLeakOriginationL1L2 asserts hasLeak(leak.IntoL1, l2Derived, true), i.e. the LeakedPrefix.UpDown flag from spf.LeakPrefixes, not the bit in the originated L1 LSP; an encoder that dropped UpDown when writing TLV 128/130 would leave it green. The negative (L1 prefix leaked up with the flag clear) is likewise struct-level. No tagged assertion decodes the L1 LSP.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L56) | unit/verify | unproven |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L52) | unit/verify | unproven |

### [`RFC2966-2-2`](#rfc2966-2-2)

Prefixes with the up/down bit set that are learned via L1 routing, must never be advertised by L1L2 routers back into L2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: re-advertising into L2 a prefix learned via L1 with the up/down bit set. TestISISLeakOriginationL1L2 assertion 3 (hasPrefix(leak.IntoL2, alreadyDown) -> t.Errorf) goes red if leakInto stops skipping p.UpDown; the negative keeps a clear-bit L1 prefix leaking up (assertion 1), so a blanket drop also goes red. Both polarities in the one unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L62) | unit/verify | unproven |
| positive | [`TestISISLeakOriginationL1L2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L59) | unit/verify | unproven |

### [`RFC2966-2-3`](#rfc2966-2-3)

However, to prevent routing-loops, L1L2 routers must never advertise L2->L1 inter-area routes that they learn via L1 routing, back into L2. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: an L1L2 router advertising back into L2 an L2->L1 inter-area route it learned via L1. TestISISLeakFixpoint round 2 re-injects the leaked-down prefix as an L1 advertisement with UpDown set and fails on hasPrefix(r2.IntoL2, l2Derived); round 1 (negative) requires the same L2 prefix to leak down, so suppressing all leaking goes red too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L133) | unit/verify | unproven |
| positive | [`TestISISLeakFixpoint`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/leak_test.go#L129) | unit/verify | unproven |

### [`RFC2966-3.2-1`](#rfc2966-3.2-1)

Some types of routes must always preferred over others, regardless of the costs that were computed in the Dijkstra calculation. One of the reasons for this is that inter-area routes can only be advertised with a maximum metric of 63. Another reason is that this maximum value of 63 does not mean infinity (e.g. like a hop count of 16 in RIP denotes unreachable). Introducing a value for infinity cost in IS-IS inter-area routes would introduce counting- to-infinity behavior via two or more L1L2 routers, which would have a bad impact on network stability. The order of preference of IP routes in IS-IS is based on a few assumptions. - RFC 1195 defines that routes derived from L1 routing are preferred over routes derived from L2 routing. - The note in RFC 1195 paragraph 3.10.2, item 2c) defines that internal routes with internal metric-type and external prefixes with internal metric-type have the same preference. - RFC 1195 defines that external routes with internal metric-type are preferred over external routes with external metric type. - Routes derived from L2 routing are preferred over L2->L1 routes derived from L1 routing. Based on these assumptions, this document defines the following route preferences. 1) L1 intra-area routes with internal metric L1 external routes with internal metric 2) L2 intra-area routes with internal metric L2 external routes with internal metric L1->L2 inter-area routes with internal metric L1->L2 inter-area external routes with internal metric 3) L2->L1 inter-area routes with internal metric L2->L1 inter-area external routes with internal metric 4) L1 external routes with external metric 5) L2 external routes with external metric L1->L2 inter-area external routes with external metric 6) L2->L1 inter-area external routes with external metric (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden behaviour: selecting a less preferred route class because its metric is lower, for any pair across the six classes. TestRFC2966SixPreferenceClasses checks each adjacent class pair through the SPF engine with one representative per class (L1 internal, L2 internal, L1 down internal, L1 external-metric, L2 external-metric, L1 down external-metric) and TestRFC2966ExternalL1DoesNotOverrideDownInternal the 3-vs-4 pair. The class members the list places at equal preference (L1 external with internal metric in class 1, L2 external and L1->L2 inter-area routes in class 2, L2->L1 inter-area external with internal metric in class 3, L1->L2 inter-area external with external metric in class 5) have no tagged assertion, so a mis-ranked member stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2966ExternalL1DoesNotOverrideDownInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC2966SixPreferenceClasses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L187) | unit/verify | unproven |

### [`RFC2966-x-1`](#rfc2966-x-1)

Upon receipt of an IP prefix with this combination, routers must ignore this prefix. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: accepting an IP Internal Reachability (TLV 128) prefix whose metric-type bit says external. TestRFC2966InvalidInternalExternalMetricExcluded sets bit 0x40 on the TLV 128 entry of the cheaper route and requires metricWinner to pick the dearer valid route (id 2, 160); clearing the bit requires the cheap route back (id 3, 2), so both an accept-anyway and a drop-everything implementation go red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L221) | unit/verify | unproven |
| positive | [`TestRFC2966InvalidInternalExternalMetricExcluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L220) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2966.txt |
| Source fingerprint | 69c5c5a8b8142a19 |
| Record | rfc/extraction/rfc2966.json |
| Mapped sentences | 5 |
| Declined as scope | 15 |
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
| `2:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The zero half of the same sender rule the previous sentence states: RFC2966-2-1 reads 'Set the up/down bit to one for L2-derived prefixes advertised into L1 LSPs, zero otherwise'. | The bit must be set to zero for all other IP prefixes in L1 or L2 LSPs. |
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
