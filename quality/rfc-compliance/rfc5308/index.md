# RFC 5308 - Routing IPv6 with IS-IS

IS-IS IPv6. Every requirement listed for RFC 5308, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 8 | of 8 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 8 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 8 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 8 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 8 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 30 | 30 of 30 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 8 | of 8 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 8 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 8 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 8 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 8 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 8 | of 9 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 8 mandatory requirements: they add up to 8. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IS-IS IPv6 |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tests linked to requirements | 30 |
| Recorded audit verdicts | 8 |
| Fault test records | 30 |
| Summary | `rfc/short/rfc5308.md` |
| Requirement table | `rfc/requirements/rfc5308.md` |
| RFC text | `rfc/full/rfc5308.txt` |

## Automatic checks

Included in automatic checks: Routing IPv6 with IS-IS: eight MUST-level requirements after the 2026-09-21 extraction walk. Seven are met and test-bound with positive+negative tags; the eighth, RFC5308-2-3 (the external bit of TLV 236 is set to 1 when the prefix came into IS-IS from another routing protocol, Section 2), the walk added and no test covers it. 2-1 (no link-local in IPv6 Reachability TLV 236) and 3-2 (no link-local in the LSP IPv6 address set) via internal/plugins/isis/lsdb origination tests; 3-1 (IPv6 Interface Address TLV 232 only for link-local in Hellos) and 4-1 (advertise IPv6 NLPID 0x8E in Protocols Supported) via internal/plugins/isis/circuit tests; 2-2 (do not route a metric above MaxV6PathMetric), 5-1 (up/down and level-aware preference), and 5-2 (clamp the path metric) via internal/plugins/isis/spf tests. Single-topology IS-IS: IPv6 rides the shared per-level SPF, so 5-1/5-2 reuse the IPv4 producers exercised for IPv6 via BuildRoutesV6.

## Published support status

**Area:** IS-IS IPv6

**Test verification:** 8 of 8 MUST proven

**Evidence:** unit 8

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **8** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (8):** [`RFC5308-2-1`](#rfc5308-2-1), [`RFC5308-2-2`](#rfc5308-2-2), [`RFC5308-2-3`](#rfc5308-2-3), [`RFC5308-3-1`](#rfc5308-3-1), [`RFC5308-3-2`](#rfc5308-3-2), [`RFC5308-4-1`](#rfc5308-4-1), [`RFC5308-5-1`](#rfc5308-5-1), [`RFC5308-5-2`](#rfc5308-5-2)

## Figures

- **MUST bar**: Ze verified 8
- **Overview bar**: Ze MUST verified 8, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 8 |
| MUST rows kept (the MUST bar) | 8 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5308.json](../../../data/rfc/rfc5308.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5308-2-1` | Link-local prefixes MUST NOT be advertised using this TLV. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L65). **negative:** `unit/verify` [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L66) |
| `RFC5308-2-2` | if a prefix is advertised with a metric larger than MAX_V6_PATH_METRIC (0xFE000000), this prefix MUST not be considered during the normal Shortest Path First (SPF) computation. (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L207). **positive:** `unit/verify` [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L98). **negative:** `unit/verify` [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L206). **negative:** `unit/verify` [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L100) |
| `RFC5308-2-3` | If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1. (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestRFC5308RedistributedIPv6SetsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L15). **negative:** `unit/verify` [`TestRFC5308ConnectedIPv6ClearsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L37) |
| `RFC5308-3-1` | For Hello PDUs, the "Interface Address" TLV MUST contain only the link-local IPv6 addresses assigned to the interface that is sending the Hello. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L30). **positive:** `unit/verify` [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L72). **positive:** `unit/verify` [`TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L211). **negative:** `unit/verify` [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L103). **negative:** `unit/verify` [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L130). **negative:** `unit/verify` [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L76) |
| `RFC5308-3-2` | For LSPs, the "Interface Address" TLVs MUST contain only the non-link-local IPv6 addresses assigned to the IS. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L110). **positive:** `unit/verify` [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L128). **negative:** `unit/verify` [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L111). **negative:** `unit/verify` [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L131) |
| `RFC5308-4-1` | The value of the IPv6 Network Layer Protocol ID (NLPID) is 142 (0x8E). As with [RFC1195] and IPv4, if the IS supports IPv6 routing using IS-IS, it MUST advertise this in the "NLPID" TLV by adding the IPv6 NLPID. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L31). **positive:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L148). **positive:** `unit/verify` [`TestRFC5308HelloNLPIDIs142`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L228). **negative:** `unit/verify` [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L78). **negative:** `unit/verify` [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L149) |
| `RFC5308-5-1` | The order of preference between paths for a given prefix MUST be modified to consider the up/down bit. The new order of preference is as follows (from best to worst). 1. Level 1 up prefix 2. Level 2 up prefix 3. Level 2 down prefix 4. Level 1 down prefix (§5) | MUST | 5 | **positive:** `unit/verify` [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L91). **positive:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L46). **positive:** `unit/verify` [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L156). **negative:** `unit/verify` [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L47). **negative:** `unit/verify` [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L159) |
| `RFC5308-5-2` | If, during the SPF, a path metric would exceed MAX_V6_PATH_METRIC, it SHALL be considered to be MAX_V6_PATH_METRIC. (Section 5) | SHALL | 5 | **positive:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250). **negative:** `unit/verify` [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) |
| `RFC5308-5-3` | Any remaining multiple paths SHOULD be considered for equal-cost multi-path routing if the router supports this; otherwise, the router can select any one of the multiple paths. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5308 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5308-2-1`](#rfc5308-2-1)

Link-local prefixes MUST NOT be advertised using this TLV. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a fe80::/64 link-local prefix advertised in TLV 236. (b) TestISISOriginateTLV236 decodes the originated LSP and errors on got["fe80::/64"] present (negative); the same unit requires the two routable prefixes present with their metric and X bit (positive). The unit runs NonLinkLocalV6Prefixes, the filter lsdb_wiring.go calls on every TLV 236 origination, then Originator.Originate, so breaking the filter reddens it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L66) | unit/verify | mutant, verified |
| positive | [`TestISISOriginateTLV236`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L65) | unit/verify | revert, verified |

### [`RFC5308-2-2`](#rfc5308-2-2)

if a prefix is advertised with a metric larger than MAX_V6_PATH_METRIC (0xFE000000), this prefix MUST not be considered during the normal Shortest Path First (SPF) computation. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a TLV 236 prefix advertised with a metric larger than MAX_V6_PATH_METRIC taking part in normal SPF. (b) spf TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered: node 3 (distance 10) advertises 2001:db8:9::/64 at 50 (positive, installed at 60 via node 3); node 2 (distance 10) advertises the same prefix at 0xFE000001 and 2001:db8:8::/64 only at 0xFFFFFFFF (negative): the shared prefix keeps one next hop at 60 and the lone prefix is absent. The lone prefix is the discriminating input: any BuildRoutesV6 that considered it would install it (at MAX after clampMetric saturation, or at a wrapped cost without it). The per-entry filter (ipv6.go, p.Metric > MaxV6PathMetric) and the accumulated-cost ceiling (total >= MaxPathMetric) are redundant for every over-max input, so no input isolates the filter alone; removing either one alone leaves Ze compliant, and the test asserts the stack outcome the RFC states. Recorded +/- on BuildRoutesV6.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestISISIPv6MetricAboveMaxIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L207) | unit/verify | revert, verified |

### [`RFC5308-2-3`](#rfc5308-2-3)

If the prefix was distributed into IS-IS from another routing protocol, the external bit SHALL be set to 1. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a prefix distributed into IS-IS from another routing protocol originated with the X bit 0. (b) TestRFC5308RedistributedIPv6SetsExternalBit injects a bgp-sourced IPv6 route through the redistribute Consumer and t.Fatalf's on !got[0].External at Level1 and Level2 (positive); TestRFC5308ConnectedIPv6ClearsExternalBit fails if a connected prefix carries External (negative), so an always-set bit is caught too.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5308ConnectedIPv6ClearsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC5308RedistributedIPv6SetsExternalBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/redistribute/rfc5308_ipv6_test.go#L15) | unit/verify | revert, verified |

### [`RFC5308-3-1`](#rfc5308-3-1)

For Hello PDUs, the "Interface Address" TLV MUST contain only the link-local IPv6 addresses assigned to the interface that is sending the Hello. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. (a) forbidden: a Hello TLV 232 carrying a non-link-local address, or a link-local address not assigned to the sending interface. (b) The earlier gap (the production address source untested) is closed: root TestRFC5308HelloAddressIsTheInterfaceLinkLocal registers a test iface backend (only the OS address list is fake; the resolver's link-local classification is real) with two interfaces holding distinct link-local addresses, and requires interfaceIPv6LinkLocal to return fe80::a:1 for isis-ll-a although its global address is listed first, and fe80::b:2 for isis-ll-b (positive); a global-only interface and an interface without ipv6-unicast yield none (negative). circuits.go builds each circuit's Config.IPv6LinkLocal from interfaceIPv6LinkLocal(ic) for that same interface, and circuit TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal proves each circuit sends exactly its own configured address; TestISISIIHTLV232RejectsNonLinkLocal and TestISISIIHTLV232LinkLocal cover the encoder's link-local-only guard. Recorded +/- on circuits.go interfaceIPv6LinkLocal.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISIIHTLV232OmittedNoLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestISISIIHTLV232RejectsNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloTLV232IsTheSendingCircuitLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloAddressIsTheInterfaceLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L72) | unit/verify | revert, verified |

### [`RFC5308-3-2`](#rfc5308-3-2)

For LSPs, the "Interface Address" TLVs MUST contain only the non-link-local IPv6 addresses assigned to the IS. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. (a) forbidden: an LSP TLV 232 carrying a link-local address. (b) The earlier gap (the production filter untested) is closed: root TestRFC5308LSPInterfaceAddressesAreNonLinkLocal runs the engine over the test iface backend with two IPv6 circuits and decodes fragment 0 at both levels, built by the production path (lsdb_wiring.go levelState -> interfaceIPv6NonLinkLocal -> Originate): TLV 232 is exactly [2001:db8:a::1] (positive), and neither fe80::a:1 (same circuit) nor fe80::b:2 (the other circuit's only IPv6 address) appears (negative). Recorded +/- on circuits.go interfaceIPv6NonLinkLocal. lsdb TestISISOriginateTLV232Scope remains tagged and pre-filters its input; it carries no weight.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L111) | unit/verify | revert, verified |
| negative | [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestISISOriginateTLV232Scope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC5308LSPInterfaceAddressesAreNonLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5308_hello_link_local_test.go#L128) | unit/verify | revert, verified |

### [`RFC5308-4-1`](#rfc5308-4-1)

The value of the IPv6 Network Layer Protocol ID (NLPID) is 142 (0x8E). As with [RFC1195] and IPv4, if the IS supports IPv6 routing using IS-IS, it MUST advertise this in the "NLPID" TLV by adding the IPv6 NLPID. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an IPv6-capable IS omitting the IPv6 NLPID from TLV 129, or advertising a value other than 142 (0x8E). (b) circuit TestRFC5308HelloNLPIDIs142 checks the transmitted dual-stack IIH's TLV 129 wire octets equal exactly cc 8e, against the literal rather than packet.NLPIDIPv6, closing the constant gap; TestISISProtocolsSupportedDualStack and TestISISIIHTLV232LinkLocal cover the advertise clause and the IPv4-only cases (TestISISProtocolsSupportedDualStack ipv4-only, TestISISIIHNoTLV232WhenIPv4Only) the absence when IPv6 is not supported. Recorded + on hello.go protocolsSupportedTLV.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISIIHNoTLV232WhenIPv4Only`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L78) | unit/verify | revert, verified |
| negative | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC5308HelloNLPIDIs142`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L228) | unit/verify | revert, verified |
| positive | [`TestISISIIHTLV232LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5308_hello_ipv6_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestISISProtocolsSupportedDualStack`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc5308_origination_ipv6_test.go#L148) | unit/verify | revert, verified |

### [`RFC5308-5-1`](#rfc5308-5-1)

The order of preference between paths for a given prefix MUST be modified to consider the up/down bit. The new order of preference is as follows (from best to worst). 1. Level 1 up prefix 2. Level 2 up prefix 3. Level 2 down prefix 4. Level 1 down prefix (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: any path order other than L1-up > L2-up > L2-down > L1-down. (b) spf TestRFC5308FourStepPathPreference advertises the four classes each from its own node at metrics that invert the class order (400, 300, 200, 100), then drops the best class in turn: the installed route is always the best class left (checked by next hop, level and up/down), so each adjacent pair is decided by class not metric, and a separate case pits L1 up against L2 down alone. This closes the two pairs the previous note named (L2-up over L2-down, L1-up over L2-down). Recorded +/- on route.go preferenceRank, which BuildRoutesV6 uses. TestISISLeakUpDownBit and TestISISIPv6LevelArbitration remain tagged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L159) | unit/verify | revert, verified |
| negative | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L47) | unit/verify | revert, verified |
| positive | [`TestRFC5308FourStepPathPreference`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5305_max_metric_order_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestISISIPv6LevelArbitration`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_ipv6_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestISISLeakUpDownBit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc5308_route_test.go#L46) | unit/verify | revert, verified |

### [`RFC5308-5-2`](#rfc5308-5-2)

If, during the SPF, a path metric would exceed MAX_V6_PATH_METRIC, it SHALL be considered to be MAX_V6_PATH_METRIC. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a path metric that exceeds MAX_V6_PATH_METRIC kept (or wrapped) instead of taken as MAX_V6_PATH_METRIC. (b) TestISISMetricWidth asserts clampMetric(MaxPathMetric-1, 10) == MaxPathMetric (negative) and clampMetric(1000, 2000) == 3000 (positive). clampMetric is the one saturating adder the IPv6 builder calls (spf/ipv6.go) and MaxPathMetric is 0xFE000000, the MAX_V6_PATH_METRIC value. Re-judged 2026-10-02: TestISISMetricWidth changed only in the prose of its RFC5305-4-1 positive tag line (claim narrowed to node reachability); the clampMetric assertions this row rests on are unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L251) | unit/verify | mutant, verified |
| positive | [`TestISISMetricWidth`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/spf_test.go#L250) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 5 | reviewed | not stated |
| `3` | not stated | 2 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence is inside a verbatim quotation of RFC 5305 that the text introduces with 'As is described in [RFC5305]:'; the up/down bit obligation is RFC 5305's, and RFC 5308 repeats it for the reader. | As is described in [RFC5305]: "The up/down bit SHALL be set to 0 when a prefix is first injected into IS-IS. |
| `2:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Second sentence of the same verbatim RFC 5305 quotation opened by 'As is described in [RFC5305]:' and closed after 'i.e., to lower levels'; the up/down bit obligation belongs to RFC 5305. | If a prefix is advertised from a higher level to a lower level (e.g. level 2 to level 1), the bit SHALL be set to 1, indicating that the prefix has traveled down the hierarchy. |

## Superseded

No document obsoletes RFC 5308, so its obligations are stated where they were written.
