# RFC 1195 - Use of OSI IS-IS for Routing in TCP/IP and Dual Environments

Experimental. Every requirement this repository extracted from RFC 1195, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 73.7% | 28 of 38 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.5% | 4 of 38 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 38 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 38 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 25.0% | 17 of 68 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 38 | of 39 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 38 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 15.8% | 6 of 38 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 38 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 38 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 28 | of 38 gated MUSTs judged | 16 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 38 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 39 |
| Gated MUST-level | 38 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 68 |
| Tagged units | 68 |
| Recorded audit verdicts | 28 |
| Discrimination records | 17 |
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
| Positive and negative tests | 28 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **38** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (28):** [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1), [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2), [`RFC1195-5.2-4`](#rfc1195-5.2-4), [`RFC1195-3.2-1`](#rfc1195-3.2-1), [`RFC1195-3.9-1`](#rfc1195-3.9-1), [`RFC1195-1.4-2`](#rfc1195-1.4-2), [`RFC1195-3.2-2`](#rfc1195-3.2-2), [`RFC1195-3.3-1`](#rfc1195-3.3-1), [`RFC1195-3.3-2`](#rfc1195-3.3-2), [`RFC1195-3.4-1`](#rfc1195-3.4-1), [`RFC1195-3.10-1`](#rfc1195-3.10-1), [`RFC1195-3.10-2`](#rfc1195-3.10-2), [`RFC1195-3.10-3`](#rfc1195-3.10-3), [`RFC1195-4.1-1`](#rfc1195-4.1-1), [`RFC1195-4.2-1`](#rfc1195-4.2-1), [`RFC1195-4.4-1`](#rfc1195-4.4-1), [`RFC1195-4.4-2`](#rfc1195-4.4-2), [`RFC1195-4.4-3`](#rfc1195-4.4-3), [`RFC1195-4.5-1`](#rfc1195-4.5-1), [`RFC1195-5.3.4-3`](#rfc1195-5.3.4-3), [`RFC1195-7-1`](#rfc1195-7-1), [`RFC1195-7-2`](#rfc1195-7-2), [`RFC1195-7-3`](#rfc1195-7-3), [`RFC1195-7-4`](#rfc1195-7-4), [`RFC1195-7-5`](#rfc1195-7-5), [`RFC1195-7-6`](#rfc1195-7-6), [`RFC1195-7-7`](#rfc1195-7-7), [`RFC1195-7-8`](#rfc1195-7-8)

**Annotated instead of tested (10):** [`RFC1195-5.2-1`](#rfc1195-5.2-1), [`RFC1195-5.2-2`](#rfc1195-5.2-2), [`RFC1195-5.2-3`](#rfc1195-5.2-3), [`RFC1195-3.1-1`](#rfc1195-3.1-1), [`RFC1195-1.4-1`](#rfc1195-1.4-1), [`RFC1195-1.2-1`](#rfc1195-1.2-1), [`RFC1195-3.2-3`](#rfc1195-3.2-3), [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1), [`RFC1195-8-1`](#rfc1195-8-1), [`RFC1195-8-2`](#rfc1195-8-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1195-5.2-1` | The "Protocols Supported" field identifies the protocols which are supported by each router. This field must be included in all IS-IS Hello packets and all LSPs with LSP number 0 transmitted by IP- capable routers. If this field is not included in an IS-IS Hello packet or an LSP with LSP number 0, it may be assumed that the packet was transmitted by an OSI-only router. The "Protocols Supported" field must also be included in ISO 9542 ISHs send by IP-capable routers over point-to-point links to other IS-IS routers. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L122). **negative:** no negative test. **{single-polarity}:** ze unconditionally emits the Protocols Supported TLV 129 with NLPID 0xCC for every IP-capable circuit IIH (internal/plugins/isis/circuit/hello.go:97-104) and LSP fragment 0 (internal/plugins/isis/lsdb/origination.go:407-409); there is no code path that omits TLV 129 or emits a non-0xCC IPv4 NLPID, so there is no negative form to reject |
| `RFC1195-5.2-2` | In Link State Packets, this field contains a list of one or more IP addresses corresponding to one or more interfaces of the router which originates the LSP. Each IP-capable router must include this field in its LSPs. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L110). **positive:** `unit/verify` [`TestISISTLVIPv4InterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L19). **negative:** no negative test. **{single-polarity}:** ze packs the IP Interface Address TLV 132 into LSP fragment 0 from the node's own interface addresses (internal/plugins/isis/lsdb/origination.go:433-438) and the codec reads it back verbatim (internal/plugins/isis/packet/tlv_ipv4.go DecodeIPv4InterfaceAddrTLV); this is an emit obligation with no decode-side reject-on-absence path, so there is no negative form to drive |
| `RFC1195-5.2-3` | Where a single router operates as both a level 1 and a level 2 router, it is required to include the same IP address(es) in its level 1 and level 2 LSPs. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISEngineOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L85). **negative:** no negative test. **{single-polarity}:** ze collects interface addresses level-independently (internal/plugins/isis/lsdb_wiring.go:436-448) so the same set is advertised at both levels by construction; there is no per-level divergence code path to drive a negative |
| `RFC1195-5.3.4-1` | Bit 7 of this field (marked I/E) indicates the metric type (internal or external) for all four TOS metrics, and must be set to zero indicating internal metrics. (§5.3.4) | MUST | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L311). **negative:** `unit/verify` [`TestRFC1195InternalOriginatorClearsExternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L411) |
| `RFC1195-5.3.4-2` | IP Internal Reachability Information -- IP addresses within the routing domain reachable directly via one or more interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.4) | MUST NOT | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L447). **negative:** `unit/verify` [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L448) |
| `RFC1195-5.2-4` | Each entry must contain a default metric (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L66). **negative:** `unit/verify` [`TestRFC1195MissingDefaultMetricCannotRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L245) |
| `RFC1195-3.2-1` | Note: If this sum results in a metric value greater than 63 (the maximum value that can be reported in level 2 LSPs), then the value 63 must be used. (Section 3.2; wide metrics use RFC 5305's separate bound) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L310). **negative:** `unit/verify` [`TestRFC1195NarrowLeakedMetricBelowCeiling`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L319) |
| `RFC1195-3.9-1` | If a packet is received which contains invalid authentication information, then the entire packet is discarded. (§3.9) | MUST | 3.9 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611) |
| `RFC1195-3.1-1` | Any codes in a received PDU that are not recognised shall be ignored and, for those packets which are forwarded (specifically Link State Packets), passed on unchanged. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16). **negative:** no negative test. **{single-polarity}:** the codec retains every TLV type it does not recognize as an opaque span and re-encodes it byte-for-byte (internal/plugins/isis/packet/tlv.go:66-89 iterator plus DecodeTLVs / writeTLVs) so the LSDB re-floods unknown TLVs verbatim (internal/plugins/isis/lsdb/flooding.go:342-360); preserving unrecognized codes IS the requirement and there is no reject path for an unknown TLV, so no negative form exists |
| `RFC1195-1.4-1` | Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4) | MUST | 1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |
| `RFC1195-1.2-1` | External links (to other routing domains) must be from level 2 routers (Section 1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 2966 Section 2.2, which updates this rule, "loosens the restrictions in RFC 1195, and allows for the inclusion of the "IP External Reachability Information" TLV in L1 LSPs"; ze advertises redistributed routes at every origination level (internal/plugins/isis/redistribute/consumer.go::InjectRoute), which the updated rule permits, and configures no circuit to another routing domain |
| `RFC1195-1.4-2` | In a pure IP routing domain, all routers must be IP-capable (Section 1.4) | MUST | 1.4 | **positive:** `unit/verify` [`TestRFC1195PureIPAdvertisesIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L185). **negative:** `unit/verify` [`TestRFC1195IPv6SelectionDoesNotRemoveIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L198) |
| `RFC1195-3.2-2` | In general, the same [IP address, subnet mask] pair may be announced in level 1 LSPs sent by multiple level 1 routers in the same area. In this case (assuming the entry is not superceded by a manually configured entry), then only one such entry shall be included in the level 2 LSP. The metric value(s) announced in level 2 LSPs correspond to the minimum of the metric value(s) that would be calculated for each of the level 1 LSP entries. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L436). **negative:** `unit/verify` [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L437) |
| `RFC1195-3.2-3` | If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep specific prefixes". Section 3.2 makes manual summary configuration optional ("Each level 2 router may be configured with one or more [IP address, subnet mask, metric] entries"). internal/plugins/isis/yang/ze-isis-conf.yang has no manual-summary configuration; spf.LeakPrefixes and lsdb.Originator.fragmentTLVs originate specific prefixes, so no locally configured summary exists to match this conditional discard rule. |
| `RFC1195-3.3-1` | The reserved field must contain "00 00", as specified in GOSIP version 2.0. (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC1195ISHGOSIPReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L15). **negative:** `unit/verify` [`TestRFC1195ISHGOSIPReservedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L39) |
| `RFC1195-3.3-2` | IEEE 802 addresses, if used, must appear in IEEE canonical format (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC1195ISHCanonicalIEEEIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L60). **negative:** `unit/verify` [`TestRFC1195ISHDoesNotReverseIdentifierBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L84) |
| `RFC1195-3.4-1` | When an external route using external metrics must be used, the lowest value of the external metric is preferred regardless of the internal cost to reach the appropriate exit point (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestRFC1195ExternalMetricBeforeExitCost`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L148). **negative:** `unit/verify` [`TestRFC1195ExternalMetricWithdrawalRecomputesWinner`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L159) |
| `RFC1195-3.10-1` | The Dijkstra calculation must calculate routes to each distinct IP reachability entry (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L29). **negative:** `unit/verify` [`TestRFC1195NoRouteForUnreachedReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L64) |
| `RFC1195-3.10-2` | If level 2 routes must be used, then routes within the routing domain (specifically, those routes using internal metrics) are prefered to routes outside of the routing domain (using external metrics). (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L169). **negative:** `unit/verify` [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L178) |
| `RFC1195-3.10-3` | However, the default metric must always be available. (Section 3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L32). **negative:** `unit/verify` [`TestRFC1195MissingDefaultMetricCannotRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L244) |
| `RFC1195-3.10.2-1` | 3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2) | MUST | 3.10.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Keep specific prefixes". The optional manual summaries described by section 3.2 ("Each level 2 router may be configured") have no configuration or origination path in internal/plugins/isis/yang/ze-isis-conf.yang, spf.LeakPrefixes, or lsdb.Originator.fragmentTLVs. There is no local manual-summary candidate to become the most desirable route; specific-prefix forwarding remains subject to normal reachability and protocol-suite rejection. |
| `RFC1195-4.1-1` | This implies that all IS-IS routers, including IP-only routers, must be able to receive IS-IS packets using the normal encapsulation for OSI packets. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC1195ParseFrameOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/frame_rfc1195_test.go#L33). **negative:** `unit/verify` [`TestRFC1195ParseFrameRefusesNonOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/frame_rfc1195_test.go#L50) |
| `RFC1195-4.2-1` | However, IP-capable routers must be able to interact correctly with other routers which assign multiple IP addresses per physical interface (up to the maximum of 63 addresses per interface). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC1195InterfaceAddrTLV63Addresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc1195_test.go#L31). **negative:** `unit/verify` [`TestRFC1195InterfaceAddrTLVTruncatedAddressRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc1195_test.go#L54) |
| `RFC1195-4.4-1` | IP-capable IS-IS routers therefore must be able to forward IP packets over existing adjacencies to routers with which they share physical connectivity, even when the IP address of the adjacent interface of the neighboring router is on a different logical IP subnet. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L36). **negative:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L37) |
| `RFC1195-4.4-2` | All IS-IS routers are therefore required to transmit and receive ISO 9542 ISH packets on point-to-point links. (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC1195ISHConfiguredPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/ish_auth_rfc1195_test.go#L22). **positive:** `unit/verify` [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L233). **positive:** `unit/verify` [`TestRFC1195ISHVethTransport`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/ish_integration_linux_test.go#L19). **negative:** `unit/verify` [`TestRFC1195ISHRejectsCorruptionAndWrongCircuit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L314). **negative:** `unit/verify` [`TestRFC1195ISHWrongPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/ish_auth_rfc1195_test.go#L64) |
| `RFC1195-4.4-3` | Thus, the value of the "protocols supported" field must be identical on every link (i.e., for any one router running IS-IS, all of the Hellos and LSPs transmitted by it must contain the same "protocols supported" values). (Section 4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L135). **negative:** `unit/verify` [`TestRFC1195NodeProtocolsReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L165) |
| `RFC1195-4.5-1` | There may be times when a dual router has to forward an IP packet to an OSI-only router, or forward an OSI packet to an IP-only router. In this case the packet must be discarded. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L38). **positive:** `unit/verify` [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L494). **positive:** `unit/verify` [`TestRFC1195OSINeighborIsTerminalIPv4NextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency_protocol_rfc1195_test.go#L39). **negative:** `unit/verify` [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L39). **negative:** `unit/verify` [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L495). **negative:** `unit/verify` [`TestRFC1195NeighborProtocolTransitions`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency_protocol_rfc1195_test.go#L56) |
| `RFC1195-5.3.4-3` | Bit 8 of this field is reserved, and must be set to zero on tranmission and ignored on reception. (Section 5.3.4), with default-metric bit 8 reassigned to up/down by RFC 2966 section 2 | MUST | 5.3.4 | **positive:** `unit/verify` [`TestRFC1195NarrowMetricReservedTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_narrow_ipv4_test.go#L14). **negative:** `unit/verify` [`TestRFC1195NarrowMetricReservedReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_narrow_ipv4_test.go#L32) |
| `RFC1195-7-1` | The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). (§B.1) | MUST | B.1 - Annex subsection B.1 | **positive:** `unit/verify` [`TestRFC1195CSNPEntriesAscending`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/snp_rfc1195_test.go#L31). **negative:** `unit/verify` [`TestRFC1195PSNPEntriesAscendingAcrossLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/snp_rfc1195_test.go#L59) |
| `RFC1195-7-2` | However, a full update must be done periodically to ensure recovery from data corruption, and studies suggest that with a very small number of link changes (perhaps 2) the expected computation complexity of the incremental update exceeds the complete recalculation. (§C.1) | MUST | C.1 - Annex subsection C.1 | **positive:** `unit/verify` [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L331). **negative:** `unit/verify` [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L332) |
| `RFC1195-7-3` | The 8 octet system identifiers which specify IP reachability entries must always be distinguishable from other system identifiers (§C.1.1) | MUST | C.1.1 - Annex subsection C.1.1 | **positive:** `unit/verify` [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L93). **negative:** `unit/verify` [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L142) |
| `RFC1195-7-4` | IP-capable level 2 routers must keep level 2 internal IP routes separate from level 2 external IP routes (§C.1.4) | MUST | C.1.4 - Annex subsection C.1.4 | **positive:** `unit/verify` [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L170). **negative:** `unit/verify` [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L179) |
| `RFC1195-7-5` | Each entry made to TENT must be marked as being either an End System or a router (§C.1.4) | MUST | C.1.4 - Annex subsection C.1.4 | **positive:** `unit/verify` [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L97). **negative:** `unit/verify` [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L143) |
| `RFC1195-7-6` | The password shall be configured on a per-link, per-area, and per- domain basis. (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195PasswordScopesResolved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L49). **negative:** `unit/verify` [`TestRFC1195PasswordScopesDoNotLeak`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L71) |
| `RFC1195-7-7` | Specifically, when this form of authentication is used: - IS-IS Hello and 9542 IS Hello packets shall contain the per-link password - Level 1 Link State Packets shall contain the per-area password - Level 2 Link State Packets shall contain the per-domain password - Level 1 Sequence Number Packets shall contain the per-area password - Level 2 Sequence Number Packets shall contain the per-domain password (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195PDUClassSignedWithItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L95). **negative:** `unit/verify` [`TestRFC1195PDUClassRefusedUnderOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L132) |
| `RFC1195-7-8` | Also, each of these three passwords shall be configured with: (i) "Transmit Password", whose value is a single password, and (ii) "Receive Passwords", whose value is a set of passwords. (§D.2) | MUST | D.2 - Annex subsection D.2 | **positive:** `unit/verify` [`TestRFC1195OneTransmitPasswordManyReceivePasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L204). **negative:** `unit/verify` [`TestRFC1195KeyOutsideReceiveSetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L230) |
| `RFC1195-8-1` | Type = 2 indicates that the external information field contains an autonomous system number tag, to be applied to subsequent IP external reachability information entries. In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. The AS tag is associated with subsequent IP External Reachability entries, until the end of the LSP, or until the next occurence of the Inter-Domain Routing Protocol Information field. (§A.2) | MUST | A.2 - Annex subsection A.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 says the IDRPI field "may be present" and "is not used by the IS-IS". internal/plugins/isis/lsdb/origination.go emits no TLV 131 and spf/graph.go has no IDRPI consumer; packet.DecodeTLVs retains unknown TLV 131 only for unchanged flooding. No AS-number association role is selected. |
| `RFC1195-8-2` | Type = 0 is reserved (must not be sent, and must be ignored on receipt). (Section A.2) | MUST NOT | A.2 - Annex subsection A.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 makes the IDRPI field optional ("may be present"). internal/plugins/isis/lsdb/origination.go has no TLV 131 producer; spf/graph.go consumes no IDRPI types, and packet.DecodeTLVs only retains the opaque field for unchanged flooding. Ze therefore neither originates type 0 nor interprets a received type 0. |
| `RFC1195-5.3.5-1` | Bit 7 of this field indicates the metric type (internal or external) for all four TOS metrics, and may be set to zero indicating internal metrics, or may be set to 1 indicating external metrics. (§5.3.5) | MAY | 5.3.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1195-1.4-1`](#rfc1195-1.4-1) Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs pure-IP Integrated IS-IS and routes no OSI/CLNP traffic (no CLNP forwarding code path), so the dual-domain topology constraint does not apply |
| [`RFC1195-1.2-1`](#rfc1195-1.2-1) External links (to other routing domains) must be from level 2 routers (Section 1.2) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 2966 Section 2.2, which updates this rule, "loosens the restrictions in RFC 1195, and allows for the inclusion of the "IP External Reachability Information" TLV in L1 LSPs"; ze advertises redistributed routes at every origination level (internal/plugins/isis/redistribute/consumer.go::InjectRoute), which the updated rule permits, and configures no circuit to another routing domain |
| [`RFC1195-3.2-3`](#rfc1195-3.2-3) If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep specific prefixes". Section 3.2 makes manual summary configuration optional ("Each level 2 router may be configured with one or more [IP address, subnet mask, metric] entries"). internal/plugins/isis/yang/ze-isis-conf.yang has no manual-summary configuration; spf.LeakPrefixes and lsdb.Originator.fragmentTLVs originate specific prefixes, so no locally configured summary exists to match this conditional discard rule. |
| [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1) 3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Keep specific prefixes". The optional manual summaries described by section 3.2 ("Each level 2 router may be configured") have no configuration or origination path in internal/plugins/isis/yang/ze-isis-conf.yang, spf.LeakPrefixes, or lsdb.Originator.fragmentTLVs. There is no local manual-summary candidate to become the most desirable route; specific-prefix forwarding remains subject to normal reachability and protocol-suite rejection. |
| [`RFC1195-8-1`](#rfc1195-8-1) Type = 2 indicates that the external information field contains an autonomous system number tag, to be applied to subsequent IP external reachability information entries. In this case, this "inter-domain routing protocol information" entry must contain precisely one 2 octet AS number. The AS tag is associated with subsequent IP External Reachability entries, until the end of the LSP, or until the next occurence of the Inter-Domain Routing Protocol Information field. (§A.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 says the IDRPI field "may be present" and "is not used by the IS-IS". internal/plugins/isis/lsdb/origination.go emits no TLV 131 and spf/graph.go has no IDRPI consumer; packet.DecodeTLVs retains unknown TLV 131 only for unchanged flooding. No AS-number association role is selected. |
| [`RFC1195-8-2`](#rfc1195-8-2) Type = 0 is reserved (must not be sent, and must be ignored on receipt). (Section A.2) | no test | no test carries this requirement id; annotated {not-applicable}: Owner decision 2026-09-21: "Do not originate IDRPI". Section 5.2 makes the IDRPI field optional ("may be present"). internal/plugins/isis/lsdb/origination.go has no TLV 131 producer; spf/graph.go consumes no IDRPI types, and packet.DecodeTLVs only retains the opaque field for unchanged flooding. Ze therefore neither originates type 0 nor interprets a received type 0. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1195-5.2-1`](#rfc1195-5.2-1)

The "Protocols Supported" field identifies the protocols which are supported by each router. This field must be included in all IS-IS Hello packets and all LSPs with LSP number 0 transmitted by IP- capable routers. If this field is not included in an IS-IS Hello packet or an LSP with LSP number 0, it may be assumed that the packet was transmitted by an OSI-only router. The "Protocols Supported" field must also be included in ISO 9542 ISHs send by IP-capable routers over point-to-point links to other IS-IS routers. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The tagged unit lsdb TestISISOriginateOnAdjacencyUp proves only the LSP-number-0 clause (TLV 129 with NLPID 0xCC in fragment 0). The quoted span also obliges Protocols Supported in every IS-IS Hello and in ISO 9542 ISHs on point-to-point links; those clauses are asserted by TestRFC1195NodeProtocolsAcrossInterfaces and TestRFC1195ISHTransmitReceive, which carry other tags, not this one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L122) | unit/verify | unproven |

### [`RFC1195-5.2-2`](#rfc1195-5.2-2)

In Link State Packets, this field contains a list of one or more IP addresses corresponding to one or more interfaces of the router which originates the LSP. Each IP-capable router must include this field in its LSPs. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. lsdb TestISISOriginateOnAdjacencyUp fails if the Originator omits TLV 132 when the test hands it InterfaceAddrs, but it checks only the TLV type, not the addresses, and the engine path that collects the router's interface addresses (lsdb_wiring.go) is not exercised by any tagged unit, so an engine that passed none stays green. The codec test proves decoding only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/origination_test.go#L110) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4InterfaceAddr`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L19) | unit/verify | unproven |

### [`RFC1195-5.2-3`](#rfc1195-5.2-3)

Where a single router operates as both a level 1 and a level 2 router, it is required to include the same IP address(es) in its level 1 and level 2 LSPs. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestISISEngineOriginateOnAdjacencyUp checks that fragment 0 exists at Level 1 and Level 2 but never compares the TLV 132 address sets of the two levels; it would pass if the levels advertised different addresses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISEngineOriginateOnAdjacencyUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb_wiring_test.go#L85) | unit/verify | unproven |

### [`RFC1195-5.3.4-1`](#rfc1195-5.3.4-1)

Bit 7 of this field (marked I/E) indicates the metric type (internal or external) for all four TOS metrics, and must be set to zero indicating internal metrics. (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a leaked internal L2 entry is decoded with ExternalMetric false. Negative: a producer asking for ExternalMetric on an internal prefix still emits TLV 128 with bit 0x40 clear. Both would fail if the originator set I/E.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InternalOriginatorClearsExternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L411) | unit/verify | unproven |
| positive | [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L311) | unit/verify | unproven |

### [`RFC1195-5.3.4-2`](#rfc1195-5.3.4-2)

IP Internal Reachability Information -- IP addresses within the routing domain reachable directly via one or more interfaces on this Intermediate system. This is permitted to appear multiple times, and in an LSP with any LSP number. However, this field must not appear in pseudonode LSPs. (§5.3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1195PseudonodeCannotInheritRouterPrefixes fails if the pseudonode LSP carries TLV 128 (also 130, 131) while the router LSP carries the prefixes. Positive and negative tags sit on the same assertion; the input is the violation attempt, so the one test discriminates.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L448) | unit/verify | unproven |
| positive | [`TestRFC1195PseudonodeCannotInheritRouterPrefixes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L447) | unit/verify | unproven |

### [`RFC1195-5.2-4`](#rfc1195-5.2-4)

Each entry must contain a default metric (§5.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The positive TestISISTLVIPv4RoundTrip round-trips TLV 135 (RFC 5305 extended reachability), which is not an RFC 1195 reachability entry. The negative TestRFC1195MissingDefaultMetricCannotRoute truncates a received TLV 128 so its length is no longer a multiple of 12, a structural error, and asserts receive-side rejection. Neither asserts that Ze's originated TLV 128/130 entries carry a default metric.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195MissingDefaultMetricCannotRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L245) | unit/verify | unproven |
| positive | [`TestISISTLVIPv4RoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_test.go#L66) | unit/verify | unproven |

### [`RFC1195-3.2-1`](#rfc1195-3.2-1)

Note: If this sum results in a metric value greater than 63 (the maximum value that can be reported in level 2 LSPs), then the value 63 must be used. (Section 3.2; wide metrics use RFC 5305's separate bound)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: L1 path 50+40 leaked to L2 with metric 63. Negative: 10+5 preserved as 15. Both fail if the clamp is removed or applied wrongly.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NarrowLeakedMetricBelowCeiling`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L319) | unit/verify | unproven |
| positive | [`TestRFC1195NarrowLeakedMetricClamps`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L310) | unit/verify | unproven |

### [`RFC1195-3.9-1`](#rfc1195-3.9-1)

If a packet is received which contains invalid authentication information, then the entire packet is discarded. (§3.9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The negative asserts only that VerifyPDU returns an error on a flipped digest bit. The obligation is that the entire packet is discarded; no tagged unit drives a bad-auth PDU through the receive path and asserts it changed no LSDB/adjacency state, so a caller that ignored the error stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L611) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L134) | unit/verify | unproven |

### [`RFC1195-3.1-1`](#rfc1195-3.1-1)

Any codes in a received PDU that are not recognised shall be ignored and, for those packets which are forwarded (specifically Link State Packets), passed on unchanged. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause 'ignored': DecodeTLVs returns no error on TLV 199/250, asserted. Clause 'forwarded LSPs passed on unchanged': TestISISUnknownTLVPassthrough only round-trips the codec (DecodeTLVs/writeTLVs); no tagged unit floods a received LSP carrying an unknown TLV and compares the bytes sent on another circuit, so a flooding path that rebuilt or stripped the LSP stays green. Tag prose still says 'sec 3.1' while the row cites 5.2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISUnknownTLVPassthrough`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_opaque_test.go#L16) | unit/verify | unproven |

### [`RFC1195-1.4-1`](#rfc1195-1.4-1)

Within a dual domain, if both IP and OSI traffic are to be routed between areas then all level 2 routers must be dual. (Section 1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-1.4-1, so no unit is bound to it.

### [`RFC1195-1.2-1`](#rfc1195-1.2-1)

External links (to other routing domains) must be from level 2 routers (Section 1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-1.2-1, so no unit is bound to it.

### [`RFC1195-1.4-2`](#rfc1195-1.4-2)

In a pure IP routing domain, all routers must be IP-capable (Section 1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195IPv6SelectionDoesNotRemoveIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC1195PureIPAdvertisesIPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L185) | unit/verify | unproven |

### [`RFC1195-3.2-2`](#rfc1195-3.2-2)

In general, the same [IP address, subnet mask] pair may be announced in level 1 LSPs sent by multiple level 1 routers in the same area. In this case (assuming the entry is not superceded by a manually configured entry), then only one such entry shall be included in the level 2 LSP. The metric value(s) announced in level 2 LSPs correspond to the minimum of the metric value(s) that would be calculated for each of the level 1 LSP entries. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Both clauses have a red assertion: metricLeak fails unless exactly one L2 entry is originated, and the test fails unless its metric is the minimum 9. But the positive and negative tags sit on the same assertion over the same input (one test wearing two hats, not a pair) and the row carries no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L437) | unit/verify | unproven |
| positive | [`TestRFC1195SpecificLeakCollapsesDuplicatePrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L436) | unit/verify | unproven |

### [`RFC1195-3.2-3`](#rfc1195-3.2-3)

If a level 2 router receives an IP packet whose IP address matches a manually configured address which it is including in its level 2 LSP, but which is not reachable via level 1 routing in the area, then the packet must be discarded. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-3.2-3, so no unit is bound to it.

### [`RFC1195-3.3-1`](#rfc1195-3.3-1)

The reserved field must contain "00 00", as specified in GOSIP version 2.0. (Section 3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Encoder emits the GOSIP NET with reserved octets zero; ParseNET and DecodeISH refuse either nonzero reserved octet with ErrNETReserved.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHGOSIPReservedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC1195ISHGOSIPReservedZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L15) | unit/verify | unproven |

### [`RFC1195-3.3-2`](#rfc1195-3.3-2)

IEEE 802 addresses, if used, must appear in IEEE canonical format (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHDoesNotReverseIdentifierBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L84) | unit/verify | unproven |
| positive | [`TestRFC1195ISHCanonicalIEEEIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/ish_test.go#L60) | unit/verify | unproven |

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
| negative | [`TestRFC1195NoRouteForUnreachedReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L29) | unit/verify | revert, verified |

### [`RFC1195-3.10-2`](#rfc1195-3.10-2)

If level 2 routes must be used, then routes within the routing domain (specifically, those routes using internal metrics) are prefered to routes outside of the routing domain (using external metrics). (Section 3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. A costly internal-metric L2 prefix beats a cheap external-metric one in the installed Loc-RIB; an external TLV with an internal metric is not penalized. Both fail if metric type is ignored.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L169) | unit/verify | unproven |

### [`RFC1195-3.10-3`](#rfc1195-3.10-3)

However, the default metric must always be available. (Section 3.10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive (TestRFC1195RouteForEachReachabilityEntry) proves SPF uses each entry's default metric (11/12/18). The negative truncates TLV 128 so its length is no longer a multiple of 12: the whole TLV fails a structural length rule first (question 3), so nothing isolates an entry lacking its default metric; same confound as 5.2-4.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195MissingDefaultMetricCannotRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L244) | unit/verify | unproven |
| positive | [`TestRFC1195RouteForEachReachabilityEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L32) | unit/verify | revert, verified |

### [`RFC1195-3.10.2-1`](#rfc1195-3.10.2-1)

3) If the specified destination is not reachable via level 1 routing, and the manually configured summary address advertised by this router (the router which has received the packet and is trying to forward it) represents the most desireable route, then the destination is unreachable and the packet must be discarded. (Section 3.10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1195-3.10.2-1, so no unit is bound to it.

### [`RFC1195-4.1-1`](#rfc1195-4.1-1)

This implies that all IS-IS routers, including IP-only routers, must be able to receive IS-IS packets using the normal encapsulation for OSI packets. (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The positive proves ParseFrame accepts the normal OSI 802.3/LLC encapsulation. The negative asserts refusal of Ethernet II and SNAP frames, which the sentence does not require; it is a neighbouring strictness property, so there is no negative for this capability obligation (a {single-polarity} marker would fit).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ParseFrameRefusesNonOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/frame_rfc1195_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC1195ParseFrameOSIEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/frame_rfc1195_test.go#L33) | unit/verify | revert, verified |

### [`RFC1195-4.2-1`](#rfc1195-4.2-1)

However, IP-capable routers must be able to interact correctly with other routers which assign multiple IP addresses per physical interface (up to the maximum of 63 addresses per interface). (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Both units exercise only DecodeIPv4InterfaceAddrTLV: 63 addresses decode, a truncated TLV is refused with ErrLength. Neither shows correct interaction (adjacency, next-hop choice) with a neighbor carrying several addresses per interface; the negative tests a malformed-length rule, not this one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InterfaceAddrTLVTruncatedAddressRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc1195_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC1195InterfaceAddrTLV63Addresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_ipv4_rfc1195_test.go#L31) | unit/verify | revert, verified |

### [`RFC1195-4.4-1`](#rfc1195-4.4-1)

IP-capable IS-IS routers therefore must be able to forward IP packets over existing adjacencies to routers with which they share physical connectivity, even when the IP address of the adjacent interface of the neighboring router is on a different logical IP subnet. (Section 4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestFIBOnLinkAdjacencyAndUnsupportedPrefix inserts a Loc-RIB path with OnLink=true by hand and proves the FIB/kernel forwards to the off-subnet gateway (and flips when the mark is cleared). Nothing tagged 4.4-1 shows IS-IS itself marks an adjacency next hop on a different logical subnet OnLink, so an IS-IS resolver that recursed instead stays green. Linux, CAP_NET_ADMIN only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L37) | unit/verify | unproven |
| positive | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L36) | unit/verify | unproven |

### [`RFC1195-4.4-2`](#rfc1195-4.4-2)

All IS-IS routers are therefore required to transmit and receive ISO 9542 ISH packets on point-to-point links. (Section 4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive clauses are asserted: TestRFC1195ISHTransmitReceive fails if no ISH is sent to AllESs before the IIH or if a received ISH does not initialize the adjacency; the veth test covers the AF_PACKET path. The negatives (corrupt ISH, wrong circuit, wrong/missing password) prove neighbouring rules, not a failure to transmit or receive ISH, and the row has no {single-polarity} marker. ish_auth tests prove the per-link password (7-7).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ISHWrongPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/ish_auth_rfc1195_test.go#L64) | unit/verify | unproven |
| negative | [`TestRFC1195ISHRejectsCorruptionAndWrongCircuit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L314) | unit/verify | unproven |
| positive | [`TestRFC1195ISHConfiguredPassword`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/ish_auth_rfc1195_test.go#L22) | unit/verify | unproven |
| positive | [`TestRFC1195ISHTransmitReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L233) | unit/verify | unproven |
| positive | [`TestRFC1195ISHVethTransport`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/transport/ish_integration_linux_test.go#L19) | unit/verify | unproven |

### [`RFC1195-4.4-3`](#rfc1195-4.4-3)

Thus, the value of the "protocols supported" field must be identical on every link (i.e., for any one router running IS-IS, all of the Hellos and LSPs transmitted by it must contain the same "protocols supported" values). (Section 4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Enabling IPv6 on one interface gives identical NLPIDs in both IIHs and the LSP; after reload no stale IPv6 capability remains on the unchanged circuit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NodeProtocolsReload`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L165) | unit/verify | unproven |
| positive | [`TestRFC1195NodeProtocolsAcrossInterfaces`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/protocol_rfc1195_test.go#L135) | unit/verify | unproven |

### [`RFC1195-4.5-1`](#rfc1195-4.5-1)

There may be times when a dual router has to forward an IP packet to an OSI-only router, or forward an OSI packet to an IP-only router. In this case the packet must be discarded. (§4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IP half: TestRFC1195OSINeighborIsTerminalIPv4NextHop fails if an OSI-only neighbor resolves to a usable IPv4 next hop; TestRFC1195CapabilityChangesInstalledRouteDisposition fails unless the installed route becomes Unreachable (production LSDB/SPF/installer) and returns to unicast when IPv4 support returns; the FIB integration test fails unless the kernel answers EHOSTUNREACH instead of the default and restores it on removal. OSI half: Ze forwards no OSI/CLNP packets, so no code path can violate it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L39) | unit/verify | unproven |
| negative | [`TestRFC1195NeighborProtocolTransitions`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency_protocol_rfc1195_test.go#L56) | unit/verify | unproven |
| negative | [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L495) | unit/verify | unproven |
| positive | [`TestFIBOnLinkAdjacencyAndUnsupportedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/onlink_integration_linux_test.go#L38) | unit/verify | unproven |
| positive | [`TestRFC1195OSINeighborIsTerminalIPv4NextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency_protocol_rfc1195_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC1195CapabilityChangesInstalledRouteDisposition`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L494) | unit/verify | unproven |

### [`RFC1195-5.3.4-3`](#rfc1195-5.3.4-3)

Bit 8 of this field is reserved, and must be set to zero on tranmission and ignored on reception. (Section 5.3.4), with default-metric bit 8 reassigned to up/down by RFC 2966 section 2

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The row quotes the DEFAULT METRIC bit-8 rule. The tagged units encode and decode bit 7 (0x40) of the DELAY, EXPENSE and ERROR metrics, which is a different sentence of 5.3.4 ('Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception'). Nothing tagged asserts default-metric bit 8, which RFC 2966 section 2 reassigns to up/down.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195NarrowMetricReservedReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_narrow_ipv4_test.go#L32) | unit/verify | unproven |
| positive | [`TestRFC1195NarrowMetricReservedTransmit`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/tlv_narrow_ipv4_test.go#L14) | unit/verify | unproven |

### [`RFC1195-7-1`](#rfc1195-7-1)

The entries shall be sorted into ascending LSPID order (the LSP number octet of the LSPID is the least significant octet). (§B.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §B.1, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: SNP entries in any order but ascending LSPID with the LSP number least significant. TestRFC1195CSNPEntriesAscending asserts the exact order [20.0, 20.1, 30.0, 40.0] from reverse inserts; TestRFC1195PSNPEntriesAscendingAcrossLists asserts [request, ack-only, ack] where list order would be (30, 10, 20). Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PSNPEntriesAscendingAcrossLists`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/snp_rfc1195_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC1195CSNPEntriesAscending`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/lsdb/snp_rfc1195_test.go#L31) | unit/verify | revert, verified |

### [`RFC1195-7-2`](#rfc1195-7-2)

However, a full update must be done periodically to ensure recovery from data corruption, and studies suggest that with a very small number of link changes (perhaps 2) the expected computation complexity of the incremental update exceeds the complete recalculation. (§C.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Still weak: TestRFC1195PeriodicSPFRecoversMissedUpdate carries both tags over one assertion (stale metric 11 repaired to 15 by the periodic pass), and the row carries no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L332) | unit/verify | unproven |
| positive | [`TestRFC1195PeriodicSPFRecoversMissedUpdate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L331) | unit/verify | unproven |

### [`RFC1195-7-3`](#rfc1195-7-3)

The 8 octet system identifiers which specify IP reachability entries must always be distinguishable from other system identifiers (§C.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1.1, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: an IP reachability entry whose identifier collides with a router's system identifier. TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix gives router B a System ID spelling 10.4.0.0/24 and asserts 3 settled router vertices, B at metric 10, C at 15 through B, and one prefix route at 11: a merged identifier space changes the vertex count. TestRFC1195PrefixCannotBridgeDisconnectedRouters asserts a lookalike prefix does not make a disconnected router's 198.51.100.0/24 reachable. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L93) | unit/verify | revert, verified |

### [`RFC1195-7-4`](#rfc1195-7-4)

IP-capable level 2 routers must keep level 2 internal IP routes separate from level 2 external IP routes (§C.1.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1.4, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: level 2 internal and external routes compared as one set. TestRFC1195InternalMetricOutranksExternal asserts the internal route at cost 163 wins over an external-metric route at cost 2 (metricWinner id 2, 163): merging the sets by cost goes red. The negative tag, TestRFC1195ExternalReachabilityWithInternalMetric, asserts an external-reachability route with an INTERNAL metric wins on cost, which is the §3.10.2 NOTE that such routes are treated as internal, a neighbouring rule, not a case where keeping the sets separate is violated. Weak: one polarity proves the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195ExternalReachabilityWithInternalMetric`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L179) | unit/verify | unproven |
| positive | [`TestRFC1195InternalMetricOutranksExternal`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc1195_spf_test.go#L170) | unit/verify | unproven |

### [`RFC1195-7-5`](#rfc1195-7-5)

Each entry made to TENT must be marked as being either an End System or a router (§C.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §C.1.4, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: a TENT entry not marked end system versus router, so an end system (a reachability entry) is used as a transit router. TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix asserts the settled set holds 3 routers only and the entry is reached through its advertising router; TestRFC1195PrefixCannotBridgeDisconnectedRouters asserts a prefix leaf does not bridge to a disconnected router's prefix. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PrefixCannotBridgeDisconnectedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L143) | unit/verify | unproven |
| positive | [`TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/spf/route_rfc1195_test.go#L97) | unit/verify | revert, verified |

### [`RFC1195-7-6`](#rfc1195-7-6)

The password shall be configured on a per-link, per-area, and per- domain basis. (§D.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §D.2, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: one password shared across link, area and domain scopes. TestRFC1195PasswordScopesResolved asserts three distinct chains iih/area/domain with distinct key ids; TestRFC1195PasswordScopesDoNotLeak asserts eth1 resolves no eth0 link chain at either level and the L2 chain is not the L1 chain. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PasswordScopesDoNotLeak`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestRFC1195PasswordScopesResolved`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L49) | unit/verify | revert, verified |

### [`RFC1195-7-7`](#rfc1195-7-7)

Specifically, when this form of authentication is used: - IS-IS Hello and 9542 IS Hello packets shall contain the per-link password - Level 1 Link State Packets shall contain the per-area password - Level 2 Link State Packets shall contain the per-domain password - Level 1 Sequence Number Packets shall contain the per-area password - Level 2 Sequence Number Packets shall contain the per-domain password (§D.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §D.2, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Still weak: the tagged units prove IS-IS Hello, L1/L2 LSP and L1/L2 CSNP/PSNP are signed with their scope's password and refused under another scope. The ISO 9542 IS Hello clause is proven only by TestRFC1195ISHConfiguredPassword and TestRFC1195ISHWrongPassword, which are tagged RFC1195-4.4-2, not this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195PDUClassRefusedUnderOtherScopes`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC1195PDUClassSignedWithItsScope`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L95) | unit/verify | revert, verified |

### [`RFC1195-7-8`](#rfc1195-7-8)

Also, each of these three passwords shall be configured with: (i) "Transmit Password", whose value is a single password, and (ii) "Receive Passwords", whose value is a set of passwords. (§D.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by TOOL-4 after the row's citation moved from §8 to §D.2, the annex subsection the heading reader now reads; the text and the tagged units are unchanged since QF-2 judged them the same day, so QF-2's reading stands. Forbidden: several transmit passwords, or a receive set of one. TestRFC1195OneTransmitPasswordManyReceivePasswords asserts sign key 1 only and a receive set of two that verifies both; TestRFC1195KeyOutsideReceiveSetRefused asserts a key outside the set is refused and the transmit key alone does not verify a key-2 PDU. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195KeyOutsideReceiveSetRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC1195OneTransmitPasswordManyReceivePasswords`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_rfc1195_test.go#L204) | unit/verify | revert, verified |

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
| Mapped sentences | 41 |
| Declined as scope | 67 |
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
| `5.3.4:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | the S bit reports an unsupported TOS metric, and RFC 1195 section 3.5 makes the feature optional: 'The support for TOS/QOS is optional.' ze offers only the default metric and encodes the RFC 5305 wide metric (internal/plugins/isis/types/metric.go), so it advertises no delay, expense or error metric at all | If this IS does not support this metric it shall set the bit "S" to 1 to indicate that the metric is unsupported. |
| `5.3.4:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the reserved-bit rule repeated for each of the delay, expense and error metric octets | Bit 7 of this field is reserved, and must be set to zero on transmission and ignored on reception. |
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
| `5.3.5:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the pseudonode prohibition repeated for the Inter-Domain Routing Protocol Information field, which the row already names | However, this field must not appear in pseudonode LSPs. |
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
