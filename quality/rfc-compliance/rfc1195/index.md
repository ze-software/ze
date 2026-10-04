# RFC 1195 - Use of OSI IS-IS for Routing in TCP/IP and Dual Environments

Experimental. Every requirement this repository extracted from RFC 1195, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.7% | 29 of 41 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.2% | 5 of 41 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 41 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 41 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 41 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 69.0% | 58 of 84 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 41 | of 42 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 41 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 17.1% | 7 of 41 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 41 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 41 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 41 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 42 |
| Gated MUST-level | 41 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 84 |
| Tagged units | 84 |
| Recorded audit verdicts | 31 |
| Discrimination records | 58 |
| Summary | `rfc/short/rfc1195.md` |
| Requirement shard | `rfc/requirements/rfc1195.md` |
| RFC text | `rfc/full/rfc1195.txt` |

## Enrolment

Enrolled: Integrated IS-IS for IP routing. The checklist records the requirements extracted from RFC 1195; the current native gate and discrimination records, not a fixed historical count, determine verified coverage. The started implementation includes narrow IPv4 reachability, ISO 9542 point-to-point discovery and scoped password authentication.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- Native IS-IS over Layer 2, L1/L2 broadcast and point-to-point circuits, IPv4 interface advertisement, node-wide protocol capability, narrow IPv4 SPF and leaking, and terminal forwarding rejection for unsupported protocol families. `circuit/ish.go` sends and receives ISO 9542 discovery
- `ish_auth.go` applies configured cleartext link passwords before discovery changes state. IS-IS Hello, area and domain authentication use their scoped key chains. This describes source coverage, not a completed validation run.


**What the ledger says remains**

Experimental pending current integration, discrimination and deployment evidence. Native IDRPI origination, manually configured summaries, extended-fragment operation and TE path computation remain outside the selected boundary. ISH discovery does not replace authenticated IIH adjacency establishment. TOS/QOS metrics and partition repair remain outside scope; Section 3.5 makes TOS/QOS optional.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 29 | one part of the gated population |
| Annotated (including scoped evidence) | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **41** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (29):** [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1), [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2), [`RFC1195-5.3.5-2`](#rfc1195-5.3.5-2), [`RFC1195-5.2-4`](#rfc1195-5.2-4), [`RFC1195-3.2-1`](#rfc1195-3.2-1), [`RFC1195-3.9-1`](#rfc1195-3.9-1), [`RFC1195-1.4-2`](#rfc1195-1.4-2), [`RFC1195-3.2-2`](#rfc1195-3.2-2), [`RFC1195-3.3-1`](#rfc1195-3.3-1), [`RFC1195-3.3-2`](#rfc1195-3.3-2), [`RFC1195-3.4-1`](#rfc1195-3.4-1), [`RFC1195-3.10-1`](#rfc1195-3.10-1), [`RFC1195-3.10-2`](#rfc1195-3.10-2), [`RFC1195-3.10-3`](#rfc1195-3.10-3), [`RFC1195-4.1-1`](#rfc1195-4.1-1), [`RFC1195-4.2-1`](#rfc1195-4.2-1), [`RFC1195-4.4-1`](#rfc1195-4.4-1), [`RFC1195-4.4-2`](#rfc1195-4.4-2), [`RFC1195-4.4-3`](#rfc1195-4.4-3), [`RFC1195-4.5-1`](#rfc1195-4.5-1), [`RFC1195-5.3.4-3`](#rfc1195-5.3.4-3), [`RFC1195-7-1`](#rfc1195-7-1), [`RFC1195-7-2`](#rfc1195-7-2), [`RFC1195-7-3`](#rfc1195-7-3), [`RFC1195-7-4`](#rfc1195-7-4), [`RFC1195-7-5`](#rfc1195-7-5), [`RFC1195-7-6`](#rfc1195-7-6), [`RFC1195-7-7`](#rfc1195-7-7), [`RFC1195-7-8`](#rfc1195-7-8)

**Annotated (including scoped evidence) (12):** [`RFC1195-5.2-1`](#rfc1195-5.2-1), [`RFC1195-5.2-2`](#rfc1195-5.2-2), [`RFC1195-5.2-3`](#rfc1195-5.2-3), [`RFC1195-3.4-2`](#rfc1195-3.4-2), [`RFC1195-3.1-1`](#rfc1195-3.1-1), [`RFC1195-1.4-1`](#rfc1195-1.4-1), [`RFC1195-1.4-3`](#rfc1195-1.4-3), [`RFC1195-1.2-1`](#rfc1195-1.2-1), [`RFC1195-3.2-3`](#rfc1195-3.2-3), [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1), [`RFC1195-8-1`](#rfc1195-8-1), [`RFC1195-8-2`](#rfc1195-8-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1195-5.2-1` | The "Protocols Supported" field identifies the protocols which are supported by each router. This field must be included in all IS-IS Hello packets and all LSPs with LSP number 0 transmitted by IP- capable routers. If this field is not included in an IS-IS Hello packet or an LSP with LSP number 0, it may be assumed that the packet was transmitted by an OSI-only router. The "Protocols Supported" field must also be included in ISO 9542 ISHs send by IP-capable routers over point-to-point links to other IS-IS routers. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L177). **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_origination_test.go#L122). **positive:** `unit/verify` [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L242). **positive:** `unit/verify` [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L138). **negative:** no negative test. **{single-polarity}:** ze unconditionally emits the Protocols Supported TLV 129 with NLPID 0xCC for every IP-capable circuit IIH (internal/plugins/isis/circuit/hello.go:97-104) and LSP fragment 0 (internal/plugins/isis/lsdb/origination.go:407-409); there is no code path that omits TLV 129 or emits a non-0xCC IPv4 NLPID, so there is no negative form to reject |
| `RFC1195-5.2-2` | In Link State Packets, this field contains a list of one or more IP addresses corresponding to one or more interfaces of the router which originates the LSP. Each IP-capable router must include this field in its LSPs. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_origination_test.go#L110). **positive:** `unit/verify` [`TestRFC1195SameInterfaceAddressesBothLevels`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L321). **negative:** no negative test. **{single-polarity}:** ze packs the IP Interface Address TLV 132 into LSP fragment 0 from the node's own interface addresses (internal/plugins/isis/lsdb/origination.go:433-438) and the codec reads it back verbatim (internal/plugins/isis/packet/tlv_ipv4.go DecodeIPv4InterfaceAddrTLV); this is an emit obligation with no decode-side reject-on-absence path, so there is no negative form to drive |
| `RFC1195-5.2-3` | Where a single router operates as both a level 1 and a level 2 router, it is required to include the same IP address(es) in its level 1 and level 2 LSPs. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC1195SameInterfaceAddressesBothLevels`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L320). **negative:** no negative test. **{single-polarity}:** ze collects interface addresses level-independently (internal/plugins/isis/lsdb_wiring.go levelState, the TLV 132 loop over every circuit) so the same set is advertised at both levels by construction; there is no per-level divergence code path to drive a negative |
| `RFC1195-5.3.4-1` | Bit 7 of this field (marked I/E) indicates the metric type (internal or external) for all four TOS metrics, and must be set to zero indicating internal metrics. (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L311). **negative:** `unit/verify` [`TestRFC1195InternalOriginatorClearsExternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L410) |
| `RFC1195-5.3.4-2` | IP Internal Reachability Information -- IP addresses within the routing domain reachable directly via one or more interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.4) | MUST NOT | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L445). **negative:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L446) |
| `RFC1195-5.3.5-2` | IP External Reachability Information -- IP addresses outside the routing domain reachable via interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.5) | MUST NOT | 5.3.5 | **positive:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L447). **negative:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L448) |
| `RFC1195-3.4-2` | The inter- domain routing protocol information field is not included in pseudonode LSPs. (§3.4) | MUST NOT | 3.4 | **positive:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L449). **negative:** no negative test. **{single-polarity}:** ze originates TLV 131 in no LSP (the IS-IS codec has no TLV 131 type, internal/plugins/isis/packet/tlv.go), so no input can drive a producer toward placing one in a pseudonode LSP and no refusal path exists; the positive proves the pseudonode LSP Ze builds carries none |
| `RFC1195-5.2-4` | Each entry must contain a default metric (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC1195OriginatedExternalEntriesCarryDefaultMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L82). **negative:** `unit/verify` [`TestRFC1195ExternalEntryWithoutDefaultMetricRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L71) |
| `RFC1195-3.2-1` | Note: If this sum results in a metric value greater than 63 (the maximum value that can be reported in level 2 LSPs), then the value 63 must be used. (Section 3.2; wide metrics use RFC 5305's separate bound) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L310). **negative:** `unit/verify` [`TestRFC1195NarrowLeakedMetricBelowCeiling`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L319) |
| `RFC1195-3.9-1` | If a packet is received which contains invalid authentication information, then the entire packet is discarded. (§3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134). **positive:** `unit/verify` [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L27). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611). **negative:** `unit/verify` [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L28) |
| `RFC1195-3.1-1` | Any codes in a received PDU that are not recognised shall be ignored and, for those packets which are forwarded (specifically Link State Packets), passed on unchanged. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16). **positive:** `unit/verify` [`TestRFC1195UnknownTLVsFloodedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go#L23). **negative:** no negative test. **{single-polarity}:** the codec retains every TLV type it does not recognize as an opaque span and re-encodes it byte-for-byte (internal/plugins/isis/packet/tlv.go:66-89 iterator plus DecodeTLVs / writeTLVs) so the LSDB re-floods unknown TLVs verbatim (internal/plugins/isis/lsdb/flooding.go:342-360); preserving unrecognized codes IS the requirement and there is no reject path for an unknown TLV, so no negative form exists |
| `RFC1195-1.4-1` | Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4) | MUST | 1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |
| `RFC1195-1.4-3` | In a dual area within a dual routing domain only dual routers may be used. (§1.4) | MUST | 1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so it never forms or joins a dual area and the dual-area membership constraint does not apply |
| `RFC1195-1.2-1` | External links (to other routing domains) must be from level 2 routers (Section 1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 2966 Section 2.2, which updates this rule, "loosens the restrictions in RFC 1195, and allows for the inclusion of the "IP External Reachability Information" TLV in L1 LSPs"; ze advertises redistributed routes at every origination level (internal/plugins/isis/redistribute/consumer.go::InjectRoute), which the updated rule permits, and configures no circuit to another routing domain |
| `RFC1195-1.4-2` | In a pure IP routing domain, all routers must be IP-capable (Section 1.4) | MUST | 1.4 | **positive:** `unit/verify` [`TestRFC1195PureIPAdvertisesIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L191). **negative:** `unit/verify` [`TestRFC1195IPv6SelectionDoesNotRemoveIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L204) |
| `RFC1195-3.2-2` | In general, the same [IP address, subnet mask] pair may be announced in level 1 LSPs sent by multiple level 1 routers in the same area. In this case (assuming the entry is not superceded by a manually configured entry), then only one such entry shall be included in the level 2 LSP. The metric value(s) announced in level 2 LSPs correspond to the minimum of the metric value(s) that would be calculated for each of the level 1 LSP entries. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L435). **negative:** `unit/verify` [`TestRFC1195SpecificLeakKeepsOnlyMinimum`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L123) |
| `RFC1195-3.2-3` | If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep specific prefixes". Section 3.2 makes manual summary configuration optional ("Each level 2 router may be configured with one or more [IP address, subnet mask, metric] entries"). internal/plugins/isis/yang/ze-isis-conf.yang has no manual-summary configuration; spf.LeakPrefixes and lsdb.Originator.fragmentTLVs originate specific prefixes, so no locally configured summary exists to match this conditional discard rule. |
| `RFC1195-3.3-1` | The reserved field must contain "00 00", as specified in GOSIP version 2.0. (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC1195ISHGOSIPReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L15). **negative:** `unit/verify` [`TestRFC1195ISHGOSIPReservedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L39) |
| `RFC1195-3.3-2` | IEEE 802 addresses, if used, must appear in IEEE canonical format (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC1195ISHCanonicalIEEEIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L60). **negative:** `unit/verify` [`TestRFC1195ISHDoesNotReverseIdentifierBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L84) |
| `RFC1195-3.4-1` | When an external route using external metrics must be used, the lowest value of the external metric is preferred regardless of the internal cost to reach the appropriate exit point (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC1195ExternalMetricBeforeExitCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L148). **negative:** `unit/verify` [`TestRFC1195ExternalMetricWithdrawalRecomputesWinner`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L159) |
| `RFC1195-3.10-1` | The Dijkstra calculation must calculate routes to each distinct IP reachability entry (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L29). **negative:** `unit/verify` [`TestRFC1195NoRouteForUnreachedReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L64) |
| `RFC1195-3.10-2` | If level 2 routes must be used, then routes within the routing domain (specifically, those routes using internal metrics) are prefered to routes outside of the routing domain (using external metrics). (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L169). **negative:** `unit/verify` [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L178) |
| `RFC1195-3.10-3` | However, the default metric must always be available. (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L32). **negative:** `unit/verify` [`TestRFC1195InternalEntryWithoutDefaultMetricRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L60) |
| `RFC1195-3.10.2-1` | 3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2) | MUST | 3.10.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep specific prefixes". The optional manual summaries described by section 3.2 ("Each level 2 router may be configured") have no configuration or origination path in internal/plugins/isis/yang/ze-isis-conf.yang, spf.LeakPrefixes, or lsdb.Originator.fragmentTLVs. There is no local manual-summary candidate to become the most desirable route; specific-prefix forwarding remains subject to normal reachability and protocol-suite rejection. |
| `RFC1195-4.1-1` | This implies that all IS-IS routers, including IP-only routers, must be able to receive IS-IS packets using the normal encapsulation for OSI packets. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC1195ParseFrameOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_frame_test.go#L33). **negative:** `unit/verify` [`TestRFC1195ParseFrameRefusesNonOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_frame_test.go#L50) |
| `RFC1195-4.2-1` | However, IP-capable routers must be able to interact correctly with other routers which assign multiple IP addresses per physical interface (up to the maximum of 63 addresses per interface). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC1195InterfaceAddrTLV63Addresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_ipv4_test.go#L31). **positive:** `unit/verify` [`TestRFC1195MultiAddressNeighborForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L124). **negative:** `unit/verify` [`TestRFC1195InterfaceAddrTLVTruncatedAddressRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_ipv4_test.go#L54). **negative:** `unit/verify` [`TestRFC1195MultiAddressNeighborRenumbered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L145) |
| `RFC1195-4.4-1` | IP-capable IS-IS routers therefore must be able to forward IP packets over existing adjacencies to routers with which they share physical connectivity, even when the IP address of the adjacent interface of the neighboring router is on a different logical IP subnet. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L36). **positive:** `unit/verify` [`TestRFC1195OffSubnetNeighborIsOnLinkNextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L168). **negative:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L37). **negative:** `unit/verify` [`TestRFC1195OffSubnetNeighborNotResolvedBySubnet`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L190) |
| `RFC1195-4.4-2` | All IS-IS routers are therefore required to transmit and receive ISO 9542 ISH packets on point-to-point links. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC1195ISHConfiguredPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_auth_test.go#L22). **positive:** `unit/verify` [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L239). **positive:** `unit/verify` [`TestRFC1195ISHVethTransport`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_ish_integration_linux_test.go#L19). **negative:** `unit/verify` [`TestRFC1195ISHRejectsCorruptionAndWrongCircuit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L324). **negative:** `unit/verify` [`TestRFC1195ISHWrongPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_auth_test.go#L64). **negative:** `unit/verify` [`TestRFC1195NeighborWithoutISHStillFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_absence_test.go#L43) |
| `RFC1195-4.4-3` | Thus, the value of the "protocols supported" field must be identical on every link (i.e., for any one router running IS-IS, all of the Hellos and LSPs transmitted by it must contain the same "protocols supported" values). (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L135). **negative:** `unit/verify` [`TestRFC1195NodeProtocolsReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L171) |
| `RFC1195-4.5-1` | There may be times when a dual router has to forward an IP packet to an OSI-only router, or forward an OSI packet to an IP-only router. In this case the packet must be discarded. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L38). **positive:** `unit/verify` [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L495). **positive:** `unit/verify` [`TestRFC1195OSINeighborIsTerminalIPv4NextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_adjacency_protocol_test.go#L39). **negative:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L39). **negative:** `unit/verify` [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L496). **negative:** `unit/verify` [`TestRFC1195NeighborProtocolTransitions`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_adjacency_protocol_test.go#L56) |
| `RFC1195-5.3.4-3` | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. (Section 5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195NarrowMetricReservedTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_narrow_ipv4_test.go#L14). **negative:** `unit/verify` [`TestRFC1195NarrowMetricReservedReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_narrow_ipv4_test.go#L32) |
| `RFC1195-7-1` | The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). (§B.1) | MUST | B.1 - Annex subsection B.1 | **positive:** `unit/verify` [`TestRFC1195CSNPEntriesAscending`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_snp_test.go#L31). **negative:** `unit/verify` [`TestRFC1195PSNPEntriesAscendingAcrossLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_snp_test.go#L59) |
| `RFC1195-7-2` | However, a full update must be done periodically to ensure recovery from data corruption, and studies suggest that with a very small number of link changes (perhaps 2) the expected computation complexity of the incremental update exceeds the complete recalculation. (§C.1) | MUST | C.1 - Annex subsection C.1 | **positive:** `unit/verify` [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L331). **negative:** `unit/verify` [`TestRFC1195PeriodicSPFRemovesMissedWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L172) |
| `RFC1195-7-3` | The 8 octet system identifiers which specify IP reachability entries must always be distinguishable from other system identifiers (§C.1.1) | MUST | C.1.1 - Annex subsection C.1.1 | **positive:** `unit/verify` [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L93). **negative:** `unit/verify` [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L142) |
| `RFC1195-7-4` | IP-capable level 2 routers must keep level 2 internal IP routes separate from level 2 external IP routes (§C.1.4) | MUST | C.1.4 - Annex subsection C.1.4 | **positive:** `unit/verify` [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L170). **negative:** `unit/verify` [`TestRFC1195ExternalSetRetainedBesideInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L134) |
| `RFC1195-7-5` | Each entry made to TENT must be marked as being either an End System or a router (§C.1.4) | MUST | C.1.4 - Annex subsection C.1.4 | **positive:** `unit/verify` [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L97). **negative:** `unit/verify` [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L143) |
| `RFC1195-7-6` | The password shall be configured on a per-link, per-area, and per- domain basis. (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195PasswordScopesResolved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L49). **negative:** `unit/verify` [`TestRFC1195PasswordScopesDoNotLeak`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L71) |
| `RFC1195-7-7` | Specifically, when this form of authentication is used: - IS-IS Hello and 9542 IS Hello packets shall contain the per-link password - Level 1 Link State Packets shall contain the per-area password - Level 2 Link State Packets shall contain the per-domain password - Level 1 Sequence Number Packets shall contain the per-area password - Level 2 Sequence Number Packets shall contain the per-domain password (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195ISHCarriesPerLinkPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_scope_test.go#L36). **positive:** `unit/verify` [`TestRFC1195PDUClassSignedWithItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L95). **negative:** `unit/verify` [`TestRFC1195ISHRefusesAreaAndDomainPasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_scope_test.go#L69). **negative:** `unit/verify` [`TestRFC1195PDUClassRefusedUnderOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L132) |
| `RFC1195-7-8` | Also, each of these three passwords shall be configured with: (i) "Transmit Password", whose value is a single password, and (ii) "Receive Passwords", whose value is a set of passwords. (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195OneTransmitPasswordManyReceivePasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L204). **negative:** `unit/verify` [`TestRFC1195KeyOutsideReceiveSetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L230) |
| `RFC1195-8-1` | Type = 2 indicates that the external information field contains an autonomous system number tag, to be applied to subsequent IP external reachability information entries. In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. The AS tag is associated with subsequent IP External Reachability entries, until the end of the LSP, or until the next occurence of the Inter-Domain Routing Protocol Information field. (§A.2) | MUST | A.2 - Annex subsection A.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 says the IDRPI field "may be present" and "is not used by the IS-IS". internal/plugins/isis/lsdb/origination.go emits no TLV 131 and spf/graph.go has no IDRPI consumer; packet.DecodeTLVs retains unknown TLV 131 only for unchanged flooding. No AS-number association role is selected. |
| `RFC1195-8-2` | Type = 0 is reserved (must not be sent, and must be ignored on receipt). (Section A.2) | MUST NOT | A.2 - Annex subsection A.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 makes the IDRPI field optional ("may be present"). internal/plugins/isis/lsdb/origination.go has no TLV 131 producer; spf/graph.go consumes no IDRPI types, and packet.DecodeTLVs only retains the opaque field for unchanged flooding. Ze therefore neither originates type 0 nor interprets a received type 0. |
| `RFC1195-5.3.5-1` | Bit 7 of this field indicates the metric type (internal or external) for all four TOS metrics, and may be set to zero indicating internal metrics, or may be set to 1 indicating external metrics. (§5.3.5) | MAY | 5.3.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1195-1.4-1`](#rfc1195-1.4-1) Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |
| [`RFC1195-1.4-3`](#rfc1195-1.4-3) In a dual area within a dual routing domain only dual routers may be used. (§1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so it never forms or joins a dual area and the dual-area membership constraint does not apply |
| [`RFC1195-1.2-1`](#rfc1195-1.2-1) External links (to other routing domains) must be from level 2 routers (Section 1.2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 2966 Section 2.2, which updates this rule, "loosens the restrictions in RFC 1195, and allows for the inclusion of the "IP External Reachability Information" TLV in L1 LSPs"; ze advertises redistributed routes at every origination level (internal/plugins/isis/redistribute/consumer.go::InjectRoute), which the updated rule permits, and configures no circuit to another routing domain |
| [`RFC1195-3.2-3`](#rfc1195-3.2-3) If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep specific prefixes". Section 3.2 makes manual summary configuration optional ("Each level 2 router may be configured with one or more [IP address, subnet mask, metric] entries"). internal/plugins/isis/yang/ze-isis-conf.yang has no manual-summary configuration; spf.LeakPrefixes and lsdb.Originator.fragmentTLVs originate specific prefixes, so no locally configured summary exists to match this conditional discard rule. |
| [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1) 3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep specific prefixes". The optional manual summaries described by section 3.2 ("Each level 2 router may be configured") have no configuration or origination path in internal/plugins/isis/yang/ze-isis-conf.yang, spf.LeakPrefixes, or lsdb.Originator.fragmentTLVs. There is no local manual-summary candidate to become the most desirable route; specific-prefix forwarding remains subject to normal reachability and protocol-suite rejection. |
| [`RFC1195-8-1`](#rfc1195-8-1) Type = 2 indicates that the external information field contains an autonomous system number tag, to be applied to subsequent IP external reachability information entries. In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. The AS tag is associated with subsequent IP External Reachability entries, until the end of the LSP, or until the next occurence of the Inter-Domain Routing Protocol Information field. (§A.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 says the IDRPI field "may be present" and "is not used by the IS-IS". internal/plugins/isis/lsdb/origination.go emits no TLV 131 and spf/graph.go has no IDRPI consumer; packet.DecodeTLVs retains unknown TLV 131 only for unchanged flooding. No AS-number association role is selected. |
| [`RFC1195-8-2`](#rfc1195-8-2) Type = 0 is reserved (must not be sent, and must be ignored on receipt). (Section A.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 makes the IDRPI field optional ("may be present"). internal/plugins/isis/lsdb/origination.go has no TLV 131 producer; spf/graph.go consumes no IDRPI types, and packet.DecodeTLVs only retains the opaque field for unchanged flooding. Ze therefore neither originates type 0 nor interprets a received type 0. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1195-5.2-1`](#rfc1195-5.2-1)

The "Protocols Supported" field identifies the protocols which are supported by each router. This field must be included in all IS-IS Hello packets and all LSPs with LSP number 0 transmitted by IP- capable routers. If this field is not included in an IS-IS Hello packet or an LSP with LSP number 0, it may be assumed that the packet was transmitted by an OSI-only router. The "Protocols Supported" field must also be included in ISO 9542 ISHs send by IP-capable routers over point-to-point links to other IS-IS routers. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (TestISISIIHOriginationTLVs claim block changed: its RFC3787 tags were split; this row's tag and the unit body are unchanged). Single-polarity positive per the row (Ze is unconditionally IP-capable). TestISISIIHOriginationTLVs asserts TLV 129 in the decoded sent L1 LAN IIH and P2P IIH; root TestRFC1195NodeProtocolsAcrossInterfaces asserts TLV 129 with NLPID 0xCC in each interface's IIH and LSP number 0; root TestRFC1195ISHTransmitReceive the ISH on a point-to-point circuit; lsdb TestISISOriginateOnAdjacencyUp LSP fragment 0. Every place the sentence names is asserted. Recorded + on circuit/hello.go protocolsSupportedTLV (three units); the lsdb unit carries no record of its own. Re-read 2026-09-30 by judge after TestISISIIHOriginationTLVs gained a TLV 132 value assertion (RFC3787-10-1); its TLV 129 assertions in the LAN and P2P IIH are unchanged, so the verdict stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISIIHOriginationTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/hello_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_origination_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L242) | unit/verify | revert, verified |
| positive | [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L138) | unit/verify | revert, verified |

### [`RFC1195-5.2-2`](#rfc1195-5.2-2)

In Link State Packets, this field contains a list of one or more IP addresses corresponding to one or more interfaces of the router which originates the LSP. Each IP-capable router must include this field in its LSPs. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive per the row. Closure rejudge 2026-10-02: the tag on packet TestISISTLVIPv4InterfaceAddr was removed (decode-only, it cannot reach Originate or levelState, so it never proved that the router includes the field); its revert record on tlv_ipv4.go::DecodeIPv4InterfaceAddrTLV is left as an orphan in rfc/discrimination/rfc1195.json. The verdict rests, unchanged, on root TestRFC1195SameInterfaceAddressesBothLevels (L1L2 engine, two circuits, fragment 0's TLV 132 at each level must be exactly {192.0.2.9, 198.51.100.9}; recorded + on lsdb_wiring.go levelState) and lsdb TestISISOriginateOnAdjacencyUp (recorded + on origination.go Originate).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_origination_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC1195SameInterfaceAddressesBothLevels`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L321) | unit/verify | revert, verified |

### [`RFC1195-5.2-3`](#rfc1195-5.2-3)

Where a single router operates as both a level 1 and a level 2 router, it is required to include the same IP address(es) in its level 1 and level 2 LSPs. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive per the row annotation (no per-level divergence path to drive a refusal). TestRFC1195SameInterfaceAddressesBothLevels runs an L1L2 engine with two circuits on the connected-prefix test backend and requires the TLV 132 set of its own fragment 0 at Level 1 and at Level 2 to equal exactly {192.0.2.9, 198.51.100.9}; a producer that dropped or added an address at one level goes red. Recorded red on lsdb_wiring.go levelState. The old tag on TestISISEngineOriginateOnAdjacencyUp, which compared nothing, is gone. Re-judged 2026-09-30: TestRFC1195SameInterfaceAddressesBothLevels changed only by an added RFC1195-5.2-2 tag comment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1195SameInterfaceAddressesBothLevels`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L320) | unit/verify | revert, verified |

### [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1)

Bit 7 of this field (marked I/E) indicates the metric type (internal or external) for all four TOS metrics, and must be set to zero indicating internal metrics. (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a leaked internal L2 entry is decoded with ExternalMetric false. Negative: a producer asking for ExternalMetric on an internal prefix still emits TLV 128 with bit 0x40 clear. Both would fail if the originator set I/E.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InternalOriginatorClearsExternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L410) | unit/verify | unproven |
| positive | [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L311) | unit/verify | unproven |

### [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2)

IP Internal Reachability Information -- IP addresses within the routing domain reachable directly via one or more interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestRFC1195PseudonodeCannotInheritRouterPrefixes gained only the RFC1195-5.3.5-2 and RFC1195-3.4-2 tag comments; the body is byte-identical. The unit fails if the pseudonode LSP carries TLV 128 while the router LSP carries the narrow internal prefix (routerTypes[128] required), and the pseudonode's TLV 22 member entry is the positive. Positive and negative tags sit on the one assertion; the input is the violation attempt.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L446) | unit/verify | unproven |
| positive | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L445) | unit/verify | unproven |

### [`RFC1195-5.3.5-2`](#rfc1195-5.3.5-2)

IP External Reachability Information -- IP addresses outside the routing domain reachable via interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30 (routing phase-8 judge). TestRFC1195PseudonodeCannotInheritRouterPrefixes originates a Level 2 router LSP carrying a narrow external prefix (203.0.113.0/24, External+ExternalMetric) so the router LSP holds TLV 130 (the unit requires routerTypes[130]), then a pseudonode LSP from the same system into the same LSDB; every pseudonode TLV is decoded and TLV 130 there is t.Fatalf. Positive: the pseudonode exists and carries the member neighbor in TLV 22 (neighborSeen). Negative: inputs forced toward the violation (the originating router holds TLV 130 in the same LSDB) and the pseudonode still has none; one assertion wearing both tags, as for RFC1195-5.3.4-2, valid because the input is the violation attempt. Producer OriginatePseudonode builds only from Members, so there is no input that selects a TLV 130. Records are body-panic reverts on lsdb/pseudonode.go::OriginatePseudonode (coarse, but the assertion itself is the discriminator).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L448) | unit/verify | revert, verified |
| positive | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L447) | unit/verify | revert, verified |

### [`RFC1195-3.4-2`](#rfc1195-3.4-2)

The inter- domain routing protocol information field is not included in pseudonode LSPs. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-30. Positive: TestRFC1195PseudonodeCannotInheritRouterPrefixes decodes every pseudonode TLV and fails on type 131. {single-polarity: positive} checked: packet/tlv.go has no TLV 131 constant and no Go producer writes type 131, so no input can drive a producer toward placing it and no refusal path exists; no HEAD negative on the row. Level MUST NOT over 'is not included in pseudonode LSPs' (no keyword) is a D-3 call, accepted. Record: revert on OriginatePseudonode, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L449) | unit/verify | revert, verified |

### [`RFC1195-5.2-4`](#rfc1195-5.2-4)

Each entry must contain a default metric (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The TLV 135 tag on packet TestISISTLVIPv4RoundTrip (RFC 5305 extended reachability, not an RFC 1195 entry) is gone, which was the only reason for weak. Positive TestRFC1195OriginatedExternalEntriesCarryDefaultMetric decodes every TLV 130 entry the Originator emits and requires the configured default metric (7 internal type, 33 external type), so an originator that dropped or zeroed it goes red. Negative TestRFC1195ExternalEntryWithoutDefaultMetricRefused removes the default metric octet from a TLV 130 entry: requireNoDefaultMetricRefused requires ErrLength and the route falls back to the competing entry (winner 2, metric 160). Both recorded red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalEntryWithoutDefaultMetricRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC1195OriginatedExternalEntriesCarryDefaultMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L82) | unit/verify | revert, verified |

### [`RFC1195-3.2-1`](#rfc1195-3.2-1)

Note: If this sum results in a metric value greater than 63 (the maximum value that can be reported in level 2 LSPs), then the value 63 must be used. (Section 3.2; wide metrics use RFC 5305's separate bound)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: L1 path 50+40 leaked to L2 with metric 63. Negative: 10+5 preserved as 15. Both fail if the clamp is removed or applied wrongly.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NarrowLeakedMetricBelowCeiling`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L319) | unit/verify | unproven |
| positive | [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L310) | unit/verify | unproven |

### [`RFC1195-3.9-1`](#rfc1195-3.9-1)

If a packet is received which contains invalid authentication information, then the entire packet is discarded. (§3.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. New root TestRFC1195InvalidAuthLSPDiscardedAtReceive drives the real dispatcher with the engine's verify hook (setKeyStore, HMAC-SHA-256 area chain): the signed L1 LSP reaches the L1 LSP handler exactly once (positive), and the same LSP with one digest octet (offset 32, past the 2-octet Key ID) flipped never reaches it (negative), so the entire PDU is dropped before any LSDB, adjacency or SNP handler. Only the digest differs, so the buffer isolates the auth rule. A receive path that verified and ignored the result goes red; +/- recorded red on auth_wiring.go verifyFrame. Packet units (VerifyPDU error on a flipped bit, accept on valid) remain as the codec-level half.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611) | unit/verify | revert, verified |
| positive | [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134) | unit/verify | revert, verified |

### [`RFC1195-3.1-1`](#rfc1195-3.1-1)

Any codes in a received PDU that are not recognised shall be ignored and, for those packets which are forwarded (specifically Link State Packets), passed on unchanged. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 'ignored': packet TestISISUnknownTLVPassthrough decodes TLVs 199/250 with no error and re-encodes the region byte-for-byte (prose now cites sec 5.2, matching the row). Clause 'forwarded LSPs passed on unchanged': new lsdb TestRFC1195UnknownTLVsFloodedUnchanged stores an LSP carrying 131/199/250 via Flooder.ReceiveLSP and after FloodTick requires the LAN circuit to send bytes.Equal to the received raw PDU, each unknown TLV present, and nothing back on the arrival circuit; a flood that rebuilt or stripped the LSP goes red. Both recorded red (ReceiveLSP, DecodeTLVs). {single-polarity: positive} holds: there is no reject path for an unrecognized code.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1195UnknownTLVsFloodedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/unknown_tlv_flood_rfc1195_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16) | unit/verify | revert, verified |

### [`RFC1195-1.4-1`](#rfc1195-1.4-1)

Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-1.4-1, so no unit is bound to it.

### [`RFC1195-1.4-3`](#rfc1195-1.4-3)

In a dual area within a dual routing domain only dual routers may be used. (§1.4)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Judged 2026-09-30. Level MUST over 'only dual routers may be used' kept under D-3; {not-applicable} accepted (binds the network design, same as 1.4-1).

No test carries RFC1195-1.4-3, so no unit is bound to it.

### [`RFC1195-1.2-1`](#rfc1195-1.2-1)

External links (to other routing domains) must be from level 2 routers (Section 1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-1.2-1, so no unit is bound to it.

### [`RFC1195-1.4-2`](#rfc1195-1.4-2)

In a pure IP routing domain, all routers must be IP-capable (Section 1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195IPv6SelectionDoesNotRemoveIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L204) | unit/verify | unproven |
| positive | [`TestRFC1195PureIPAdvertisesIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L191) | unit/verify | unproven |

### [`RFC1195-3.2-2`](#rfc1195-3.2-2)

In general, the same [IP address, subnet mask] pair may be announced in level 1 LSPs sent by multiple level 1 routers in the same area. In this case (assuming the entry is not superceded by a manually configured entry), then only one such entry shall be included in the level 2 LSP. The metric value(s) announced in level 2 LSPs correspond to the minimum of the metric value(s) that would be calculated for each of the level 1 LSP entries. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: more than one L2 entry for a prefix several L1 routers announce, or an L2 metric above the minimum. metricLeak fails unless exactly one L2 entry for the prefix is originated; TestRFC1195SpecificLeakCollapsesDuplicatePrefix (two routers, 20 and 9) and TestRFC1195SpecificLeakKeepsOnlyMinimum (three routers, 20, 12 and 9, cheapest not first in system-id order) each require metric 9. Distinct inputs, so no longer one assertion with two tags; a first-seen or last-seen leak producer fails the three-router case. No refusal path exists for an originator. Negative carries an observed-red record on leakInto; the positive has none but reads red on the same breaks.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195SpecificLeakKeepsOnlyMinimum`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L435) | unit/verify | revert, verified |

### [`RFC1195-3.2-3`](#rfc1195-3.2-3)

If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-3.2-3, so no unit is bound to it.

### [`RFC1195-3.3-1`](#rfc1195-3.3-1)

The reserved field must contain "00 00", as specified in GOSIP version 2.0. (Section 3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Encoder emits the GOSIP NET with reserved octets zero; ParseNET and DecodeISH refuse either nonzero reserved octet with ErrNETReserved.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHGOSIPReservedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC1195ISHGOSIPReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L15) | unit/verify | unproven |

### [`RFC1195-3.3-2`](#rfc1195-3.3-2)

IEEE 802 addresses, if used, must appear in IEEE canonical format (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHDoesNotReverseIdentifierBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L84) | unit/verify | unproven |
| positive | [`TestRFC1195ISHCanonicalIEEEIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_ish_test.go#L60) | unit/verify | unproven |

### [`RFC1195-3.4-1`](#rfc1195-3.4-1)

When an external route using external metrics must be used, the lowest value of the external metric is preferred regardless of the internal cost to reach the appropriate exit point (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalMetricWithdrawalRecomputesWinner`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC1195ExternalMetricBeforeExitCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L148) | unit/verify | unproven |

### [`RFC1195-3.10-1`](#rfc1195-3.10-1)

The Dijkstra calculation must calculate routes to each distinct IP reachability entry (Section 3.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NoRouteForUnreachedReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L29) | unit/verify | revert, verified |

### [`RFC1195-3.10-2`](#rfc1195-3.10-2)

If level 2 routes must be used, then routes within the routing domain (specifically, those routes using internal metrics) are prefered to routes outside of the routing domain (using external metrics). (Section 3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after a comment-only edit above TestRFC1195ExternalReachabilityWithInternalMetric (its RFC1195-7-4 tag moved away); the body is unchanged. A costly internal-metric L2 prefix (163) beats a cheap external-metric one in the installed Loc-RIB; an external TLV with an internal metric is not penalized (3, 2). Both fail if metric type is ignored.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L169) | unit/verify | unproven |

### [`RFC1195-3.10-3`](#rfc1195-3.10-3)

However, the default metric must always be available. (Section 3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a reachability entry used with no default metric. The default metric octet has no S (unsupported) bit in the 12-octet TLV 128/130 layout, so an entry lacking it can only arrive as a short entry, and the length refusal is this rule's own refusal. TestRFC1195InternalEntryWithoutDefaultMetricRefused asserts DecodeNarrowIPReachTLV returns ErrLength for the entry with its first octet removed while the complete entry decodes with metric 9, and the full LSDB/SPF path keeps the competing route (3 then 2,160); a floor(len/12) decoder goes red. Positive TestRFC1195RouteForEachReachabilityEntry: SPF uses each entry's own default metric. Both units carry observed-red revert records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InternalEntryWithoutDefaultMetricRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L32) | unit/verify | revert, verified |

### [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1)

3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-3.10.2-1, so no unit is bound to it.

### [`RFC1195-4.1-1`](#rfc1195-4.1-1)

This implies that all IS-IS routers, including IP-only routers, must be able to receive IS-IS packets using the normal encapsulation for OSI packets. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) under OWNER RULING 2, which names this row: an accept-on-reception obligation is proven by acceptance (positive) and by how Ze handles the absent or wrong form (negative), and 'refusing non-OSI encapsulation is that negative'. Positive transport TestRFC1195ParseFrameOSIEncapsulation: an 802.3 length + LLC FE/FE/03 frame is accepted and the PDU and source MAC come back intact. Negative TestRFC1195ParseFrameRefusesNonOSIEncapsulation (prose reworded, D-15; body unchanged): an Ethernet II ethertype yields ErrNotISO and a SNAP LLC yields ErrBadLLC, each with a nil PDU, so a frame lacking the normal OSI encapsulation is never read as an IS-IS PDU. Read against transport/frame.go ParseFrame. Records +/- revert ParseFrame, both observed red. The earlier weak verdict called the refusal a neighbouring strictness property; the owner ruled it is this row's negative.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ParseFrameRefusesNonOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_frame_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC1195ParseFrameOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_frame_test.go#L33) | unit/verify | revert, verified |

### [`RFC1195-4.2-1`](#rfc1195-4.2-1)

However, IP-capable routers must be able to interact correctly with other routers which assign multiple IP addresses per physical interface (up to the maximum of 63 addresses per interface). (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. New root units drive the engine path (dispatched P2P IIH -> adjacency table -> engineNextHopResolver) with a fake iface backend. Positive TestRFC1195MultiAddressNeighborForwarding: a neighbor listing the TLV 132 maximum of 63 addresses forms an Up adjacency and resolves to its own first address, on-link via the arrival interface, so a receiver that refused, truncated or mis-read a 63-address list goes red. Negative TestRFC1195MultiAddressNeighborRenumbered: the same neighbor re-sends a different 63-address list and no address of the old list survives as the next hop; forwarding to an address the neighbor no longer assigns (sec 4.2: each Hello carries the interface's current list) is an incorrect interaction and is rejected. Producer circuit/runtime.go helloInput takes the first decoded TLV 132 address per Hello; +/- recorded on it. The older packet units (63-address decode, truncated TLV refused) remain as codec cover.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InterfaceAddrTLVTruncatedAddressRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_ipv4_test.go#L54) | unit/verify | revert, verified |
| negative | [`TestRFC1195MultiAddressNeighborRenumbered`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC1195InterfaceAddrTLV63Addresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_ipv4_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC1195MultiAddressNeighborForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L124) | unit/verify | revert, verified |

### [`RFC1195-4.4-1`](#rfc1195-4.4-1)

IP-capable IS-IS routers therefore must be able to forward IP packets over existing adjacencies to routers with which they share physical connectivity, even when the IP address of the adjacent interface of the neighboring router is on a different logical IP subnet. (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. The earlier gap (nothing showed IS-IS itself marks an off-subnet adjacency next hop on-link) is closed at the boundary Ze owns. Positive TestRFC1195OffSubnetNeighborIsOnLinkNextHop: adjacency on isis-on-a 192.0.2.1/24, neighbor 198.51.100.2 (setup asserts off-subnet) resolves to 198.51.100.2 via isis-on-a with OnLink. Negative TestRFC1195OffSubnetNeighborNotResolvedBySubnet: isis-on-b holds the covering 198.51.100.1/24 but no adjacency, and the next hop is never placed on isis-on-b by subnet match; a subnet or recursive resolver goes red. Producer spf_wiring.go ResolveNextHop returns the Up adjacency's circuit with OnLink=true; +/- recorded on it. The FIB side (kernel installs the OnLink gateway, flips when the mark is cleared) is carried by fib/kernel onlink_integration_linux_test.go, Linux CAP_NET_ADMIN only and without a discrimination record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestRFC1195OffSubnetNeighborNotResolvedBySubnet`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L190) | unit/verify | revert, verified |
| positive | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC1195OffSubnetNeighborIsOnLinkNextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_neighbor_addresses_test.go#L168) | unit/verify | revert, verified |

### [`RFC1195-4.4-2`](#rfc1195-4.4-2)

All IS-IS routers are therefore required to transmit and receive ISO 9542 ISH packets on point-to-point links. (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) under OWNER RULING 2 (receive row: the negative is Ze's handling of a missing ISH). RFC 1195 section 4.4: 'For point-to-point links, IS-IS requires exchange of ISO 9542 ISHs, as the first step in establishing the link between routers.' Positive: root TestRFC1195ISHTransmitReceive requires an ISH to AllESs, with NET and 0xCC, before the first IIH, and a received ISH through the production dispatcher to initialize the peer; transport TestRFC1195ISHVethTransport sends and receives it over AF_PACKET on a veth pair. Negative, NEW root TestRFC1195NeighborWithoutISHStillFormsAdjacency: a P2P neighbor that never sends an ISH is not refused; its IIH three-way Down makes an Initializing adjacency and its IIH three-way Initializing naming Ze brings it Up. The ISH is the classification step, the IIH exchange makes the adjacency, so waiting forever on an ISH the peer may not send is the failure this pins (author's overlay making applyHello refuse an ISH-less Down adjacency went red, state down). Held negatives (corrupt ISH and wrong circuit, wrong or missing ISH password) prove neighbouring rules and stay supplementary. Records: - revert applyHello (new unit); this judge recorded + revert sendISH (TestRFC1195ISHTransmitReceive), - revert receiveISH (corruption/wrong circuit), + revert signISHPDU and - revert verifyISHFrame (password units), all observed red. NOT YET RECORDED: TestRFC1195ISHVethTransport (+ revert transport.go SendISH), a guest proof; the guest le build failed on another session's uncommitted internal/component/l2tp/session_fsm.go handleSLI signature change, so that one record is owed when the tree builds. The ISO 10589 immediate-IIH-on-ISH reply is journaled (plan/journal/queued-request-waits-for-an-unrelated-trigger.md), not this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHRejectsCorruptionAndWrongCircuit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L324) | unit/verify | revert, verified |
| negative | [`TestRFC1195NeighborWithoutISHStillFormsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_absence_test.go#L43) | unit/verify | revert, verified |
| negative | [`TestRFC1195ISHWrongPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_auth_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L239) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHConfiguredPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_auth_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHVethTransport`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/rfc1195_ish_integration_linux_test.go#L19) | unit/verify | revert, verified |

### [`RFC1195-4.4-3`](#rfc1195-4.4-3)

Thus, the value of the "protocols supported" field must be identical on every link (i.e., for any one router running IS-IS, all of the Hellos and LSPs transmitted by it must contain the same "protocols supported" values). (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Enabling IPv6 on one interface gives identical NLPIDs in both IIHs and the LSP; after reload no stale IPv6 capability remains on the unchanged circuit. Re-judged 2026-09-30 (twice): TestRFC1195NodeProtocolsAcrossInterfaces changed only by added RFC1195-5.2-1 and RFC3787-x-2 tag comments.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NodeProtocolsReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L171) | unit/verify | unproven |
| positive | [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L135) | unit/verify | unproven |

### [`RFC1195-4.5-1`](#rfc1195-4.5-1)

There may be times when a dual router has to forward an IP packet to an OSI-only router, or forward an OSI packet to an IP-only router. In this case the packet must be discarded. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IP half: TestRFC1195OSINeighborIsTerminalIPv4NextHop fails if an OSI-only neighbor resolves to a usable IPv4 next hop; TestRFC1195CapabilityChangesInstalledRouteDisposition fails unless the installed route becomes Unreachable (production LSDB/SPF/installer) and returns to unicast when IPv4 support returns; the FIB integration test fails unless the kernel answers EHOSTUNREACH instead of the default and restores it on removal. OSI half: Ze forwards no OSI/CLNP packets, so no code path can violate it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L39) | unit/verify | unproven |
| negative | [`TestRFC1195NeighborProtocolTransitions`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_adjacency_protocol_test.go#L56) | unit/verify | unproven |
| negative | [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L496) | unit/verify | unproven |
| positive | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc1195_onlink_integration_linux_test.go#L38) | unit/verify | unproven |
| positive | [`TestRFC1195OSINeighborIsTerminalIPv4NextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_adjacency_protocol_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L495) | unit/verify | unproven |

### [`RFC1195-5.3.4-3`](#rfc1195-5.3.4-3)

Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. (Section 5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row corrected (rfc/corrections/rfc1195.md) to the Section 5.3.4 bit-7 sentence of the DELAY/EXPENSE/ERROR octets, verbatim at rfc1195.txt line 2365; default-metric bit 8 is RFC 2966 sec 2's up/down bit (extraction site 5.3.4:3 excluded cross-document). Transmit: TestRFC1195NarrowMetricReservedTransmit encodes 0xff/0x6a/0xc7 and pins 0xbf/0x2a/0x87, so bit 7 (0x40) is cleared while S and the six-bit value survive; an encoder that passed bit 7 through goes red. Receive: TestRFC1195NarrowMetricReservedReceive sets 0x40 on all three octets and requires the decoded entry equal the clean one (63, 0x80, 7), so a decoder that kept bit 7 goes red. Recorded red on WriteTo and DecodeNarrowIPReachTLV.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NarrowMetricReservedReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_narrow_ipv4_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC1195NarrowMetricReservedTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc1195_tlv_narrow_ipv4_test.go#L14) | unit/verify | revert, verified |

### [`RFC1195-7-1`](#rfc1195-7-1)

The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). (§B.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §B.1, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: SNP entries in any order but ascending LSPID with the LSP number least significant. TestRFC1195CSNPEntriesAscending asserts the exact order [20.0, 20.1, 30.0, 40.0] from reverse inserts; TestRFC1195PSNPEntriesAscendingAcrossLists asserts [request, ack-only, ack] where list order would be (30, 10, 20). Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PSNPEntriesAscendingAcrossLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_snp_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC1195CSNPEntriesAscending`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/rfc1195_snp_test.go#L31) | unit/verify | revert, verified |

### [`RFC1195-7-2`](#rfc1195-7-2)

However, a full update must be done periodically to ensure recovery from data corruption, and studies suggest that with a very small number of link changes (perhaps 2) the expected computation complexity of the incremental update exceeds the complete recalculation. (§C.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Obligation: a periodic full update recovers what incremental processing missed. TestRFC1195PeriodicSPFRecoversMissedUpdate: a foreign metric change stored with no trigger leaves the route stale at 11, and the aging timer's refresh repairs it to 15. TestRFC1195PeriodicSPFRemovesMissedWithdrawal: a silently withdrawn prefix stays installed (asserted) until ageOnce's periodic pass removes it. Two separate inputs (changed metric, withdrawal); both go red if the periodic pass stops running SPF. The negative is recorded red on refreshOwnLSPs. The 'studies suggest' clause is descriptive and carries no obligation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PeriodicSPFRemovesMissedWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L172) | unit/verify | revert, verified |
| positive | [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L331) | unit/verify | revert, verified |

### [`RFC1195-7-3`](#rfc1195-7-3)

The 8 octet system identifiers which specify IP reachability entries must always be distinguishable from other system identifiers (§C.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1.1, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: an IP reachability entry whose identifier collides with a router's system identifier. TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix gives router B a System ID spelling 10.4.0.0/24 and asserts 3 settled router vertices, B at metric 10, C at 15 through B, and one prefix route at 11: a merged identifier space changes the vertex count. TestRFC1195PrefixCannotBridgeDisconnectedRouters asserts a lookalike prefix does not make a disconnected router's 198.51.100.0/24 reachable. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L93) | unit/verify | revert, verified |

### [`RFC1195-7-4`](#rfc1195-7-4)

IP-capable level 2 routers must keep level 2 internal IP routes separate from level 2 external IP routes (§C.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: L2 internal and external routes compared as one set. TestRFC1195InternalMetricOutranksExternal: the internal route at cost 163 beats an external-metric route at cost 2, so merging the sets by cost goes red. TestRFC1195ExternalSetRetainedBesideInternal: after the internal route is withdrawn the external set is still there and installs at its external metric 1, not the internal sum 2, so discarding or folding the external set into internal-metric arithmetic goes red. Negative recorded red on spf/route.go better. The old negative (external TLV with internal metric) no longer carries this tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalSetRetainedBesideInternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_selection_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L170) | unit/verify | revert, verified |

### [`RFC1195-7-5`](#rfc1195-7-5)

Each entry made to TENT must be marked as being either an End System or a router (§C.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1.4, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: a TENT entry not marked end system versus router, so an end system (a reachability entry) is used as a transit router. TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix asserts the settled set holds 3 routers only and the entry is reached through its advertising router; TestRFC1195PrefixCannotBridgeDisconnectedRouters asserts a prefix leaf does not bridge to a disconnected router's prefix. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L143) | unit/verify | unproven |
| positive | [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/rfc1195_route_test.go#L97) | unit/verify | revert, verified |

### [`RFC1195-7-6`](#rfc1195-7-6)

The password shall be configured on a per-link, per-area, and per- domain basis. (§D.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §D.2, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: one password shared across link, area and domain scopes. TestRFC1195PasswordScopesResolved asserts three distinct chains iih/area/domain with distinct key ids; TestRFC1195PasswordScopesDoNotLeak asserts eth1 resolves no eth0 link chain at either level and the L2 chain is not the L1 chain. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PasswordScopesDoNotLeak`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC1195PasswordScopesResolved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L49) | unit/verify | revert, verified |

### [`RFC1195-7-7`](#rfc1195-7-7)

Specifically, when this form of authentication is used: - IS-IS Hello and 9542 IS Hello packets shall contain the per-link password - Level 1 Link State Packets shall contain the per-area password - Level 2 Link State Packets shall contain the per-domain password - Level 1 Sequence Number Packets shall contain the per-area password - Level 2 Sequence Number Packets shall contain the per-domain password (§D.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Every PDU class of the row now has both polarities. IS-IS Hello, L1/L2 LSP and L1/L2 CSNP/PSNP: auth_rfc1195_test.go TestRFC1195PDUClassSignedWithItsScope and TestRFC1195PDUClassRefusedUnderOtherScopes (recorded on signLevelPDU). ISO 9542 IS Hello, the clause the earlier verdict found missing: TestRFC1195ISHCarriesPerLinkPassword configures distinct link, area and domain cleartext passwords, requires the transmitted ISH to verify with the link password and with neither other, and a link-password peer ISH to initialize; TestRFC1195ISHRefusesAreaAndDomainPasswords requires a peer ISH carrying the area or domain password to create no adjacency state. Recorded red on signISHPDU and verifyISHFrame.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PDUClassRefusedUnderOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L132) | unit/verify | revert, verified |
| negative | [`TestRFC1195ISHRefusesAreaAndDomainPasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_scope_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC1195PDUClassSignedWithItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC1195ISHCarriesPerLinkPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_ish_scope_test.go#L36) | unit/verify | revert, verified |

### [`RFC1195-7-8`](#rfc1195-7-8)

Also, each of these three passwords shall be configured with: (i) "Transmit Password", whose value is a single password, and (ii) "Receive Passwords", whose value is a set of passwords. (§D.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §D.2, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: several transmit passwords, or a receive set of one. TestRFC1195OneTransmitPasswordManyReceivePasswords asserts sign key 1 only and a receive set of two that verifies both; TestRFC1195KeyOutsideReceiveSetRefused asserts a key outside the set is refused and the transmit key alone does not verify a key-2 PDU. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195KeyOutsideReceiveSetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC1195OneTransmitPasswordManyReceivePasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_auth_test.go#L204) | unit/verify | revert, verified |

### [`RFC1195-8-1`](#rfc1195-8-1)

Type = 2 indicates that the external information field contains an autonomous system number tag, to be applied to subsequent IP external reachability information entries. In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. The AS tag is associated with subsequent IP External Reachability entries, until the end of the LSP, or until the next occurence of the Inter-Domain Routing Protocol Information field. (§A.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-8-1, so no unit is bound to it.

### [`RFC1195-8-2`](#rfc1195-8-2)

Type = 0 is reserved (must not be sent, and must be ignored on receipt). (Section A.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-8-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc1195.txt |
| Source fingerprint | 4adf2025be11a7dd |
| Record | rfc/extraction/rfc1195.json |
| Mapped sentences | 42 |
| Declined as scope | 66 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 1 | walked | not stated |
| `1.2` | not stated | 2 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 3 | walked | not stated |
| `1.5` | not stated | 4 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `3.3` | not stated | 9 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 1 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `3.8` | not stated | 1 | walked | not stated |
| `3.9` | not stated | 1 | walked | not stated |
| `3.10` | not stated | 3 | walked | not stated |
| `3.10.1` | not stated | 0 | walked | not stated |
| `3.10.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 2 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 7 | walked | not stated |
| `4.5` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 6 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 0 | walked | not stated |
| `5.3.2` | not stated | 0 | walked | not stated |
| `5.3.3` | not stated | 0 | walked | not stated |
| `5.3.4` | not stated | 10 | walked | not stated |
| `5.3.5` | not stated | 19 | walked | not stated |
| `5.3.6` | not stated | 0 | walked | not stated |
| `5.3.7` | not stated | 0 | walked | not stated |
| `5.3.8` | not stated | 0 | walked | not stated |
| `5.3.9` | not stated | 0 | walked | not stated |
| `5.3.10` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `A.1` | Annex subsection A.1 | 1 | walked | Annex subsection A.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `A.2` | Annex subsection A.2 | 2 | walked | Annex subsection A.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B.1` | Annex subsection B.1 | 2 | walked | Annex subsection B.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B.2` | Annex subsection B.2 | 2 | walked | Annex subsection B.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B.3` | Annex subsection B.3 | 2 | walked | Annex subsection B.3. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B.4` | Annex subsection B.4 | 2 | walked | Annex subsection B.4. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 2 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C.1` | Annex subsection C.1 | 1 | walked | Annex subsection C.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C.1.1` | Annex subsection C.1.1 | 1 | walked | Annex subsection C.1.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C.1.2` | Annex subsection C.1.2 | 0 | walked | Annex subsection C.1.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `C.1.3` | Annex subsection C.1.3 | 0 | walked | Annex subsection C.1.3. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `C.1.4` | Annex subsection C.1.4 | 6 | walked | Annex subsection C.1.4. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 6 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C.2` | Annex subsection C.2 | 0 | walked | Annex subsection C.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `C.2.1` | Annex subsection C.2.1 | 0 | walked | Annex subsection C.2.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `C.2.2` | Annex subsection C.2.2 | 0 | walked | Annex subsection C.2.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `D.1` | Annex subsection D.1 | 0 | walked | Annex subsection D.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `D.2` | Annex subsection D.2 | 7 | walked | Annex subsection D.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of section 8, where its 7 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `E.1` | Annex subsection E.1 | 0 | walked | Annex subsection E.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `E.2` | Annex subsection E.2 | 0 | walked | Annex subsection E.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `E.2.1` | Annex subsection E.2.1 | 0 | walked | Annex subsection E.2.1. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |
| `E.2.2` | Annex subsection E.2.2 | 0 | walked | Annex subsection E.2.2. Until 2026-09-27 the heading reader did not read a column-0 annex subsection heading written without a trailing dot, so its text was read as part of the section before it. It carries no site, so no decision moved. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | descriptive scoping sentence about what this RFC documents; 'required additional features' names the document's contents, not an obligation on an implementation | This RFC is considered a companion to the OSI IS-IS Routing spec, and will only describe the required additional features. |
| `1.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the obligation is the Requirements for Internet Gateways document the sentence cites as reference [4]; RFC 1195 only points at it | IP-only and dual routers are required to conform to the requirements of Internet Gateways [4]. |
| `1.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the keyword sits inside a statement that the standard does NOT specify the behaviour: 'Since the standard does not specify that the end system MUST autoconfigure its area address'. The sentence grants a permission (an end system may be configured) rather than imposing an obligation | Since the standard does not specify that the end system MUST autoconfigure its area address, an end system may be configured with an area address. |
| `1.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the pure-OSI mirror of the domain-capability rule already carried by the dual/OSI row; ze runs pure-IP Integrated IS-IS and routes no CLNP | In a pure OSI routing domain, all routers must be OSI-capable. |
| `1.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a sentence comparing the integrated approach with ships-in-the-night; 'required' describes network management effort, not an implementation obligation | A primary advantage of the integrated IS-IS relates to the network management effort required. |
| `1.5:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | descriptive observation that two independent protocols share resources; it imposes nothing on an implementation | Note that the operation of two routing protocols with the S.I.N. approach are not really independent, since they must share common resources. |
| `1.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same descriptive sentence as site 1.5:2, cut twice by the splitter; neither carries an obligation | Note that the operation of two routing protocols with the S.I.N. approach are not really independent, since they must share common resources. |
| `1.5:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a general observation about real time guarantees in routing protocols, with no addressee and no protocol behaviour | However, in all routing protocols, there are real time guarantees which must be met in order to ensure correct operation. |
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a parenthetical rationale for carrying the IP Interface Address field; the ICMP redirect obligation it describes belongs to the IP host and router requirements, and the RFC 1195 obligation it motivates is at site 3.1:2 | This is required for sending ICMP redirects (when an IP-capable router sends an ICMP redirect to a host, it must include the IP address of the appropriate interface of the correct next-hop router). |
| `3.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | figure annotation for the NSAP layout; the prose sentence carrying the same obligation is site 3.3:4 | Reserved 2 octets must be "00 00" |
| `3.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | figure annotation pointing at the assignment prose below it, which sites 3.3:5 to 3.3:7 carry | Area 2 octets must be assigned as described below |
| `3.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | figure annotation pointing at the assignment prose below it, which sites 3.3:6 and 3.3:7 carry | ID 6 octets must be assigned as described below |
| `3.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence addresses the authority responsible for the routing domain, the numbering role that allocates area values; ze consumes the allocated NET rather than assigning it The role has no producer in this tree: internal/plugins/isis/types/net.go parses the NET the operator supplies and ze assigns no area value or system ID of its own. | The Area field must be assigned by the authority responsible for the routing domain, such that each area in the routing domain must have a unique Area value. |
| `3.3:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence addresses the authority responsible for the routing domain, which assigns system IDs; ze consumes the configured NET rather than assigning it The role has no producer in this tree: internal/plugins/isis/types/net.go parses the NET the operator supplies and ze assigns no area value or system ID of its own. | The ID must be assigned by the authority responsible for the routing domain. |
| `3.3:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | uniqueness of the assigned system ID across the routing domain is the allocating authority's obligation; ze cannot observe the whole domain, it only parses and uses the NET it was given The role has no producer in this tree: internal/plugins/isis/types/net.go parses the NET the operator supplies and ze assigns no area value or system ID of its own. | The ID must be assigned such that every router in the routing domain has a unique value. |
| `3.3:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | a constraint on the operator's addressing plan: every router in one area must be configured with the same high order address part. ze checks area match when forming a Level 1 adjacency but does not assign the addresses The role has no producer in this tree: internal/plugins/isis/types/net.go parses the NET the operator supplies and ze assigns no area value or system ID of its own. | Note that within an area, whether ISO addresses are configured into the routers through ISO address assignment, or whether the ISO-style address is generated directly from the AS number and IP address, all routers within an area must have the same high order part of address (AFI, ICD, DFI, AA, RD, and Area). |
| `3.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | indicative prose ('may be required to occur in LSP number 0') introducing the per-field rules; the obligations themselves are stated at the fields, sites 5.3.4:1 and 5.3.5:1 | Some specific variable length fields may be required to occur in LSP number 0. |
| `3.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence removes an obligation rather than imposing one: routers are NOT required to implement encapsulation or decapsulation | Routers complying with the Integrated IS-IS are not required to implement encapsulation nor decapsulation. |
| `3.9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence removes an obligation: routers are NOT required to interpret authentication information. The obligation section 3.9 does impose, discarding a packet whose authentication is invalid, is the row RFC1195-3.9-1 | Routers are not required to be able to interpret authentication information. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence binds whoever defines a link type, and the RFC says so in the next sentence: 'Definition of such methods for common link types is outside of the scope of this specification' | For any particular link type, a method must be defined for encapsulation of both OSI and IP packets. |
| `4.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates for ISO 9542 ISHs the Protocols Supported obligation the row already carries, as section 5.2 states it again at site 5.2:2 | Similarly, those 9542 ISHs sent over point-to-point links, where there is (or may be) another IS-IS router at the other end of the point-to-point link, must also contains the "protocols supported" field. |
| `4.4:5` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence attributes the obligation to ISO 9542 itself ('in accordance to ISO 9542, the End System is required to ignore the field'); it binds an OSI End System under that standard, not an IS-IS router under RFC 1195 | Note that if this field is mistakenly sent in a 9542 ISH where there is an ordinary OSI-only End System at the other end of the link, then (in accordance to ISO 9542) the End System is required to ignore the field and interpret the ISH correctly. |
| `4.4:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the dual-router operating constraint the dual-domain row already carries; ze runs pure-IP Integrated IS-IS and is not a dual router | Dual routers must operate in a dual fashion on every link in the routing domain over which they are running IS-IS. |
| `4.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sentence says so itself: 'Again, the packet must be discarded, as specified above', restating site 4.5:1 for the IP-only router case | Again, the packet must be discarded, as specified above. |
| `5.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the ISO 9542 ISH obligation on point-to-point links that section 4.4 states at site 4.4:2 | All routers implementing IS-IS (whether IP-only, OSI-only, or dual), if they have any interfaces on point-to-point links, must therefore be able to transmit ISO 9542 ISHs on their point-to-point links. |
| `5.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ISO 9542 ISH clause of the same Protocols Supported obligation, cut into its own sentence by the splitter | The "Protocols Supported" field must also be included in ISO 9542 ISHs send by IP-capable routers over point-to-point links to other IS-IS routers. |
| `5.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same default-metric sentence, written once for IP Internal Reachability and once for IP External Reachability | Each entry must contain a default metric, and may contain delay, expense, and error metrics. |
| `5.3.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the field description of CODE 129 restating that Protocols Supported appears once in LSP number 0, which the row already carries | This must appear once in LSP number 0. x CODE - 129 |
| `5.3.4:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 2966 section 2, which updates RFC 1195, reassigns bit 8 of the default metric of TLVs 128 and 130 to the up/down bit, so the obligation on that bit is RFC 2966's (RFC2966-2-1): 'L1L2 routers must set this bit to one for prefixes that are derived from L2 routing and are advertised into L1 LSPs.' | Bit 8 of this field is reserved, and must be set to zero on tranmission and ignored on reception. |
| `5.3.4:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the S bit reports an unsupported TOS metric, and RFC 1195 section 3.5 makes the feature optional: 'The support for TOS/QOS is optional.' ze offers only the default metric and encodes the RFC 5305 wide metric (internal/plugins/isis/types/metric.go), so it advertises no delay, expense or error metric at all | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.4:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for the expense metric octet; the TOS feature it belongs to is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.4:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for the expense metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.4:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for the error metric octet; the TOS feature it belongs to is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.4:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for the error metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the field description restating that the field appears once in LSP number 0, the obligation section 5.2 carries for Protocols Supported | This must appear once in LSP number 0. |
| `5.3.5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the bit 8 reserved rule of the default metric octet, repeated in the external reachability field description | Bit 8 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the I/E bit rule repeated in the section 5.3.5 field descriptions | Bit 7 of this field indicates the metric type (internal or external) for all four TOS metrics, and must be set to zero indicating internal metrics. |
| `5.3.5:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for a TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for a TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for a TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the bit 8 reserved rule repeated in a second metric octet description | Bit 8 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for a TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for a TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `5.3.5:18` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the unsupported-metric S bit rule repeated for a TOS metric octet; the TOS feature is out of scope as recorded at site 5.3.4:5 | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.5:19` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for the last TOS metric octet | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
| `A.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the single 2 octet AS number rule for the same Inter-Domain Information Type | In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. |
| `B.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule for option fields, already carried by the Sequence Number PDU row | The option fields, if they appear more than once, shall appear sorted into ascending LSPID order. |
| `B.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule, repeated for the option fields of the same Sequence Number PDU description | The option fields, if they appear more than once, shall appear sorted into ascending LSPID order. |
| `B.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule repeated for the next Sequence Number PDU type | The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). |
| `B.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule repeated for the option fields of that PDU type | The option fields, if they appear more than once, shall appear sorted into ascending LSPID order. |
| `B.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule repeated for the next Sequence Number PDU type | The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). |
| `B.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule repeated for the option fields of that PDU type | The option fields, if they appear more than once, shall appear sorted into ascending LSPID order. |
| `B.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ascending LSPID ordering rule repeated for the last Sequence Number PDU type | The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). |
| `C.1.4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | running the Decision Process once per supported routing metric is an obligation conditional on supporting more than the default metric, and RFC 1195 section 3.5 makes that optional: 'The support for TOS/QOS is optional.' ze supports the default metric only (internal/plugins/isis/types/metric.go), so the algorithm runs once | The Decision Process Algorithm must be run once for each supported routing metric (i.e., for each supported Type of Service). |
| `C.1.4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a note deriving a consequence ('Note that this implies that routers ... must run the SPF algorithm 8 times') from the rule at site 7:10; it adds no obligation of its own | Note that this implies that routers which are both level 1 and level 2 routers, and which support all four routing metrics, must run the SPF algorithm 8 times (assuming partition repair is not implemented). |
| `C.1.4:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the sentence names the feature as optional itself: 'If this system is a Level 2 Router which supports the partition repair optional function the Decision Process algorithm for computing Level 1 paths must be run twice for the default metric.' ze implements no partition repair: internal/plugins/isis holds no partition-repair path, only the L1/L2 decision process | If this system is a Level 2 Router which supports the partition repair optional function the Decision Process algorithm for computing Level 1 paths must be run twice for the default metric. |
| `C.1.4:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a mathematical note about the Dijkstra invariant ('d(N) must be less than dist(P,N), or else N would not have been put into PATHS'), not an obligation on an implementation | Note: d(N) must be less than dist(P,N), or else N would not have been put into PATHS. |
| `D.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the same per-PDU password list, for Level 1 Link State Packets | - Level 1 Link State Packets shall contain the per-area password |
| `D.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the same per-PDU password list, for Level 2 Link State Packets | - Level 2 Link State Packets shall contain the per-domain password |
| `D.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the same per-PDU password list, for Level 1 Sequence Number Packets | - Level 1 Sequence Number Packets shall contain the per-area password |
| `D.2:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the same per-PDU password list, for Level 2 Sequence Number Packets | - Level 2 Sequence Number Packets shall contain the per-domain password |

## Superseded

No document obsoletes RFC 1195, so its obligations are stated where they were written.
