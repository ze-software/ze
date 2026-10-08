# RFC 5308 - Routing IPv6 with IS-IS

Experimental. Every requirement this repository extracted from RFC 5308, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 8 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 86.7% | 26 of 30 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 9 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tagged units | 30 |
| Recorded audit verdicts | 8 |
| Discrimination records | 26 |
| Summary | `rfc/short/rfc5308.md` |
| Requirement shard | `rfc/requirements/rfc5308.md` |
| RFC text | `rfc/full/rfc5308.txt` |

## Enrolment

Enrolled: Routing IPv6 with IS-IS: eight MUST-level requirements after the 2026-09-21 extraction walk. Seven are met and test-bound with positive+negative tags; the eighth, RFC5308-2-3 (the external bit of TLV 236 is set to 1 when the prefix came into IS-IS from another routing protocol, Section 2), the walk added and no test covers it. 2-1 (no link-local in IPv6 Reachability TLV 236) and 3-2 (no link-local in the LSP IPv6 address set) via internal/plugins/isis/lsdb origination tests; 3-1 (IPv6 Interface Address TLV 232 only for link-local in Hellos) and 4-1 (advertise IPv6 NLPID 0x8E in Protocols Supported) via internal/plugins/isis/circuit tests; 2-2 (do not route a metric above MaxV6PathMetric), 5-1 (up/down and level-aware preference), and 5-2 (clamp the path metric) via internal/plugins/isis/spf tests. Single-topology IS-IS: IPv6 rides the shared per-level SPF, so 5-1/5-2 reuse the IPv4 producers exercised for IPv6 via BuildRoutesV6.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

IPv6 reachability over the same IS-IS instance.

**What the ledger says remains:**

Same IS-IS experimental status. [`RFC5308-2-3`](#rfc5308-2-3), the external bit in TLV 236 for a redistributed prefix, was added by the 2026-09-21 extraction walk and carries no test.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC5308-2-1`](#rfc5308-2-1), [`RFC5308-2-2`](#rfc5308-2-2), [`RFC5308-2-3`](#rfc5308-2-3), [`RFC5308-3-1`](#rfc5308-3-1), [`RFC5308-3-2`](#rfc5308-3-2), [`RFC5308-4-1`](#rfc5308-4-1), [`RFC5308-5-1`](#rfc5308-5-1), [`RFC5308-5-2`](#rfc5308-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5308-2-1` | Link-local prefixes MUST NOT be advertised using this TLV. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L65). **negative:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L66) |
| `RFC5308-2-2` | if a prefix is advertised with a metric larger than MAX_V6_PATH_METRIC (0xFE000000), this prefix MUST not be considered during the normal Shortest Path First (SPF) computation. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L207). **positive:** `unit/verify` [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L98). **negative:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L206). **negative:** `unit/verify` [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L100) |
| `RFC5308-2-3` | If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1. (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestRFC5308RedistributedIPv6SetsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L15). **negative:** `unit/verify` [`TestRFC5308ConnectedIPv6ClearsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L37) |
| `RFC5308-3-1` | For Hello PDUs, the "Interface Address" TLV MUST contain only the link-local IPv6 addresses assigned to the interface that is sending the Hello. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L30). **positive:** `unit/verify` [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L60). **positive:** `unit/verify` [`TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L211). **negative:** `unit/verify` [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L103). **negative:** `unit/verify` [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L130). **negative:** `unit/verify` [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L64) |
| `RFC5308-3-2` | For LSPs, the "Interface Address" TLVs MUST contain only the non-link-local IPv6 addresses assigned to the IS. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L110). **positive:** `unit/verify` [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L116). **negative:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L111). **negative:** `unit/verify` [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L119) |
| `RFC5308-4-1` | The value of the IPv6 Network Layer Protocol ID (NLPID) is 142 (0x8E). As with [RFC1195] and IPv4, if the IS supports IPv6 routing using IS-IS, it MUST advertise this in the "NLPID" TLV by adding the IPv6 NLPID. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L31). **positive:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L148). **positive:** `unit/verify` [`TestRFC5308HelloNLPIDIs142`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L228). **negative:** `unit/verify` [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L78). **negative:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L149) |
| `RFC5308-5-1` | The order of preference between paths for a given prefix MUST be modified to consider the up/down bit. The new order of preference is as follows (from best to worst). 1. Level 1 up prefix 2. Level 2 up prefix 3. Level 2 down prefix 4. Level 1 down prefix (§5) | MUST | 5 | **positive:** `unit/verify` [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L91). **positive:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L46). **positive:** `unit/verify` [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L156). **negative:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L47). **negative:** `unit/verify` [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L159) |
| `RFC5308-5-2` | If, during the SPF, a path metric would exceed MAX_V6_PATH_METRIC, it SHALL be considered to be MAX_V6_PATH_METRIC. (Section 5) | SHALL | 5 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) |
| `RFC5308-5-3` | Any remaining multiple paths SHOULD be considered for equal-cost multi-path routing if the router supports this; otherwise, the router can select any one of the multiple paths. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5308 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5308-2-1`](#rfc5308-2-1)

Link-local prefixes MUST NOT be advertised using this TLV. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a fe80::/64 link-local prefix advertised in TLV 236. (b) TestISISOriginateTLV236 decodes the originated LSP and errors on got["fe80::/64"] present (negative); the same unit requires the two routable prefixes present with their metric and X bit (positive). The unit runs NonLinkLocalV6Prefixes, the filter lsdb_wiring.go calls on every TLV 236 origination, then Originator.Originate, so breaking the filter reddens it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L66) | unit/verify | unproven |
| positive | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L65) | unit/verify | unproven |

### [`RFC5308-2-2`](#rfc5308-2-2)

if a prefix is advertised with a metric larger than MAX_V6_PATH_METRIC (0xFE000000), this prefix MUST not be considered during the normal Shortest Path First (SPF) computation. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/isis/spf/rfc5308_ipv6_test.go::TestISISIPv6MetricAboveMaxIgnored, internal/plugins/isis/spf/rfc5308_ipv6_test.go::TestISISIPv6MetricAboveMaxIgnored#2 moved. (a) forbidden: a TLV 236 prefix advertised with a metric larger than MAX_V6_PATH_METRIC taking part in normal SPF. (b) spf TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered: node 3 (distance 10) advertises 2001:db8:9::/64 at 50 (positive, installed at 60 via node 3); node 2 (distance 10) advertises the same prefix at 0xFE000001 and 2001:db8:8::/64 only at 0xFFFFFFFF (negative): the shared prefix keeps one next hop at 60 and the lone prefix is absent. The lone prefix is the discriminating input: any BuildRoutesV6 that considered it would install it (at MAX after clampMetric saturation, or at a wrapped cost without it). The per-entry filter (ipv6.go, p.Metric > MaxV6PathMetric) and the accumulated-cost ceiling (total >= MaxPathMetric) are redundant for every over-max input, so no input isolates the filter alone; removing either one alone leaves Ze compliant, and the test asserts the stack outcome the RFC states. Recorded +/- on BuildRoutesV6.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L207) | unit/verify | revert, verified |

### [`RFC5308-2-3`](#rfc5308-2-3)

If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a prefix distributed into IS-IS from another routing protocol originated with the X bit 0. (b) TestRFC5308RedistributedIPv6SetsExternalBit injects a bgp-sourced IPv6 route through the redistribute Consumer and t.Fatalf's on !got[0].External at Level1 and Level2 (positive); TestRFC5308ConnectedIPv6ClearsExternalBit fails if a connected prefix carries External (negative), so an always-set bit is caught too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5308ConnectedIPv6ClearsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC5308RedistributedIPv6SetsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L15) | unit/verify | revert, verified |

### [`RFC5308-3-1`](#rfc5308-3-1)

For Hello PDUs, the "Interface Address" TLV MUST contain only the link-local IPv6 addresses assigned to the interface that is sending the Hello. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. (a) forbidden: a Hello TLV 232 carrying a non-link-local address, or a link-local address not assigned to the sending interface. (b) The earlier gap (the production address source untested) is closed: root TestRFC5308HelloAddressIsTheInterfaceLinkLocal registers a test iface backend (only the OS address list is fake; the resolver's link-local classification is real) with two interfaces holding distinct link-local addresses, and requires interfaceIPv6LinkLocal to return fe80::a:1 for isis-ll-a although its global address is listed first, and fe80::b:2 for isis-ll-b (positive); a global-only interface and an interface without ipv6-unicast yield none (negative). circuits.go builds each circuit's Config.IPv6LinkLocal from interfaceIPv6LinkLocal(ic) for that same interface, and circuit TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal proves each circuit sends exactly its own configured address; TestISISIIHTLV232RejectsNonLinkLocal and TestISISIIHTLV232LinkLocal cover the encoder's link-local-only guard. Recorded +/- on circuits.go interfaceIPv6LinkLocal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L60) | unit/verify | revert, verified |

### [`RFC5308-3-2`](#rfc5308-3-2)

For LSPs, the "Interface Address" TLVs MUST contain only the non-link-local IPv6 addresses assigned to the IS. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. (a) forbidden: an LSP TLV 232 carrying a link-local address. (b) The earlier gap (the production filter untested) is closed: root TestRFC5308LSPInterfaceAddressesAreNonLinkLocal runs the engine over the test iface backend with two IPv6 circuits and decodes fragment 0 at both levels, built by the production path (lsdb_wiring.go levelState -> interfaceIPv6NonLinkLocal -> Originate): TLV 232 is exactly [2001:db8:a::1] (positive), and neither fe80::a:1 (same circuit) nor fe80::b:2 (the other circuit's only IPv6 address) appears (negative). Recorded +/- on circuits.go interfaceIPv6NonLinkLocal. lsdb TestISISOriginateTLV232Scope remains tagged and pre-filters its input; it carries no weight.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L111) | unit/verify | revert, verified |
| negative | [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L116) | unit/verify | revert, verified |

### [`RFC5308-4-1`](#rfc5308-4-1)

The value of the IPv6 Network Layer Protocol ID (NLPID) is 142 (0x8E). As with [RFC1195] and IPv4, if the IS supports IPv6 routing using IS-IS, it MUST advertise this in the "NLPID" TLV by adding the IPv6 NLPID. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an IPv6-capable IS omitting the IPv6 NLPID from TLV 129, or advertising a value other than 142 (0x8E). (b) circuit TestRFC5308HelloNLPIDIs142 checks the transmitted dual-stack IIH's TLV 129 wire octets equal exactly cc 8e, against the literal rather than packet.NLPIDIPv6, closing the constant gap; TestISISProtocolsSupportedDualStack and TestISISIIHTLV232LinkLocal cover the advertise clause and the IPv4-only cases (TestISISProtocolsSupportedDualStack ipv4-only, TestISISIIHNoTLV232WhenIPv4Only) the absence when IPv6 is not supported. Recorded + on hello.go protocolsSupportedTLV.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L78) | unit/verify | revert, verified |
| negative | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloNLPIDIs142`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L228) | unit/verify | revert, verified |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L148) | unit/verify | revert, verified |

### [`RFC5308-5-1`](#rfc5308-5-1)

The order of preference between paths for a given prefix MUST be modified to consider the up/down bit. The new order of preference is as follows (from best to worst). 1. Level 1 up prefix 2. Level 2 up prefix 3. Level 2 down prefix 4. Level 1 down prefix (§5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/isis/spf/rfc5308_ipv6_test.go::TestISISIPv6LevelArbitration moved. (a) forbidden: any path order other than L1-up > L2-up > L2-down > L1-down. (b) spf TestRFC5308FourStepPathPreference advertises the four classes each from its own node at metrics that invert the class order (400, 300, 200, 100), then drops the best class in turn: the installed route is always the best class left (checked by next hop, level and up/down), so each adjacent pair is decided by class not metric, and a separate case pits L1 up against L2 down alone. This closes the two pairs the previous note named (L2-up over L2-down, L1-up over L2-down). Recorded +/- on route.go preferenceRank, which BuildRoutesV6 uses. TestISISLeakUpDownBit and TestISISIPv6LevelArbitration remain tagged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L159) | unit/verify | revert, verified |
| negative | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L46) | unit/verify | revert, verified |

### [`RFC5308-5-2`](#rfc5308-5-2)

If, during the SPF, a path metric would exceed MAX_V6_PATH_METRIC, it SHALL be considered to be MAX_V6_PATH_METRIC. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a path metric that exceeds MAX_V6_PATH_METRIC kept (or wrapped) instead of taken as MAX_V6_PATH_METRIC. (b) TestISISMetricWidth asserts clampMetric(MaxPathMetric-1, 10) == MaxPathMetric (negative) and clampMetric(1000, 2000) == 3000 (positive). clampMetric is the one saturating adder the IPv6 builder calls (spf/ipv6.go) and MaxPathMetric is 0xFE000000, the MAX_V6_PATH_METRIC value. Re-judged 2026-10-02: TestISISMetricWidth changed only in the prose of its RFC5305-4-1 positive tag line (claim narrowed to node reachability); the clampMetric assertions this row rests on are unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) | unit/verify | unproven |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5308.txt |
| Source fingerprint | 5357f43116f448ce |
| Record | rfc/extraction/rfc5308.json |
| Mapped sentences | 8 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 5 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence is inside a verbatim quotation of RFC 5305 that the text introduces with 'As is described in [RFC5305]:'; the up/down bit obligation is RFC 5305's, and RFC 5308 repeats it for the reader. | As is described in [RFC5305]: "The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS. |
| `2:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Second sentence of the same verbatim RFC 5305 quotation opened by 'As is described in [RFC5305]:' and closed after 'i.e., to lower levels'; the up/down bit obligation belongs to RFC 5305. | If a prefix is advertised from a higher level to a lower level (e.g. level 2 to level 1), the bit SHALL be set to 1, indicating that the prefix has traveled down the hierarchy. |

## Superseded

No document obsoletes RFC 5308, so its obligations are stated where they were written.
