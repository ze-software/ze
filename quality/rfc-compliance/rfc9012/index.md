# RFC 9012 - The BGP Tunnel Encapsulation Attribute

Partial. Every requirement this repository extracted from RFC 9012, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 31.6% | 24 of 76 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1.3% | 1 of 76 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 76 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 76 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 76.4% | 68 of 89 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 76 | of 98 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 76 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.8% | 9 of 76 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 76 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 76 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 55.3% | 42 of 76 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 31 | of 76 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 76 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 98 |
| Gated MUST-level | 76 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 42 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 89 |
| Tagged units | 89 |
| Recorded audit verdicts | 31 |
| Discrimination records | 68 |
| Summary | `rfc/short/rfc9012.md` |
| Requirement shard | `rfc/requirements/rfc9012.md` |
| RFC text | `rfc/full/rfc9012.txt` |

## Enrolment

Enrolled: The BGP Tunnel Encapsulation Attribute

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Attribute code 23 retains opaque Tunnel Type values
- SubTLVs walks short and long headers, and Preference decodes the RFC 9830 six-octet value ([`internal/core/bgp/attribute/tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/tunnel_encap.go)). Receive enforcement checks framing, endpoint count and encoded length, and the Section 3.1 destination/forwardable restrictions using embedded IANA IPv4/IPv6 Special-Purpose Address Registries with longest-prefix overrides ([`internal/component/bgp/reactor/session_tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_tunnel_encap.go); internal/core/ipregistry). Invalid endpoint TLVs are removed
- malformed framing or no surviving TLV invokes treat-as-withdraw. Receive, storage and final-wire fixtures exercise all seven Section 6 carriers, unrelated attributes, unknown/duplicate content, reserved endpoint bytes, VXLAN/NVGRE R bits and nonzero Color Extended Community Flags. These are BGP signaling assertions, not tunnel dataplane proof.


**What the ledger says remains**

Full RFC support is not established; each canonical verdict and discrimination record bounds its own clause. Per-session outgoing attribute-23 filtering now has production cache, session-writer and downstream-receive unit proof through configured export policy, using a simulated external-plugin transport rather than a live daemon. This mechanism, configured exact-value community removal and whole-attribute egress suppression do not implement the missing default EBGP filtering of attribute 23 or the Encapsulation Extended Community. Other Section 11 scope obligations retain their checklist dispositions. Extraction site 13:7 maps to [`RFC9012-13-21`](#rfc9012-13-21), an explicit first-occurrence consumer gap; the historical RFC9012-13-7 identity remains retired. Attribute-driven tunnel selection/feasibility, encapsulation-header construction, recursive resolution, label handling, first-occurrence and type-specific sub-TLV interpretation, Router's MAC precedence and participating-router traffic filtering remain unimplemented consumer behavior, owned by [the separate tunnel-consumption spec](../../[`plan/spec-bgp-tunnel-encapsulation-consumption.md`](https://github.com/ze-software/ze/blob/main/plan/spec-bgp-tunnel-encapsulation-consumption.md)). In particular, signaling acceptance/propagation does not prove [`RFC9012-13-16`](#rfc9012-13-16)/-17/-21. The optional Section 3.1.1 origin-AS procedure is not commissioned. Tag presence and producer-dependence records alone do not establish whole-requirement conformance.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 24 | one part of the gated population |
| Annotated (including scoped evidence) | 52 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **76** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (24):** [`RFC9012-3.1-2`](#rfc9012-3.1-2), [`RFC9012-3.1-3`](#rfc9012-3.1-3), [`RFC9012-3.1-4`](#rfc9012-3.1-4), [`RFC9012-3.1-5`](#rfc9012-3.1-5), [`RFC9012-3.1-7`](#rfc9012-3.1-7), [`RFC9012-3.1-8`](#rfc9012-3.1-8), [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2), [`RFC9012-3.5-3`](#rfc9012-3.5-3), [`RFC9012-4.3-2`](#rfc9012-4.3-2), [`RFC9012-11-9`](#rfc9012-11-9), [`RFC9012-11-10`](#rfc9012-11-10), [`RFC9012-13-1`](#rfc9012-13-1), [`RFC9012-13-2`](#rfc9012-13-2), [`RFC9012-13-3`](#rfc9012-13-3), [`RFC9012-13-5`](#rfc9012-13-5), [`RFC9012-13-8`](#rfc9012-13-8), [`RFC9012-13-9`](#rfc9012-13-9), [`RFC9012-13-11`](#rfc9012-13-11), [`RFC9012-13-12`](#rfc9012-13-12), [`RFC9012-13-13`](#rfc9012-13-13), [`RFC9012-13-14`](#rfc9012-13-14), [`RFC9012-13-15`](#rfc9012-13-15), [`RFC9012-13-18`](#rfc9012-13-18), [`RFC9012-13-19`](#rfc9012-13-19)

**Annotated (including scoped evidence) (52):** [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3), [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1), [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3), [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4), [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5), [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6), [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1), [`RFC9012-3.2.5-1`](#rfc9012-3.2.5-1), [`RFC9012-3.3-1`](#rfc9012-3.3-1), [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1), [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2), [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3), [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4), [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2), [`RFC9012-3.5-1`](#rfc9012-3.5-1), [`RFC9012-3.5-2`](#rfc9012-3.5-2), [`RFC9012-3.5-4`](#rfc9012-3.5-4), [`RFC9012-3.6-1`](#rfc9012-3.6-1), [`RFC9012-3.6-2`](#rfc9012-3.6-2), [`RFC9012-3.6-3`](#rfc9012-3.6-3), [`RFC9012-3.6-4`](#rfc9012-3.6-4), [`RFC9012-3.6-5`](#rfc9012-3.6-5), [`RFC9012-3.6-6`](#rfc9012-3.6-6), [`RFC9012-3.6-7`](#rfc9012-3.6-7), [`RFC9012-3.6-8`](#rfc9012-3.6-8), [`RFC9012-3.6-13`](#rfc9012-3.6-13), [`RFC9012-3.7-2`](#rfc9012-3.7-2), [`RFC9012-3.7-3`](#rfc9012-3.7-3), [`RFC9012-3.7-4`](#rfc9012-3.7-4), [`RFC9012-4.1-1`](#rfc9012-4.1-1), [`RFC9012-4.1-2`](#rfc9012-4.1-2), [`RFC9012-4.1-3`](#rfc9012-4.1-3), [`RFC9012-4.2-1`](#rfc9012-4.2-1), [`RFC9012-4.3-1`](#rfc9012-4.3-1), [`RFC9012-6-2`](#rfc9012-6-2), [`RFC9012-7.1-1`](#rfc9012-7.1-1), [`RFC9012-8-1`](#rfc9012-8-1), [`RFC9012-8-2`](#rfc9012-8-2), [`RFC9012-10-1`](#rfc9012-10-1), [`RFC9012-11-1`](#rfc9012-11-1), [`RFC9012-11-2`](#rfc9012-11-2), [`RFC9012-11-5`](#rfc9012-11-5), [`RFC9012-11-6`](#rfc9012-11-6), [`RFC9012-11-8`](#rfc9012-11-8), [`RFC9012-11-11`](#rfc9012-11-11), [`RFC9012-13-4`](#rfc9012-13-4), [`RFC9012-13-6`](#rfc9012-13-6), [`RFC9012-13-10`](#rfc9012-13-10), [`RFC9012-13-16`](#rfc9012-13-16), [`RFC9012-13-17`](#rfc9012-13-17), [`RFC9012-13-21`](#rfc9012-13-21), [`RFC9012-15-1`](#rfc9012-15-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9012-3.1-2` | The Reserved subfield SHOULD be originated as zero. It MUST be disregarded on receipt, and it MUST be propagated unchanged. (§3.1) Context: this identity owns receipt-disregard of the Reserved subfield; unchanged propagation remains RFC9012-3.1-3. | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L359). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L360) |
| `RFC9012-3.1-3` | The Reserved subfield SHOULD be originated as zero. It MUST be disregarded on receipt, and it MUST be propagated unchanged. (§3.1) Context: this identity owns unchanged propagation of the Reserved subfield; receipt-disregard remains RFC9012-3.1-2. | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L333). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L351). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L336). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L352) |
| `RFC9012-3.1-4` | If the Address Family subfield contains the value for IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L402). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L248) |
| `RFC9012-3.1-5` | If the Address Family subfield contains the value for IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L403). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L249) |
| `RFC9012-3.1-7` | In this case, the Length field of Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06). (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L404). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L250) |
| `RFC9012-3.1-8` | When the Tunnel Encapsulation attribute is carried in an UPDATE message of one of the AFI/SAFIs specified in this document (see the first paragraph of Section 6), each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L35). **negative:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L36) |
| `RFC9012-3.1.1-3` | Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-3.2.1-1` | The remaining bits in the 8-bit Flags field are reserved for further use. They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the supported BGP origination path buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go emits SR Policy tunnel type 15, not VXLAN or NVGRE Encapsulation sub-TLVs; this origination boundary does not establish receive-disregard conformance |
| `RFC9012-3.2.1-2` | Intermediate routers MUST propagate them without modification. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L346). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L353). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L349). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L354) |
| `RFC9012-3.2.1-3` | Any receiving routers MUST ignore these bits upon receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-4` | If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-5` | If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-6` | Reserved: MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.4-1` | Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4) | MUST NOT | 3.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go originates SR Policy tunnel type 15, not a GRE Encapsulation sub-TLV; interface GRE keys in internal/plugins/iface/netlink/tunnel_linux.go are not advertised by that BGP producer |
| `RFC9012-3.2.5-1` | Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.5) | MUST NOT | 3.2.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go originates SR Policy tunnel type 15, not an MPLS-in-GRE Encapsulation sub-TLV; this distinct originator condition is not a receive or tunnel-consumption proof |
| `RFC9012-3.3-1` | If an outer Encapsulation sub-TLV occurs in a TLV for a tunnel type that does not use the corresponding outer encapsulation, the sub-TLV MUST be treated as if it were an unrecognized type of sub-TLV. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze recognizes no Encapsulation sub-TLV under any tunnel type, so it never makes the tunnel-type-specific determination this requires: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.3.2-1` | If the reserved value zero is received, the sub-TLV MUST be treated as malformed, according to the rules of Section 13. (§3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no UDP Destination Port sub-TLV decoder exists, so the reserved value zero is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-2` | Packets with other payload types MUST NOT be encapsulated in the relevant tunnel. (§3.4.1) | MUST NOT | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no tunnel encapsulation from the attribute, so no payload type is ever checked against a Protocol Type sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the FIB receives no tunnel from BGP (grep for SubTLVs over internal/plugins/fib matches nothing) |
| `RFC9012-3.4.1-3` | If the reserved value 0xFFFF is received, the sub-TLV MUST be treated as malformed according to the rules of Section 13. (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Protocol Type sub-TLV decoder exists, so the reserved value 0xFFFF is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-4` | Also, for "X-in-Y" type tunnels, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored; this is discussed further in Section 13. (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Protocol Type sub-TLV and forms no X-in-Y tunnel, so a mismatched payload type is never identified: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.4.2-2` | If the Length field of a Color sub-TLV has a value other than 8, or the first two octets of its Value field are not 0x030b, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV (see Section 13). (§3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV decoder exists, so neither its Length nor its leading 0x030b octets are checked: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-1` | If the Tunnel Encapsulation attribute is attached to an UPDATE of a non-labeled address family, then the sub-TLV MUST be disregarded. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Embedded Label Handling sub-TLV is never decoded and the attribute is never correlated with the address family of the UPDATE carrying it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-2` | If the sub-TLV is contained in a TLV whose tunnel type does not have a virtual network identifier in its encapsulation header, the sub-TLV MUST be disregarded. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code relates a sub-TLV to whether its tunnel type uses a virtual network identifier: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-3` | In those cases where the sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated. (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L365). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L355). **negative:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L373). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L356) |
| `RFC9012-3.5-4` | If any value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed, according to the procedures of Section 13. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Embedded Label Handling value is never read, so a value outside 1 and 2 is not detected and not treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.6-1` | When this label stack is pushed onto a packet, this ordering MUST be preserved. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no ordering is preserved or lost: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/mpls matches nothing |
| `RFC9012-3.6-2` | If a packet is to be sent through the tunnel identified in a particular TLV, and if that TLV contains an MPLS Label Stack sub-TLV, then the label stack appearing in the sub-TLV MUST be pushed onto the packet before any other labels are pushed onto the packet. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no MPLS Label Stack sub-TLV is decoded and no push order is established: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-3` | In particular, if the Tunnel Encapsulation attribute is attached to a BGP UPDATE of a labeled address family, the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the label embedded in the NLRI is pushed onto the packet. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never combines an MPLS Label Stack sub-TLV with the label embedded in a labeled-unicast NLRI: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-4` | If the MPLS Label Stack sub-TLV is included in a TLV identifying a tunnel type that uses virtual network identifiers (see Section 9), the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the procedures of Section 9 are applied. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies no virtual network identifier procedure, so nothing is sequenced against it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-5` | The number of label stack entries in the sub-TLV MUST be determined from the Sub-TLV Length field. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no label stack entries are ever counted: SubTLVs uses the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and no caller reads sub-TLV type 10 |
| `RFC9012-3.6-6` | When the label stack entries are pushed onto a packet that already has a label stack, the S bits of all the entries being pushed MUST be cleared. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no S bit is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-7` | When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no bottom-of-stack bit is set: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-8` | When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set, and the S bit of all the other label stack entries MUST be cleared. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no S bit of a non-bottom entry is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-13` | If any label stack entry in the sub-TLV has a TTL value of zero, the router that is pushing the stack onto a packet MUST change the value to a non-zero value, either 255 or some other value as determined by policy as discussed above. (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so a zero TTL in a received entry is never rewritten: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.7-2` | [RFC8669] only defines behavior when the BGP Prefix-SID attribute is attached to routes of type IPv4/IPv6 Labeled Unicast [RFC4760][RFC8277], and it only defines values of the BGP Prefix-SID attribute for those cases. Therefore, similar limitations exist for the Prefix-SID sub-TLV: it SHOULD only be included in a BGP UPDATE message for one of the address families for which [RFC8669] has a defined behavior, namely BGP IPv4/IPv6 Labeled Unicast [RFC4760] [RFC8277]. If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** opaque attribute receipt is not family-conditioned interpretation of the Prefix-SID sub-TLV; the absent attribute-driven consumer is owned by plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-3.7-3` | If an Originator SRGB is specified in the sub-TLV, that SRGB MUST be interpreted to be the SRGB used by the tunnel's egress endpoint. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| `RFC9012-3.7-4` | If a Label-Index is present in the Prefix-SID sub-TLV, then when a packet is sent through the tunnel identified by the TLV, if that tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-4.1-1` | In situations where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| `RFC9012-4.1-2` | Notwithstanding, an implementation MUST be prepared to process a tunnel received encoded as a barebones TLV. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| `RFC9012-4.1-3` | Packets with other payload types MUST NOT be carried through such tunnels. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| `RFC9012-4.2-1` | [EVPN-INTER-SUBNET] defines a router's MAC Extended Community. This extended community, as its name implies, carries the MAC address of the advertising router. Since the VXLAN and NVGRE Encapsulation sub-TLVs can also optionally carry a router's MAC, a conflict can arise if both the Router's MAC Extended Community and such an Encapsulation sub-TLV are present at the same time but have different values. In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the two received opaque values can disagree even without a decoder; ParseTunnelEncap/SubTLVs and extended-community byte preservation do not implement MAC precedence. The absent consumer is owned by plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-4.3-1` | No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| `RFC9012-4.3-2` | No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver; the value MUST NOT be changed when propagating this extended community. (§4.3) Context: the Flags field of the Color Extended Community; this identity owns unchanged propagation, not the subsequent Color Value field. | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L397). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L357). **negative:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L400). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L358) |
| `RFC9012-6-2` | Then router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| `RFC9012-7.1-1` | If a route includes the Tunnel Encapsulation attribute, and if that attribute includes no tunnel that is feasible, then that route MUST NOT be considered resolvable for the purposes of the route resolvability condition ([RFC4271], Section 9.1.2.1). (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-8-1` | Then packet P MUST be sent through one of the tunnels identified in the Tunnel Encapsulation attribute of UPDATE U2. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-8-2` | In that case, packet P MUST NOT be sent through the tunnel contained in that TLV, unless U1 is carrying a Color Extended Community that is identified in one of U2's Color sub-TLVs. (§8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| `RFC9012-10-1` | Any document specifying such joint use MUST provide details as to how interactions should be handled. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| `RFC9012-11-1` | However, the Tunnel Encapsulation attribute MUST be used only within a well-defined scope, for example, within a set of ASes that belong to a single administrative entity. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go validates framing and endpoints, not administrative scope. The configured policy path has no attribute-23 name in internal/component/bgp/reactor/filter_format.go attrNameToCode; endpoint validation does not establish scope enforcement |
| `RFC9012-11-2` | To prevent the Tunnel Encapsulation attribute from being distributed beyond its intended scope, any BGP speaker that understands the attribute MUST be able to filter the attribute from incoming BGP UPDATE messages. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the built-in text policy path cannot request attribute-23 removal: internal/component/bgp/reactor/filter_delta.go textDeltaToModOps uses attrNameToCode in filter_format.go, which has no attribute-23 entry. applyTunnelEncap removes invalid endpoint TLVs, not otherwise valid attributes by session scope |
| `RFC9012-11-5` | For each external BGP (EBGP) session, filtering of the attribute on incoming UPDATEs MUST be enabled by default. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/reactor/filter_ordered.go runIngressPolicyChain accepts when no active policy is configured; internal/component/bgp/reactor/session_tunnel_encap.go applyTunnelEncap has no EBGP scope-filter decision. Receive validation is not default attribute-23 filtering |
| `RFC9012-11-6` | In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/reactor/filter_delta.go textDeltaToModOps has no attribute-23 policy name, and internal/component/bgp/reactor/filter_delta_handlers.go attrModHandlersWithDefaults supplies no attribute-23 handler. The generic rebuild mechanism in forward_build.go buildModifiedPayload alone does not provide the missing configured filter |
| `RFC9012-11-8` | For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/reactor/filter_ordered.go runEgressPolicyChainASN4 accepts when no active export policy is configured; the current forwarding policy has no default attribute-23 suppression |
| `RFC9012-11-9` | * Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. (§11) Context: internal/component/bgp/plugins/filter_community/filter.go applyIngressFilter and ingressStripCommunities remove configured exact eight-octet extended-community values. This can remove a named Encapsulation Extended Community; lack of a type-wide matcher alone does not prove that no filtering capability exists. Whole-clause proof is not established here. | MUST | 11 | **positive:** `unit/verify` [`TestRFC9012ConfiguredEncapsulationECIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L93). **negative:** `unit/verify` [`TestRFC9012ConfiguredEncapsulationECIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L94) |
| `RFC9012-11-10` | It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) Context: internal/component/bgp/plugins/filter_community/egress.go applyEgressFilter records configured exact-value removals, and handler.go genericCommunityHandler applies them. internal/component/bgp/reactor/peer_forward_facts.go applySendCommunityMask can also suppress the entire Extended Communities attribute. These mechanisms are not an EBGP default or independent whole-clause proof. | MUST | 11 | **positive:** `unit/verify` [`TestRFC9012ConfiguredEncapsulationECEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L104). **negative:** `unit/verify` [`TestRFC9012ConfiguredEncapsulationECEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L105) |
| `RFC9012-11-11` | * In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/filter_community/filter.go applyIngressFilter strips configured values, not Encapsulation Extended Communities by default; internal/component/bgp/reactor/peer_forward_facts.go sendCommunitySuppression returns no suppression for an empty list or all. Configurable community removal does not implement this EBGP default |
| `RFC9012-13-1` | The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L307). **negative:** `unit/verify` [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L308) |
| `RFC9012-13-2` | The final octet of a TLV MUST also be the final octet of its final sub-TLV. If this is not the case, the TLV MUST be considered to be malformed, and the "Treat-as-withdraw" procedure of [RFC7606] is applied. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L30). **positive:** `unit/verify` [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L309). **negative:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L31). **negative:** `unit/verify` [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L310) |
| `RFC9012-13-3` | If a Tunnel Encapsulation attribute can be parsed correctly but contains a TLV whose tunnel type is not recognized by a particular BGP speaker, that BGP speaker MUST NOT consider the attribute to be malformed. (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L334). **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L115). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L150). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L335) |
| `RFC9012-13-4` | Rather, it MUST interpret the attribute as if that TLV had not been present. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the attribute-driven tunnel consumer is absent, so opaque carriage of an unknown tunnel type does not prove its exclusion from tunnel selection or header construction. applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go validates framing and endpoints, and separately enforces RFC9830 tunnel-type constraints on SR Policy carriers; those receive checks are not the absent RFC9012 consumer. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-13-5` | If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized TLV MUST remain in the attribute. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L336). **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L124). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L337). **negative:** `unit/verify` [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L138) |
| `RFC9012-13-6` | The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| `RFC9012-13-8` | However, the Tunnel TLV containing them MUST NOT be considered to be malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L338). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L160). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L339) |
| `RFC9012-13-9` | all the sub-TLVs MUST be propagated if the route carrying the Tunnel Encapsulation attribute is propagated. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L179). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L340). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L193). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L341) |
| `RFC9012-13-10` | If a TLV of a Tunnel Encapsulation attribute contains a sub-TLV that is not recognized by a particular BGP speaker, the BGP speaker MUST process that TLV as if the unrecognized sub-TLV had not been present. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L342). **positive:** `unit/verify` [`TestRFC9012UnknownSubTLVEndpointProcessing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L648). **positive:** `unit/verify` [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L212). **negative:** no negative test. **{single-polarity}:** an unrecognized sub-TLV is legal input. Endpoint malformation and missing or duplicate endpoints exercise processing equivalence in the presence of that legal input; they do not create an invalid unknown type. Separate framing and endpoint obligations retain their own negative coverage. |
| `RFC9012-13-11` | If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized sub-TLV MUST remain in the attribute. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L186). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L343). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L196). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L344) |
| `RFC9012-13-12` | In general, if a TLV contains a sub-TLV that is malformed, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L240). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L345). **negative:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L249). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L346) |
| `RFC9012-13-13` | if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L555). **positive:** `unit/verify` [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L104). **positive:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L251). **negative:** `unit/verify` [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L556). **negative:** `unit/verify` [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L105). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L252) |
| `RFC9012-13-14` | There is one exception to this rule: if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation attribute before the route carrying that attribute is distributed. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L552). **positive:** `unit/verify` [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L100). **positive:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L245). **negative:** `unit/verify` [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L553). **negative:** `unit/verify` [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L101). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L246) |
| `RFC9012-13-15` | Within a Tunnel Encapsulation attribute that is carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L243). **positive:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L32). **negative:** `unit/verify` [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L244). **negative:** `unit/verify` [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L33) |
| `RFC9012-13-16` | A TLV identifying a particular tunnel type may contain a sub-TLV that is meaningless for that tunnel type. For example, perhaps the TLV contains a UDP Destination Port sub-TLV, but the identified tunnel type does not use UDP encapsulation at all, or a tunnel of the form "X-in-Y" contains a Protocol Type sub-TLV that specifies something other than "X". Sub-TLVs of this sort MUST be disregarded. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no attribute-driven encapsulation consumer applies the type-specific disregard rule; signaling acceptance and unchanged propagation do not prove it. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md; owner ruling 2026-10-08: retain opaque carry/non-malformed tests under RFC9012-13-18/-19 and existing RFC9830 identities; the absent attribute-driven consumer remains owned by plan/spec-bgp-tunnel-encapsulation-consumption.md. |
| `RFC9012-13-17` | That is, they MUST NOT affect the creation of the encapsulation header. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze creates no encapsulation header from attribute 23, so this conditional header-construction behavior is unimplemented, not proven by propagating opaque bytes. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-13-18` | However, the sub-TLV MUST NOT be considered to be malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L295). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L347). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L348) |
| `RFC9012-13-19` | the sub-TLV MUST NOT be considered to be malformed and MUST NOT be removed from the TLV before the route carrying the Tunnel Encapsulation attribute is distributed. (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L316). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L349). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L319). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L350) |
| `RFC9012-13-21` | If a Tunnel TLV has more than one of any of these sub-TLVs, all but the first occurrence of each such sub-TLV type MUST be disregarded. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the absent attribute-driven consumer does not implement first-occurrence interpretation for Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack or Prefix-SID. internal/core/bgp/attribute/tunnel_encap.go TunnelTLV.SubTLVs preserves every occurrence; TunnelTLV.Preference concerns a different RFC 9830 sub-TLV and does not prove this rule. On Section 6 carriers, the separate exactly-one Tunnel Egress Endpoint exception is enforced by tunnelTLVLayout and applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go and remains accounted for by RFC9012-13-13/-14/-15. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-15-1` | RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). For these purposes, traffic introduced into an SR domain using the Prefix-SID sub-TLV lies within the SR domain, even though the Prefix-SIDs used by the routers at the two ends of the tunnel may be different, as discussed in Section 3.7. This implies that the duty to filter external traffic extends to all routers participating in such tunnels. (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the absent attribute-driven Prefix-SID tunnel consumer does not install this participating-router traffic filter; attribute-23 ingress/egress filtering is a different behavior. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| `RFC9012-3.1-1` | The Reserved subfield SHOULD be originated as zero. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-9` | The Traffic Class (TC) field [RFC3270][RFC5129] of each label stack entry SHOULD be set to 0, unless changed by policy at the originator of the sub-TLV. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-10` | When pushing the label stack onto a packet, the TC of each label stack SHOULD be preserved, unless local policy results in a modification. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-11` | The TTL (Time to Live) field of each label stack entry SHOULD be set to 255, unless changed to some other non-zero value by policy at the originator of the sub-TLV. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-12` | When pushing the label stack onto a packet, the TTL of each label stack entry SHOULD be preserved, unless local policy results in a modification to some other non-zero value. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.7-1` | similar limitations exist for the Prefix-SID sub-TLV: it SHOULD only be included in a BGP UPDATE message for one of the address families for which [RFC8669] has a defined behavior, namely BGP IPv4/IPv6 Labeled Unicast [RFC4760] [RFC8277]. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-3` | When the attribute is filtered from an incoming UPDATE, the attribute is neither processed nor distributed. This filtering SHOULD be possible on a per-BGP-session basis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-7` | This filtering SHOULD be possible on a per-BGP-session basis. (§11) Context: outgoing Tunnel Encapsulation attribute filtering; RFC9012-11-6 owns the preceding filter-capability MUST. | SHOULD | 11 | **positive:** `unit/verify` [`TestRFC9012TunnelFilterSelectedSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_session_filter_test.go#L37). **negative:** `unit/verify` [`TestRFC9012TunnelFilterDoesNotAffectOtherSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_session_filter_test.go#L50) |
| `RFC9012-1.5-1` | the Load-Balancing Block sub-TLV MAY be included in any Tunnel Encapsulation attribute where load balancing is desired. (§1.5) | MAY | 1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1-6` | the Tunnel Egress Endpoint sub-TLV MAY have a Value field whose Address Family subfield contains 0. This means that the tunnel's egress endpoint is the address of the next hop. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1-9` | (Such routes are sometimes colloquially known as "Martians".) This restriction MAY be relaxed by explicit configuration. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1.1-1` | This section provides a procedure that MAY be applied to validate that the IP address in the sub-TLV's Address subfield belongs to the AS that originated the route that contains the attribute. (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1.1-2` | In some cases, a network operator who controls a set of ASes might wish to allow a tunnel egress endpoint to reside in an AS other than Route_AS; configuration MAY allow for such a case, in which case the check becomes: if Egress_AS is not within the configured set of permitted AS numbers, then the Tunnel Egress Endpoint sub-TLV is considered to be "malformed". (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.3.1-1` | an implementation MAY provide a facility to use policy to filter or modify the DS field. (§3.3.1) | MAY | 3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.4.1-1` | The Protocol Type sub-TLV MAY be included in a given TLV to indicate the type of the payload packets that are allowed to be encapsulated with the tunnel parameters that are being signaled in the TLV. (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.4.2-1` | The Color sub-TLV MAY be used as a way to "color" the corresponding Tunnel TLV. (§3.4.2) | MAY | 3.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-14` | If an invalid or unsupported label stack is received, the tunnel MAY be treated as not feasible, according to the procedures of Section 6. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-1` | The BGP Tunnel Encapsulation attribute MAY be carried in any BGP UPDATE message whose AFI/SAFI is 1/1 (IPv4 Unicast), 2/1 (IPv6 Unicast), 1/4 (IPv4 Labeled Unicast), 2/4 (IPv6 Labeled Unicast), 1/128 (VPN-IPv4 Labeled Unicast), 2/128 (VPN-IPv6 Labeled Unicast), or 25/70 (Ethernet VPN, usually known as EVPN). (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-3` | Notwithstanding anything said in this document, a BGP speaker MAY have local policy that influences the choice of tunnel and the way the encapsulation is formed. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-4` | A BGP speaker MAY also have a local policy that tells it to ignore the Tunnel Encapsulation attribute entirely or in part. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-4` | finer granularities (for example, per route and/or per attribute TLV) MAY be supported. (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-13-20` | An implementation MAY log a message when it encounters such a sub-TLV. (§13) | MAY | 13 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3) Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1) The remaining bits in the 8-bit Flags field are reserved for further use. They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the supported BGP origination path buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go emits SR Policy tunnel type 15, not VXLAN or NVGRE Encapsulation sub-TLVs; this origination boundary does not establish receive-disregard conformance |
| [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3) Any receiving routers MUST ignore these bits upon receipt. (§3.2.1, §3.2.2) | {gap}, no test | ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4) If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5) If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6) Reserved: MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1) Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4) | no test | no test carries this requirement id; annotated {not-applicable}: buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go originates SR Policy tunnel type 15, not a GRE Encapsulation sub-TLV; interface GRE keys in internal/plugins/iface/netlink/tunnel_linux.go are not advertised by that BGP producer |
| [`RFC9012-3.2.5-1`](#rfc9012-3.2.5-1) Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.5) | no test | no test carries this requirement id; annotated {not-applicable}: buildTunnelEncap in internal/component/bgp/plugins/nlri/srpolicy/config.go originates SR Policy tunnel type 15, not an MPLS-in-GRE Encapsulation sub-TLV; this distinct originator condition is not a receive or tunnel-consumption proof |
| [`RFC9012-3.3-1`](#rfc9012-3.3-1) If an outer Encapsulation sub-TLV occurs in a TLV for a tunnel type that does not use the corresponding outer encapsulation, the sub-TLV MUST be treated as if it were an unrecognized type of sub-TLV. (§3.3) | {gap}, no test | ze recognizes no Encapsulation sub-TLV under any tunnel type, so it never makes the tunnel-type-specific determination this requires: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1) If the reserved value zero is received, the sub-TLV MUST be treated as malformed, according to the rules of Section 13. (§3.3.2) | {gap}, no test | no UDP Destination Port sub-TLV decoder exists, so the reserved value zero is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2) Packets with other payload types MUST NOT be encapsulated in the relevant tunnel. (§3.4.1) | {gap}, no test | ze builds no tunnel encapsulation from the attribute, so no payload type is ever checked against a Protocol Type sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the FIB receives no tunnel from BGP (grep for SubTLVs over internal/plugins/fib matches nothing) |
| [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3) If the reserved value 0xFFFF is received, the sub-TLV MUST be treated as malformed according to the rules of Section 13. (§3.4.1) | {gap}, no test | no Protocol Type sub-TLV decoder exists, so the reserved value 0xFFFF is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4) Also, for "X-in-Y" type tunnels, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored; this is discussed further in Section 13. (§3.4.1) | {gap}, no test | ze reads no Protocol Type sub-TLV and forms no X-in-Y tunnel, so a mismatched payload type is never identified: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2) If the Length field of a Color sub-TLV has a value other than 8, or the first two octets of its Value field are not 0x030b, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV (see Section 13). (§3.4.2) | {gap}, no test | no Color sub-TLV decoder exists, so neither its Length nor its leading 0x030b octets are checked: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-1`](#rfc9012-3.5-1) If the Tunnel Encapsulation attribute is attached to an UPDATE of a non-labeled address family, then the sub-TLV MUST be disregarded. (§3.5) | {gap}, no test | the Embedded Label Handling sub-TLV is never decoded and the attribute is never correlated with the address family of the UPDATE carrying it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-2`](#rfc9012-3.5-2) If the sub-TLV is contained in a TLV whose tunnel type does not have a virtual network identifier in its encapsulation header, the sub-TLV MUST be disregarded. (§3.5) | {gap}, no test | no code relates a sub-TLV to whether its tunnel type uses a virtual network identifier: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-4`](#rfc9012-3.5-4) If any value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed, according to the procedures of Section 13. (§3.5) | {gap}, no test | the Embedded Label Handling value is never read, so a value outside 1 and 2 is not detected and not treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.6-1`](#rfc9012-3.6-1) When this label stack is pushed onto a packet, this ordering MUST be preserved. (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no ordering is preserved or lost: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/mpls matches nothing |
| [`RFC9012-3.6-2`](#rfc9012-3.6-2) If a packet is to be sent through the tunnel identified in a particular TLV, and if that TLV contains an MPLS Label Stack sub-TLV, then the label stack appearing in the sub-TLV MUST be pushed onto the packet before any other labels are pushed onto the packet. (§3.6) | {gap}, no test | no MPLS Label Stack sub-TLV is decoded and no push order is established: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-3`](#rfc9012-3.6-3) In particular, if the Tunnel Encapsulation attribute is attached to a BGP UPDATE of a labeled address family, the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the label embedded in the NLRI is pushed onto the packet. (§3.6) | {gap}, no test | ze never combines an MPLS Label Stack sub-TLV with the label embedded in a labeled-unicast NLRI: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-4`](#rfc9012-3.6-4) If the MPLS Label Stack sub-TLV is included in a TLV identifying a tunnel type that uses virtual network identifiers (see Section 9), the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the procedures of Section 9 are applied. (§3.6) | {gap}, no test | ze applies no virtual network identifier procedure, so nothing is sequenced against it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-5`](#rfc9012-3.6-5) The number of label stack entries in the sub-TLV MUST be determined from the Sub-TLV Length field. (§3.6) | {gap}, no test | no label stack entries are ever counted: SubTLVs uses the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and no caller reads sub-TLV type 10 |
| [`RFC9012-3.6-6`](#rfc9012-3.6-6) When the label stack entries are pushed onto a packet that already has a label stack, the S bits of all the entries being pushed MUST be cleared. (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no S bit is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-7`](#rfc9012-3.6-7) When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no bottom-of-stack bit is set: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-8`](#rfc9012-3.6-8) When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set, and the S bit of all the other label stack entries MUST be cleared. (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no S bit of a non-bottom entry is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-13`](#rfc9012-3.6-13) If any label stack entry in the sub-TLV has a TTL value of zero, the router that is pushing the stack onto a packet MUST change the value to a non-zero value, either 255 or some other value as determined by policy as discussed above. (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so a zero TTL in a received entry is never rewritten: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.7-2`](#rfc9012-3.7-2) [RFC8669] only defines behavior when the BGP Prefix-SID attribute is attached to routes of type IPv4/IPv6 Labeled Unicast [RFC4760][RFC8277], and it only defines values of the BGP Prefix-SID attribute for those cases. Therefore, similar limitations exist for the Prefix-SID sub-TLV: it SHOULD only be included in a BGP UPDATE message for one of the address families for which [RFC8669] has a defined behavior, namely BGP IPv4/IPv6 Labeled Unicast [RFC4760] [RFC8277]. If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7) | {gap}, no test | opaque attribute receipt is not family-conditioned interpretation of the Prefix-SID sub-TLV; the absent attribute-driven consumer is owned by plan/spec-bgp-tunnel-encapsulation-consumption.md |
| [`RFC9012-3.7-3`](#rfc9012-3.7-3) If an Originator SRGB is specified in the sub-TLV, that SRGB MUST be interpreted to be the SRGB used by the tunnel's egress endpoint. (§3.7) | {gap}, no test | no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| [`RFC9012-3.7-4`](#rfc9012-3.7-4) If a Label-Index is present in the Prefix-SID sub-TLV, then when a packet is sent through the tunnel identified by the TLV, if that tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack. (§3.7) | {gap}, no test | no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-4.1-1`](#rfc9012-4.1-1) In situations where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| [`RFC9012-4.1-2`](#rfc9012-4.1-2) Notwithstanding, an implementation MUST be prepared to process a tunnel received encoded as a barebones TLV. (§4.1) | {gap}, no test | a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| [`RFC9012-4.1-3`](#rfc9012-4.1-3) Packets with other payload types MUST NOT be carried through such tunnels. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| [`RFC9012-4.2-1`](#rfc9012-4.2-1) [EVPN-INTER-SUBNET] defines a router's MAC Extended Community. This extended community, as its name implies, carries the MAC address of the advertising router. Since the VXLAN and NVGRE Encapsulation sub-TLVs can also optionally carry a router's MAC, a conflict can arise if both the Router's MAC Extended Community and such an Encapsulation sub-TLV are present at the same time but have different values. In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2) | {gap}, no test | the two received opaque values can disagree even without a decoder; ParseTunnelEncap/SubTLVs and extended-community byte preservation do not implement MAC precedence. The absent consumer is owned by plan/spec-bgp-tunnel-encapsulation-consumption.md |
| [`RFC9012-4.3-1`](#rfc9012-4.3-1) No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| [`RFC9012-6-2`](#rfc9012-6-2) Then router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U. (§6) | {gap}, no test | ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| [`RFC9012-7.1-1`](#rfc9012-7.1-1) If a route includes the Tunnel Encapsulation attribute, and if that attribute includes no tunnel that is feasible, then that route MUST NOT be considered resolvable for the purposes of the route resolvability condition ([RFC4271], Section 9.1.2.1). (§7.1) | {gap}, no test | tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-8-1`](#rfc9012-8-1) Then packet P MUST be sent through one of the tunnels identified in the Tunnel Encapsulation attribute of UPDATE U2. (§8) | {gap}, no test | a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-8-2`](#rfc9012-8-2) In that case, packet P MUST NOT be sent through the tunnel contained in that TLV, unless U1 is carrying a Color Extended Community that is identified in one of U2's Color sub-TLVs. (§8) | {gap}, no test | no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| [`RFC9012-10-1`](#rfc9012-10-1) Any document specifying such joint use MUST provide details as to how interactions should be handled. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| [`RFC9012-11-1`](#rfc9012-11-1) However, the Tunnel Encapsulation attribute MUST be used only within a well-defined scope, for example, within a set of ASes that belong to a single administrative entity. (§11) | {gap}, no test | applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go validates framing and endpoints, not administrative scope. The configured policy path has no attribute-23 name in internal/component/bgp/reactor/filter_format.go attrNameToCode; endpoint validation does not establish scope enforcement |
| [`RFC9012-11-2`](#rfc9012-11-2) To prevent the Tunnel Encapsulation attribute from being distributed beyond its intended scope, any BGP speaker that understands the attribute MUST be able to filter the attribute from incoming BGP UPDATE messages. (§11) | {gap}, no test | the built-in text policy path cannot request attribute-23 removal: internal/component/bgp/reactor/filter_delta.go textDeltaToModOps uses attrNameToCode in filter_format.go, which has no attribute-23 entry. applyTunnelEncap removes invalid endpoint TLVs, not otherwise valid attributes by session scope |
| [`RFC9012-11-5`](#rfc9012-11-5) For each external BGP (EBGP) session, filtering of the attribute on incoming UPDATEs MUST be enabled by default. (§11) | {gap}, no test | internal/component/bgp/reactor/filter_ordered.go runIngressPolicyChain accepts when no active policy is configured; internal/component/bgp/reactor/session_tunnel_encap.go applyTunnelEncap has no EBGP scope-filter decision. Receive validation is not default attribute-23 filtering |
| [`RFC9012-11-6`](#rfc9012-11-6) In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. (§11) | {gap}, no test | internal/component/bgp/reactor/filter_delta.go textDeltaToModOps has no attribute-23 policy name, and internal/component/bgp/reactor/filter_delta_handlers.go attrModHandlersWithDefaults supplies no attribute-23 handler. The generic rebuild mechanism in forward_build.go buildModifiedPayload alone does not provide the missing configured filter |
| [`RFC9012-11-8`](#rfc9012-11-8) For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | {gap}, no test | internal/component/bgp/reactor/filter_ordered.go runEgressPolicyChainASN4 accepts when no active export policy is configured; the current forwarding policy has no default attribute-23 suppression |
| [`RFC9012-11-11`](#rfc9012-11-11) * In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11) | {gap}, no test | internal/component/bgp/plugins/filter_community/filter.go applyIngressFilter strips configured values, not Encapsulation Extended Communities by default; internal/component/bgp/reactor/peer_forward_facts.go sendCommunitySuppression returns no suppression for an empty list or all. Configurable community removal does not implement this EBGP default |
| [`RFC9012-13-4`](#rfc9012-13-4) Rather, it MUST interpret the attribute as if that TLV had not been present. (§13) | {gap}, no test | the attribute-driven tunnel consumer is absent, so opaque carriage of an unknown tunnel type does not prove its exclusion from tunnel selection or header construction. applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go validates framing and endpoints, and separately enforces RFC9830 tunnel-type constraints on SR Policy carriers; those receive checks are not the absent RFC9012 consumer. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| [`RFC9012-13-6`](#rfc9012-13-6) The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| [`RFC9012-13-16`](#rfc9012-13-16) A TLV identifying a particular tunnel type may contain a sub-TLV that is meaningless for that tunnel type. For example, perhaps the TLV contains a UDP Destination Port sub-TLV, but the identified tunnel type does not use UDP encapsulation at all, or a tunnel of the form "X-in-Y" contains a Protocol Type sub-TLV that specifies something other than "X". Sub-TLVs of this sort MUST be disregarded. (§13) | {gap}, no test | no attribute-driven encapsulation consumer applies the type-specific disregard rule; signaling acceptance and unchanged propagation do not prove it. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md; owner ruling 2026-10-08: retain opaque carry/non-malformed tests under RFC9012-13-18/-19 and existing RFC9830 identities; the absent attribute-driven consumer remains owned by plan/spec-bgp-tunnel-encapsulation-consumption.md. |
| [`RFC9012-13-17`](#rfc9012-13-17) That is, they MUST NOT affect the creation of the encapsulation header. (§13) | {gap}, no test | Ze creates no encapsulation header from attribute 23, so this conditional header-construction behavior is unimplemented, not proven by propagating opaque bytes. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| [`RFC9012-13-21`](#rfc9012-13-21) If a Tunnel TLV has more than one of any of these sub-TLVs, all but the first occurrence of each such sub-TLV type MUST be disregarded. (§13) | {gap}, no test | the absent attribute-driven consumer does not implement first-occurrence interpretation for Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack or Prefix-SID. internal/core/bgp/attribute/tunnel_encap.go TunnelTLV.SubTLVs preserves every occurrence; TunnelTLV.Preference concerns a different RFC 9830 sub-TLV and does not prove this rule. On Section 6 carriers, the separate exactly-one Tunnel Egress Endpoint exception is enforced by tunnelTLVLayout and applyTunnelEncap in internal/component/bgp/reactor/session_tunnel_encap.go and remains accounted for by RFC9012-13-13/-14/-15. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |
| [`RFC9012-15-1`](#rfc9012-15-1) RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). For these purposes, traffic introduced into an SR domain using the Prefix-SID sub-TLV lies within the SR domain, even though the Prefix-SIDs used by the routers at the two ends of the tunnel may be different, as discussed in Section 3.7. This implies that the duty to filter external traffic extends to all routers participating in such tunnels. (§15) | {gap}, no test | the absent attribute-driven Prefix-SID tunnel consumer does not install this participating-router traffic filter; attribute-23 ingress/egress filtering is a different behavior. Owner: plan/spec-bgp-tunnel-encapsulation-consumption.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9012-3.1-2`](#rfc9012-3.1-2)

The Reserved subfield SHOULD be originated as zero. It MUST be disregarded on receipt, and it MUST be propagated unchanged. (§3.1) Context: this identity owns receipt-disregard of the Reserved subfield; unchanged propagation remains RFC9012-3.1-3.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §3.1: "The Reserved subfield SHOULD be originated as zero. It MUST be disregarded on receipt, and it MUST be propagated unchanged." The receipt-disregard identity is enforced: tunnelEndpointValid reads AFI after the four Reserved octets, and zero/nonzero endpoint Reserved controls both retain ActionNone, exact attribute bytes and downstream announcements. This does not credit the separately allocated origination SHOULD. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L360) | unit/verify | revert, verified |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L359) | unit/verify | revert, verified |

### [`RFC9012-3.1-3`](#rfc9012-3.1-3)

The Reserved subfield SHOULD be originated as zero. It MUST be disregarded on receipt, and it MUST be propagated unchanged. (§3.1) Context: this identity owns unchanged propagation of the Reserved subfield; receipt-disregard remains RFC9012-3.1-2.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §3.1 requires Reserved to be propagated unchanged. Nonzero endpoint Reserved octets survive receipt, default cached export, MED rebuilding and downstream delivery byte-for-byte. ParseTunnelEncap and TunnelEncap.WriteTo also retain their opaque value; an actual-output comparison, not the fabricated clean-encoding comparison alone, supplies proof. Origination is not credited. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L352) | unit/verify | revert, verified |
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L336) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L351) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L333) | unit/verify | unproven |

### [`RFC9012-3.1-4`](#rfc9012-3.1-4)

If the Address Family subfield contains the value for IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestTunnelEndpointLengthsAccepted accepts an AFI-1 endpoint containing exactly four address octets unchanged. TestRFC9012EndpointInvalidTLVsRemovedOnUnicast rejects a short AFI-1 address, removing only its TLV around a valid sibling or selecting exact TreatAsWithdraw without a survivor. tunnelEndpointValid requires a ten-octet endpoint value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L248) | unit/verify | revert, verified |
| positive | [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L402) | unit/verify | revert, verified |

### [`RFC9012-3.1-5`](#rfc9012-3.1-5)

If the Address Family subfield contains the value for IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestTunnelEndpointLengthsAccepted accepts an AFI-2 endpoint containing exactly sixteen address octets unchanged. TestRFC9012EndpointInvalidTLVsRemovedOnUnicast rejects a short AFI-2 address, removing only its TLV around a valid sibling or selecting exact TreatAsWithdraw without a survivor. tunnelEndpointValid requires a twenty-two-octet endpoint value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L249) | unit/verify | revert, verified |
| positive | [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L403) | unit/verify | revert, verified |

### [`RFC9012-3.1-7`](#rfc9012-3.1-7)

In this case, the Length field of Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestTunnelEndpointLengthsAccepted accepts AFI zero with a six-octet value and no Address subfield. TestRFC9012EndpointInvalidTLVsRemovedOnUnicast rejects an overlong AFI-zero value, removing its TLV while preserving a valid sibling or selecting exact TreatAsWithdraw without one. No next-hop ownership or reachability procedure is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L250) | unit/verify | revert, verified |
| positive | [`TestTunnelEndpointLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L404) | unit/verify | revert, verified |

### [`RFC9012-3.1-8`](#rfc9012-3.1-8)

When the Tunnel Encapsulation attribute is carried in an UPDATE message of one of the AFI/SAFIs specified in this document (see the first paragraph of Section 6), each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC9012ReceiveRemovesInstalledNativeRoutes exercises all seven Section 6 carriers through actual Session receipt and received-route storage: exactly one endpoint installs, a marked valid sibling survives missing/duplicate-endpoint TLVs, and no survivor removes only the target route before same-session reinstall. tunnelTLVLayout counts endpoints and applyTunnelEncap removes offending TLVs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L35) | unit/verify | revert, verified |

### [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3)

Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1.1-3, so no unit is bound to it.

### [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1)

The remaining bits in the 8-bit Flags field are reserved for further use. They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. RFC 9012 Sections 3.2.1 and 3.2.2 state: "The remaining bits in the 8-bit Flags field are reserved for further use. They MUST always be set to 0 by the originator of the sub-TLV." The supported SR Policy originator emits type15, not VXLAN type8 or NVGRE type9 Encapsulation sub-TLVs. Existing SR Policy field tests and intermediate-router propagation of received VXLAN/NVGRE bytes do not fill this originator role.

No test carries RFC9012-3.2.1-1, so no unit is bound to it.

### [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2)

Intermediate routers MUST propagate them without modification. (§3.2.1, §3.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §§3.2.1/3.2.2: "Intermediate routers MUST propagate them without modification." Both VXLAN type8 and NVGRE type9 occur with V/M held fixed and reserved R bits zero or set. Exact actual received and downstream values prevent masking those bits; codec round trips supplement the two cached export modes. No tunnel-header interpretation or origination is inferred. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L354) | unit/verify | revert, verified |
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L349) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L353) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L346) | unit/verify | unproven |

### [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3)

Any receiving routers MUST ignore these bits upon receipt. (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-3, so no unit is bound to it.

### [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4)

If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-4, so no unit is bound to it.

### [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5)

If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-5, so no unit is bound to it.

### [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6)

Reserved: MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-6, so no unit is bound to it.

### [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1)

Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.4-1, so no unit is bound to it.

### [`RFC9012-3.2.5-1`](#rfc9012-3.2.5-1)

Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.5)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. RFC 9012 Section 3.2.5 states: "Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present." This is the MPLS-in-GRE originator condition, distinct from GRE in Section 3.2.4. The supported parseConfigRoute/buildTunnelEncap boundary emits SR Policy tunnel type15 and its own sub-TLVs, not tunnel type11 or its Encapsulation sub-TLV. Generic serialization and received-attribute propagation are not that originator role; no receive or tunnel-consumption credit is claimed.

No test carries RFC9012-3.2.5-1, so no unit is bound to it.

### [`RFC9012-3.3-1`](#rfc9012-3.3-1)

If an outer Encapsulation sub-TLV occurs in a TLV for a tunnel type that does not use the corresponding outer encapsulation, the sub-TLV MUST be treated as if it were an unrecognized type of sub-TLV. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.3-1, so no unit is bound to it.

### [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1)

If the reserved value zero is received, the sub-TLV MUST be treated as malformed, according to the rules of Section 13. (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.3.2-1, so no unit is bound to it.

### [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2)

Packets with other payload types MUST NOT be encapsulated in the relevant tunnel. (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-2, so no unit is bound to it.

### [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3)

If the reserved value 0xFFFF is received, the sub-TLV MUST be treated as malformed according to the rules of Section 13. (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-3, so no unit is bound to it.

### [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4)

Also, for "X-in-Y" type tunnels, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored; this is discussed further in Section 13. (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-4, so no unit is bound to it.

### [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2)

If the Length field of a Color sub-TLV has a value other than 8, or the first two octets of its Value field are not 0x030b, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV (see Section 13). (§3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.2-2, so no unit is bound to it.

### [`RFC9012-3.5-1`](#rfc9012-3.5-1)

If the Tunnel Encapsulation attribute is attached to an UPDATE of a non-labeled address family, then the sub-TLV MUST be disregarded. (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-1, so no unit is bound to it.

### [`RFC9012-3.5-2`](#rfc9012-3.5-2)

If the sub-TLV is contained in a TLV whose tunnel type does not have a virtual network identifier in its encapsulation header, the sub-TLV MUST be disregarded. (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-2, so no unit is bound to it.

### [`RFC9012-3.5-3`](#rfc9012-3.5-3)

In those cases where the sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §3.5: "In those cases where the sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated." Embedded Label Handling1/2 under GRE on unlabeled IPv4 unicast supplies the applicable ignored condition. Single and duplicate inputs retain every byte through actual receipt and both cached export modes. Label interpretation/realization remains outside this non-stripping claim. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L356) | unit/verify | revert, verified |
| negative | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L373) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L355) | unit/verify | revert, verified |
| positive | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L365) | unit/verify | unproven |

### [`RFC9012-3.5-4`](#rfc9012-3.5-4)

If any value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed, according to the procedures of Section 13. (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-4, so no unit is bound to it.

### [`RFC9012-3.6-1`](#rfc9012-3.6-1)

When this label stack is pushed onto a packet, this ordering MUST be preserved. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-1, so no unit is bound to it.

### [`RFC9012-3.6-2`](#rfc9012-3.6-2)

If a packet is to be sent through the tunnel identified in a particular TLV, and if that TLV contains an MPLS Label Stack sub-TLV, then the label stack appearing in the sub-TLV MUST be pushed onto the packet before any other labels are pushed onto the packet. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-2, so no unit is bound to it.

### [`RFC9012-3.6-3`](#rfc9012-3.6-3)

In particular, if the Tunnel Encapsulation attribute is attached to a BGP UPDATE of a labeled address family, the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the label embedded in the NLRI is pushed onto the packet. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-3, so no unit is bound to it.

### [`RFC9012-3.6-4`](#rfc9012-3.6-4)

If the MPLS Label Stack sub-TLV is included in a TLV identifying a tunnel type that uses virtual network identifiers (see Section 9), the contents of the MPLS Label Stack sub-TLV MUST be pushed onto the packet before the procedures of Section 9 are applied. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-4, so no unit is bound to it.

### [`RFC9012-3.6-5`](#rfc9012-3.6-5)

The number of label stack entries in the sub-TLV MUST be determined from the Sub-TLV Length field. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-5, so no unit is bound to it.

### [`RFC9012-3.6-6`](#rfc9012-3.6-6)

When the label stack entries are pushed onto a packet that already has a label stack, the S bits of all the entries being pushed MUST be cleared. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-6, so no unit is bound to it.

### [`RFC9012-3.6-7`](#rfc9012-3.6-7)

When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-7, so no unit is bound to it.

### [`RFC9012-3.6-8`](#rfc9012-3.6-8)

When the label stack entries are pushed onto a packet that does not already have a label stack, the S bit of the bottommost label stack entry MUST be set, and the S bit of all the other label stack entries MUST be cleared. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-8, so no unit is bound to it.

### [`RFC9012-3.6-13`](#rfc9012-3.6-13)

If any label stack entry in the sub-TLV has a TTL value of zero, the router that is pushing the stack onto a packet MUST change the value to a non-zero value, either 255 or some other value as determined by policy as discussed above. (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-13, so no unit is bound to it.

### [`RFC9012-3.7-2`](#rfc9012-3.7-2)

[RFC8669] only defines behavior when the BGP Prefix-SID attribute is attached to routes of type IPv4/IPv6 Labeled Unicast [RFC4760][RFC8277], and it only defines values of the BGP Prefix-SID attribute for those cases. Therefore, similar limitations exist for the Prefix-SID sub-TLV: it SHOULD only be included in a BGP UPDATE message for one of the address families for which [RFC8669] has a defined behavior, namely BGP IPv4/IPv6 Labeled Unicast [RFC4760] [RFC8277]. If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. RFC 9012 Section 3.7 limits Prefix-SID sub-TLV inclusion to address families defined by RFC8669, namely IPv4/IPv6 labeled unicast, and states: "If included in a BGP UPDATE for any other address family, it MUST be ignored." The subject is type11 inside attribute23, not standalone attribute40. SubTLVs preserves its opaque bytes; tunnelTLVLayout processes framing, carriers and endpoints but does not implement Prefix-SID family-conditioned consumer semantics. checkRouteBestChange obtains labeled-unicast labels from the winning NLRI and SRv6 information from attribute40, not attribute23. Opaque receipt and propagation are not evidence of the missing consumer. Retain the gap under plan/spec-bgp-tunnel-encapsulation-consumption.md without an exclusion or new implementation authorization.

No test carries RFC9012-3.7-2, so no unit is bound to it.

### [`RFC9012-3.7-3`](#rfc9012-3.7-3)

If an Originator SRGB is specified in the sub-TLV, that SRGB MUST be interpreted to be the SRGB used by the tunnel's egress endpoint. (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.7-3, so no unit is bound to it.

### [`RFC9012-3.7-4`](#rfc9012-3.7-4)

If a Label-Index is present in the Prefix-SID sub-TLV, then when a packet is sent through the tunnel identified by the TLV, if that tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack. (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.7-4, so no unit is bound to it.

### [`RFC9012-4.1-1`](#rfc9012-4.1-1)

In situations where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-1, so no unit is bound to it.

### [`RFC9012-4.1-2`](#rfc9012-4.1-2)

Notwithstanding, an implementation MUST be prepared to process a tunnel received encoded as a barebones TLV. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-2, so no unit is bound to it.

### [`RFC9012-4.1-3`](#rfc9012-4.1-3)

Packets with other payload types MUST NOT be carried through such tunnels. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-3, so no unit is bound to it.

### [`RFC9012-4.2-1`](#rfc9012-4.2-1)

[EVPN-INTER-SUBNET] defines a router's MAC Extended Community. This extended community, as its name implies, carries the MAC address of the advertising router. Since the VXLAN and NVGRE Encapsulation sub-TLVs can also optionally carry a router's MAC, a conflict can arise if both the Router's MAC Extended Community and such an Encapsulation sub-TLV are present at the same time but have different values. In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. RFC 9012 Section 4.2 states: "In case of such a conflict, the information in the Router's MAC Extended Community MUST be used." The conflict is between that community and a different router MAC in a VXLAN/NVGRE Encapsulation sub-TLV. Received values can disagree even when stored opaquely. SubTLVs retains tunnel bytes, and the receive/forwarding boundary does not compare or select these MAC sources. checkRouteBestChange exports next-hop, metrics, NLRI labels and attribute40 SRv6 information, not an attribute23-derived encapsulation MAC. This is an absent consumer, not an impossible input or another role. Preserve the explicit gap under plan/spec-bgp-tunnel-encapsulation-consumption.md AC-3/AC-6; no opaque-propagation test establishes selected forwarding MAC or packet headers.

No test carries RFC9012-4.2-1, so no unit is bound to it.

### [`RFC9012-4.3-1`](#rfc9012-4.3-1)

No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.3-1, so no unit is bound to it.

### [`RFC9012-4.3-2`](#rfc9012-4.3-2)

No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver; the value MUST NOT be changed when propagating this extended community. (§4.3) Context: the Flags field of the Color Extended Community; this identity owns unchanged propagation, not the subsequent Color Value field.

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §4.3 requires the Color Extended Community Flags value not to change during propagation. Exact downstream equality includes Flags0x3fa5, not merely the Color Value, through default export and MED rebuilding. ParseExtendedCommunities and ExtendedCommunities.WriteTo copy all eight octets; actual zero/nonzero Flags round trips supplement the consumer. Origination is not credited. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L358) | unit/verify | revert, verified |
| negative | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L400) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L357) | unit/verify | revert, verified |
| positive | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L397) | unit/verify | unproven |

### [`RFC9012-6-2`](#rfc9012-6-2)

Then router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-6-2, so no unit is bound to it.

### [`RFC9012-7.1-1`](#rfc9012-7.1-1)

If a route includes the Tunnel Encapsulation attribute, and if that attribute includes no tunnel that is feasible, then that route MUST NOT be considered resolvable for the purposes of the route resolvability condition ([RFC4271], Section 9.1.2.1). (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-7.1-1, so no unit is bound to it.

### [`RFC9012-8-1`](#rfc9012-8-1)

Then packet P MUST be sent through one of the tunnels identified in the Tunnel Encapsulation attribute of UPDATE U2. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-8-1, so no unit is bound to it.

### [`RFC9012-8-2`](#rfc9012-8-2)

In that case, packet P MUST NOT be sent through the tunnel contained in that TLV, unless U1 is carrying a Color Extended Community that is identified in one of U2's Color sub-TLVs. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-8-2, so no unit is bound to it.

### [`RFC9012-10-1`](#rfc9012-10-1)

Any document specifying such joint use MUST provide details as to how interactions should be handled. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-10-1, so no unit is bound to it.

### [`RFC9012-11-1`](#rfc9012-11-1)

However, the Tunnel Encapsulation attribute MUST be used only within a well-defined scope, for example, within a set of ASes that belong to a single administrative entity. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-1, so no unit is bound to it.

### [`RFC9012-11-2`](#rfc9012-11-2)

To prevent the Tunnel Encapsulation attribute from being distributed beyond its intended scope, any BGP speaker that understands the attribute MUST be able to filter the attribute from incoming BGP UPDATE messages. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-2, so no unit is bound to it.

### [`RFC9012-11-5`](#rfc9012-11-5)

For each external BGP (EBGP) session, filtering of the attribute on incoming UPDATEs MUST be enabled by default. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-5, so no unit is bound to it.

### [`RFC9012-11-6`](#rfc9012-11-6)

In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-6, so no unit is bound to it.

### [`RFC9012-11-8`](#rfc9012-11-8)

For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-8, so no unit is bound to it.

### [`RFC9012-11-9`](#rfc9012-11-9)

* Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. (§11) Context: internal/component/bgp/plugins/filter_community/filter.go applyIngressFilter and ingressStripCommunities remove configured exact eight-octet extended-community values. This can remove a named Encapsulation Extended Community; lack of a type-wide matcher alone does not prove that no filtering capability exists. Whole-clause proof is not established here.

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9012 Section 11 says: Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. TestRFC9012ConfiguredEncapsulationECIngress configures the actual registered bgp-filter-community SDK runner with exact Encapsulation Extended Community values 030c000000000002 and 030c000000000008. Established Session receipt reaches the registered ingressFilter, applyIngressFilter and ingressStripCommunities; reactor_notify consumes the replacement payload. The removal case asserts both Encapsulation ECs are absent before dispatch, in cached ForwardUpdate socket output and after downstream Session receipt, while the unrelated RT, ORIGIN, AS_PATH, NEXT_HOP and NLRI survive. A populated no-match policy preserves the identical two Encapsulation ECs through those boundaries, distinguishing configured removal from unconditional stripping or blanket attribute removal. Both named cases passed in the retained f688dd08 race run. This proves configurable ingress filtering, not the separate default-EBGP requirement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ConfiguredEncapsulationECIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC9012ConfiguredEncapsulationECIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L93) | unit/verify | revert, verified |

### [`RFC9012-11-10`](#rfc9012-11-10)

It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) Context: internal/component/bgp/plugins/filter_community/egress.go applyEgressFilter records configured exact-value removals, and handler.go genericCommunityHandler applies them. internal/component/bgp/reactor/peer_forward_facts.go applySendCommunityMask can also suppress the entire Extended Communities attribute. These mechanisms are not an EBGP default or independent whole-clause proof.

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9012 Section 11 says: It MUST be possible to filter the Encapsulation Extended Community from outgoing messages. TestRFC9012ConfiguredEncapsulationECEgress configures the actual registered bgp-filter-community SDK runner with exact Encapsulation Extended Community values 030c000000000002 and 030c000000000008. Both values remain present at source receive dispatch; the configured destination's registered egressFilter and applyEgressFilter produce removal operations consumed by the registered Extended Communities handler and genericCommunityHandler during actual ForwardUpdate export. Socket output and downstream Session receipt contain only the unrelated RT, preserving ORIGIN, AS_PATH, NEXT_HOP and NLRI. A destination with a populated no-match policy receives both original Encapsulation ECs, distinguishing configured destination filtering from unconditional removal. Both named cases passed in the retained f688dd08 race run. This proves configurable outgoing filtering without requiring a type-wide matcher or claiming the separate default-EBGP behavior.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ConfiguredEncapsulationECEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC9012ConfiguredEncapsulationECEgress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_encapsulation_filter_test.go#L104) | unit/verify | revert, verified |

### [`RFC9012-11-11`](#rfc9012-11-11)

* In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-11, so no unit is bound to it.

### [`RFC9012-13-1`](#rfc9012-13-1)

The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudgment against RFC9012 Section 13: the accepted control ends exactly at the tunnel boundary; malformed short/long sub-TLV headers and values now follow a valid endpoint and retain exact TreatAsWithdraw assertions. The old eight-case unit survived selective omission of tunnelTLVLayout end > size (job-enum-framing-blindspot-before-da6b72ea.log); after fixture isolation the identical omission failed both truncated-value cases with expected 2, actual 0 (job-enum-framing-boundary-after-red-da6b72ea.log). The real-producer race smoke passed all eight cases and endpoint-removal/composition/length/reset siblings (job-enum-framing-repaired-race-smoke-5388cca1.log). This proves structural boundary discrimination, not separate mutation observations for every header guard or daemon interoperability.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L308) | unit/verify | revert, verified |
| positive | [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L307) | unit/verify | revert, verified |

### [`RFC9012-13-2`](#rfc9012-13-2)

The final octet of a TLV MUST also be the final octet of its final sub-TLV. If this is not the case, the TLV MUST be considered to be malformed, and the "Treat-as-withdraw" procedure of [RFC7606] is applied. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Framing controls isolate outer and short/long sub-TLV boundary violations against valid endpoints. TestRFC9012ReceiveRemovesInstalledNativeRoutes now supplements the exact-action root with actual target removal, independent-route preservation and same-session reinstall on all seven Section 6 carriers. The prior classification-only and missing-carrier objections no longer describe the tagged test set.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L31) | unit/verify | revert, verified |
| negative | [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L310) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC9012TunnelFramingReceiveVerdicts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L309) | unit/verify | revert, verified |

### [`RFC9012-13-3`](#rfc9012-13-3)

If a Tunnel Encapsulation attribute can be parsed correctly but contains a TLV whose tunnel type is not recognized by a particular BGP speaker, that BGP speaker MUST NOT consider the attribute to be malformed. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 forbids calling a correctly framed unknown tunnel type malformed. Isolated0xfffe and type2 controls share valid endpoint framing and require exact ActionNone, attribute retention and downstream delivery. applyTunnelEncap restricts tunnel types only on the separately governed SR-Policy SAFI73, not this ordinary-unicast carrier. Genuine outer-framing rejection controls remain. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L335) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L150) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L334) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L115) | unit/verify | unproven |

### [`RFC9012-13-4`](#rfc9012-13-4)

Rather, it MUST interpret the attribute as if that TLV had not been present. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-4, so no unit is bound to it.

### [`RFC9012-13-5`](#rfc9012-13-5)

If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized TLV MUST remain in the attribute. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13: "If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized TLV MUST remain in the attribute." Isolated/combined unknown tunnel TLVs remain byte-identical before forwarding, on default/rebuilt cached socket output and at downstream receipt. Codec round trip is supplementary; the separate constructed known-only comparison is not credited as independent producer-negative proof. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L337) | unit/verify | revert, verified |
| negative | [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L138) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L336) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L124) | unit/verify | unproven |

### [`RFC9012-13-6`](#rfc9012-13-6)

The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-6, so no unit is bound to it.

### [`RFC9012-13-8`](#rfc9012-13-8)

However, the Tunnel TLV containing them MUST NOT be considered to be malformed (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 requires duplicate single-instance sub-TLVs not to make their Tunnel TLV malformed and requires all sub-TLVs to propagate. Duplicate Embedded Label Handling, a type actually named by this section, and its single control retain exact ActionNone and bytes with one valid endpoint. Preference is not substituted for this applicable ELH case. First-occurrence semantic consumption remains a separate explicit gap. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L339) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L160) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L338) | unit/verify | revert, verified |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269) | unit/verify | unproven |

### [`RFC9012-13-9`](#rfc9012-13-9)

all the sub-TLVs MUST be propagated if the route carrying the Tunnel Encapsulation attribute is propagated. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 requires every duplicate single-instance sub-TLV to propagate. Both ELH occurrences remain in order and byte-identical in actual downstream output under default and MED-rebuilt cached export; removing either would fail those comparisons. Exact ActionNone observes the companion classification clause. Fabricated shortened codec encodings alone are not credited, and no first-occurrence interpretation is inferred. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L341) | unit/verify | revert, verified |
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L193) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L340) | unit/verify | revert, verified |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L179) | unit/verify | unproven |

### [`RFC9012-13-10`](#rfc9012-13-10)

If a TLV of a Tunnel Encapsulation attribute contains a sub-TLV that is not recognized by a particular BGP speaker, the BGP speaker MUST process that TLV as if the unrecognized sub-TLV had not been present. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC9012 Section 13 requires the containing TLV to be processed as if an unrecognized sub-TLV were absent. TestRFC9012UnknownSubTLVEndpointProcessing now compares absent controls with framed short- and long-header unknowns before, after and between endpoints across 27 cases: valid, length-invalid, registry-disallowed, duplicate and missing endpoints. It observes exact retained TLVs through original-input Session receipt, cache/export, default and MED-rebuilt socket output and downstream receipt; real received-route storage observes replacement, removal with no surviving TLV, an unaffected independent route and same-session recovery. A selective tunnelTLVLayout break stopping at unknown types 100/200 leaves absent controls passing but fails exact exported bytes and installed-target removal, including duplicate-endpoint cases with an intervening unknown; restored claim roots pass. Existing carry assertions retain admission and opaque-propagation proof. TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing separately proves local Preference accessor equivalence and absence of a Preference in an all-unknown TLV, not SRPM selection. The positive-only annotation is appropriate: unknown types are legal inputs and surrounding endpoint errors are controls for processing equivalence, not invalid unknown types. This whole conditional receive/transit obligation is enforced without claiming the absent attribute-driven tunnel-selection, encapsulation-header or SRPM consumers, which retain their separate ownership and gaps.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L342) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnknownSubTLVEndpointProcessing`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L648) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L212) | unit/verify | revert, verified |

### [`RFC9012-13-11`](#rfc9012-13-11)

If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized sub-TLV MUST remain in the attribute. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 requires an unrecognized sub-TLV to remain when its attribute propagates. Isolated long-header type200 under GRE survives actual receipt, default cached export, MED rebuild and downstream receipt exactly. Actual codec round trips supplement short/long-header coverage; fabricated shorter encodings alone are not negative evidence. tunnelTLVLayout walks non-endpoint sub-TLVs by their declared framing without stripping unknown types. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L344) | unit/verify | revert, verified |
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L196) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L343) | unit/verify | revert, verified |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L186) | unit/verify | unproven |

### [`RFC9012-13-12`](#rfc9012-13-12)

In general, if a TLV contains a sub-TLV that is malformed, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 generally treats a malformed sub-TLV as unrecognized, with the explicit malformed-endpoint exception retained. Applicable MPLS-in-UDP type13 inputs with UDP length3 or value0, against absent/valid controls, retain ActionNone and exact bytes through actual default/rebuilt cached output like unknown sub-TLVs. TunnelTLV.Preference skips malformed four-byte Preference and reads a following valid six-byte777, with standalone555 control. No UDP-header or tunnel-consumer feature is claimed. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L346) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L249) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L345) | unit/verify | revert, verified |
| positive | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L240) | unit/verify | unproven |

### [`RFC9012-13-13`](#rfc9012-13-13)

if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Structural endpoint cases, original-input registry propagation and real native-route storage jointly prove that malformed endpoint TLVs are ignored wholesale. Valid marked siblings survive; all-invalid replacements remove the installed target on all seven Section 6 carriers. Registry Destination/Forwardable restrictions and allowed exceptions are covered. The optional origin-AS determination is not implemented or inferred, so no unproved determination is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L105) | unit/verify | revert, verified |
| negative | [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L556) | unit/verify | revert, verified |
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L252) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L555) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L251) | unit/verify | revert, verified |

### [`RFC9012-13-14`](#rfc9012-13-14)

There is one exception to this rule: if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation attribute before the route carrying that attribute is distributed. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The current tagged set proves structural and registry-semantic malformed-endpoint removal, original-input receive dispatch, exact raw/rebuilt downstream output and real received-route storage across all seven Section 6 carriers. Marked valid siblings and independent routes survive; all-invalid replacements withdraw only the target and permit same-session recovery. applyRFC7606 consumes the rewritten body and attributes, and processValidatedMessage dispatches synthesized withdrawals. Optional origin-AS validation is neither provided nor claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L101) | unit/verify | revert, verified |
| negative | [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L553) | unit/verify | revert, verified |
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L246) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointAddressRegistryStorage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointAddressRegistryPropagation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L552) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L245) | unit/verify | revert, verified |

### [`RFC9012-13-15`](#rfc9012-13-15)

Within a Tunnel Encapsulation attribute that is carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Missing and duplicate endpoint TLVs are now exercised through actual Session receipt and real received-route storage for every Section 6 AFI/SAFI, with a causal marked sibling, an independent control route, actual target withdrawal and same-session reinstall. Existing unicast raw/rebuilt forwarding assertions remain complementary. The prior missing-carrier and classification-only objections are closed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReceiveRemovesInstalledNativeRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_receive_storage_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC9012EndpointInvalidTLVsRemovedOnUnicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9830_tunnel_receive_test.go#L243) | unit/verify | revert, verified |

### [`RFC9012-13-16`](#rfc9012-13-16)

A TLV identifying a particular tunnel type may contain a sub-TLV that is meaningless for that tunnel type. For example, perhaps the TLV contains a UDP Destination Port sub-TLV, but the identified tunnel type does not use UDP encapsulation at all, or a tunnel of the form "X-in-Y" contains a Protocol Type sub-TLV that specifies something other than "X". Sub-TLVs of this sort MUST be disregarded. (§13)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Section 13 defines disregard of tunnel-type-meaningless sub-TLVs in terms of encapsulation-header construction. Existing opaque carry and Preference-readback tests do not observe that consumer. checkRouteBestChange exports no attribute-23 tunnel parameters; the absent consumer remains owned by plan/spec-bgp-tunnel-encapsulation-consumption.md AC-2/AC-6. The applied owner-ruling annotation and removal of the four obsolete 13-16 carry tags preserve their assertions and existing 13-18/-19/RFC9830 identities. Preserve the complete quote and MUST applicability, with no whole-requirement credit.

No test carries RFC9012-13-16, so no unit is bound to it.

### [`RFC9012-13-17`](#rfc9012-13-17)

That is, they MUST NOT affect the creation of the encapsulation header. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-17, so no unit is bound to it.

### [`RFC9012-13-18`](#rfc9012-13-18)

However, the sub-TLV MUST NOT be considered to be malformed (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 forbids classifying a meaningless sub-TLV as malformed or removing it. Well-formed two-byte nonzero UDP Destination Port under GRE isolates meaninglessness from bad length/value; it receives ActionNone and survives exact actual downstream propagation. GRE-without-UDP and applicable-valid UDP are controls; malformed outer framing remains rejected. This classification identity does not establish encapsulation-header behavior. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L348) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L347) | unit/verify | revert, verified |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L295) | unit/verify | revert, verified |

### [`RFC9012-13-19`](#rfc9012-13-19)

the sub-TLV MUST NOT be considered to be malformed and MUST NOT be removed from the TLV before the route carrying the Tunnel Encapsulation attribute is distributed. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent FinalTunnelRejudgment retained this allocated obligation; Main read RFC9012 full text, session_tunnel_encap.go::applyTunnelEncap/tunnelTLVLayout/tunnelEndpointValid, tunnel_encap.go codec producers and the real consumer. RFC9012 §13 forbids removing a meaningless sub-TLV before distribution. UDP-in-GRE retains exact bytes through default and MED-rebuilt cached output and downstream delivery, with ActionNone and applicable/absent controls. Codec round trip supplements retention; constructed shorter encodings alone are not independent negative evidence. The separately owned13-16/13-17 consumer gaps remain unchanged. Actual consumer: internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go::TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong and teForwardedAttrs drive Session.processMessage, cache publication, ForwardUpdate, worker/socket output and downstream Session receipt. These are two cached export modes, not an independently exercised reactor-native RS rail; no absent attribute-driven tunnel-selection feature is credited.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L350) | unit/verify | revert, verified |
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L319) | unit/verify | revert, verified |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L349) | unit/verify | revert, verified |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L316) | unit/verify | revert, verified |

### [`RFC9012-13-21`](#rfc9012-13-21)

If a Tunnel TLV has more than one of any of these sub-TLVs, all but the first occurrence of each such sub-TLV type MUST be disregarded. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-21, so no unit is bound to it.

### [`RFC9012-15-1`](#rfc9012-15-1)

RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). For these purposes, traffic introduced into an SR domain using the Prefix-SID sub-TLV lies within the SR domain, even though the Prefix-SIDs used by the routers at the two ends of the tunnel may be different, as discussed in Section 3.7. This implies that the duty to filter external traffic extends to all routers participating in such tunnels. (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-15-1, so no unit is bound to it.

### [`RFC9012-11-7`](#rfc9012-11-7)

This filtering SHOULD be possible on a per-BGP-session basis. (§11) Context: outgoing Tunnel Encapsulation attribute filtering; RFC9012-11-6 owns the preceding filter-capability MUST.

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 9012 §11 says, “In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. This filtering SHOULD be possible on a per-BGP-session basis.” This identity owns the per-session SHOULD, not the preceding capability MUST or default EBGP filtering. TestRFC9012TunnelFilterSelectedSession and TestRFC9012TunnelFilterDoesNotAffectOtherSession pass one valid received attribute-23 announcement through production Session receipt, dispatch, the recent-update cache and one ForwardUpdate fanout to two established-session unit fixtures. Each destination has independently populated ExportFilters and refreshed forwarding facts; the second test swaps which destination strips the attribute. The external-plugin transport fixture transforms the actual raw request produced by policyFilterFunc, removing only attribute 23 rather than returning a precomputed expected body. Production PolicyFilterChain, runEgressPolicyChainASN4 and the destination-specific forwarding loop apply its response. Both production session writers write through recordingConn, and the captured bodies are consumed by downstream Session.processMessage. Exact assertions require removal only for the configured destination, full original attribute-23 preservation for its sibling, unchanged NLRI and unrelated attribute encodings except mandatory IBGP LOCAL_PREF, no synthesized withdrawal, exactly one downstream dispatch, unchanged downstream payloads, and preservation of both the retained cached source and original receive buffer. Both unchanged tagged tests passed twenty race-enabled iterations. A selective omission of the raw replacement for destination 192.0.2.2 failed the selected-session test's actual output and downstream attribute-23 removal assertions while the swapped test passed; a separate selective substitution of the stripping chain for that destination failed the swapped test's sibling-preservation assertions while the selected-session test passed. These observed semantic failures demonstrate removal and session isolation separately. Both exact tags also have native revert records in rfc/discrimination/rfc9012.json bound to filter_ordered.go::runEgressPolicyChainASN4; their producer-halting failures establish binding and are not substituted for the semantic cuts. Evidence is bounded to production unit-level session-writer/downstream-receive paths using recordingConn and a simulated external-plugin transport: it is not a live external plugin, daemon, TCP-socket, FRR interoperability or tunnel-dataplane result.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelFilterDoesNotAffectOtherSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_session_filter_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestRFC9012TunnelFilterSelectedSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_session_filter_test.go#L37) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc9012.txt |
| Source fingerprint | 0d9a9771610007d7 |
| Record | rfc/extraction/rfc9012.json |
| Mapped sentences | 70 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `1.5` | not stated | 0 | walked | not stated |
| `1.6` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 5 | walked | not stated |
| `3.1.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 6 | walked | not stated |
| `3.2.2` | not stated | 6 | walked | not stated |
| `3.2.3` | not stated | 0 | walked | not stated |
| `3.2.4` | not stated | 1 | walked | not stated |
| `3.2.5` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 3 | walked | not stated |
| `3.4.2` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 4 | walked | not stated |
| `3.6` | not stated | 8 | walked | not stated |
| `3.7` | not stated | 3 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 1 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.2.1` | not stated | 0 | walked | not stated |
| `9.2.2` | not stated | 0 | walked | not stated |
| `9.2.2.1` | not stated | 0 | walked | not stated |
| `9.2.2.2` | not stated | 0 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 8 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 16 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 0 | walked | not stated |
| `14.2` | not stated | 0 | walked | not stated |
| `14.3` | not stated | 0 | walked | not stated |
| `14.4` | not stated | 0 | walked | not stated |
| `14.5` | not stated | 0 | walked | not stated |
| `14.6` | not stated | 0 | walked | not stated |
| `14.7` | not stated | 0 | walked | not stated |
| `14.8` | not stated | 0 | walked | not stated |
| `14.9` | not stated | 0 | walked | not stated |
| `14.10` | not stated | 0 | walked | not stated |
| `15` | not stated | 1 | walked | not stated |
| `16` | not stated | 0 | walked | not stated |
| `16.1` | not stated | 0 | walked | not stated |
| `16.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | They MUST always be set to 0 by the originator of the sub-TLV. |
| `3.2.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | Intermediate routers MUST propagate them without modification. |
| `3.2.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | Any receiving routers MUST ignore these bits upon receipt. |
| `3.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. |
| `3.2.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. |
| `3.2.2:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.2 (NVGRE Encapsulation sub-TLV) repeats Section 3.2.1's flag and field rules word for word; the mapped row states the obligation for both sub-TLVs and cites both sections. | Reserved: MUST be set to zero on transmission and disregarded on receipt. |

## Superseded

No document obsoletes RFC 9012, so its obligations are stated where they were written.
