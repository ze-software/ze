# RFC 9012 - The BGP Tunnel Encapsulation Attribute

Partial. Every requirement this repository extracted from RFC 9012, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 18.9% | 14 of 74 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 74 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 74 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 74 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 50.0% | 28 of 56 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 74 | of 96 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 74 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.2% | 9 of 74 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 74 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 74 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 68.9% | 51 of 74 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 74 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 96 |
| Gated MUST-level | 74 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 51 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 56 |
| Tagged units | 56 |
| Recorded audit verdicts | 14 |
| Discrimination records | 28 |
| Summary | `rfc/short/rfc9012.md` |
| Requirement shard | `rfc/requirements/rfc9012.md` |
| RFC text | `rfc/full/rfc9012.txt` |

## Enrolment

Enrolled: The BGP Tunnel Encapsulation Attribute

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Attribute code 23 parses into Tunnel Type TLVs whose values are kept as raw bytes ([`internal/core/bgp/attribute/tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/tunnel_encap.go))
- sub-TLVs are walked on demand with the 1-octet or 2-octet length header the type value selects (tunnel_encap.go)
- the RFC 9830 Preference sub-TLV is decoded at its mandated 6-octet length (tunnel_encap.go)
- and every TLV and sub-TLV -- recognized, unrecognized, meaningless for its tunnel type, malformed or duplicated -- is re-advertised byte for byte (tunnel_encap.go). ze originates the attribute for SR Policy: tunnel type 15 carrying preference, MPLS and SRv6 binding SID, priority, weighted segment lists and the policy and candidate-path names ([`internal/component/bgp/plugins/nlri/srpolicy/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/config.go)). Requirements bound per line in [`rfc/short/rfc9012.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9012.md).


**What the ledger says remains**

51 MUST-level gaps annotated in [`rfc/short/rfc9012.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9012.md). No sub-TLV other than Preference is decoded, so every obligation attached to one is unmet: Tunnel Egress Endpoint ([`RFC9012-3.1-2`](#rfc9012-3.1-2), [`RFC9012-3.1-4`](#rfc9012-3.1-4), [`RFC9012-3.1-5`](#rfc9012-3.1-5), [`RFC9012-3.1-7`](#rfc9012-3.1-7), [`RFC9012-3.1-8`](#rfc9012-3.1-8), [`RFC9012-13-13`](#rfc9012-13-13), [`RFC9012-13-14`](#rfc9012-13-14), [`RFC9012-13-15`](#rfc9012-13-15)); VXLAN and NVGRE Encapsulation ([`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3), [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4), [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5), [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6), [`RFC9012-3.3-1`](#rfc9012-3.3-1)); UDP Destination Port ([`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1)); Protocol Type ([`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2), [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3), [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4)); Color sub-TLV ([`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2)); Embedded Label Handling ([`RFC9012-3.5-1`](#rfc9012-3.5-1), [`RFC9012-3.5-2`](#rfc9012-3.5-2), [`RFC9012-3.5-4`](#rfc9012-3.5-4)); MPLS Label Stack ([`RFC9012-3.6-1`](#rfc9012-3.6-1), [`RFC9012-3.6-2`](#rfc9012-3.6-2), [`RFC9012-3.6-3`](#rfc9012-3.6-3), [`RFC9012-3.6-4`](#rfc9012-3.6-4), [`RFC9012-3.6-5`](#rfc9012-3.6-5), [`RFC9012-3.6-6`](#rfc9012-3.6-6), [`RFC9012-3.6-7`](#rfc9012-3.6-7), [`RFC9012-3.6-8`](#rfc9012-3.6-8), [`RFC9012-3.6-13`](#rfc9012-3.6-13)); Prefix-SID ([`RFC9012-3.7-2`](#rfc9012-3.7-2), [`RFC9012-3.7-3`](#rfc9012-3.7-3), [`RFC9012-3.7-4`](#rfc9012-3.7-4)). No tunnel named by the attribute reaches forwarding, so tunnel selection, resolvability and encapsulation obligations are unmet ([`RFC9012-4.1-2`](#rfc9012-4.1-2), [`RFC9012-6-2`](#rfc9012-6-2), [`RFC9012-7.1-1`](#rfc9012-7.1-1), [`RFC9012-8-1`](#rfc9012-8-1), [`RFC9012-8-2`](#rfc9012-8-2), [`RFC9012-13-4`](#rfc9012-13-4), [`RFC9012-13-17`](#rfc9012-13-17)). Sub-TLV framing inside a TLV is never validated, so a TLV whose final sub-TLV overruns it is accepted instead of treated as withdraw ([`RFC9012-13-1`](#rfc9012-13-1), [`RFC9012-13-2`](#rfc9012-13-2)). The attribute and the Encapsulation Extended Community cannot be filtered per session or by default on EBGP sessions ([`RFC9012-11-1`](#rfc9012-11-1), [`RFC9012-11-2`](#rfc9012-11-2), [`RFC9012-11-5`](#rfc9012-11-5), [`RFC9012-11-6`](#rfc9012-11-6), [`RFC9012-11-8`](#rfc9012-11-8), [`RFC9012-11-9`](#rfc9012-11-9), [`RFC9012-11-10`](#rfc9012-11-10), [`RFC9012-11-11`](#rfc9012-11-11), [`RFC9012-15-1`](#rfc9012-15-1)). Nine further MUSTs are annotated not-applicable: ze originates no VXLAN, NVGRE, GRE or single-instance sub-TLV and neither reads nor writes the Encapsulation, Router's MAC or Color Extended Communities ([`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3), [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1), [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1), [`RFC9012-4.1-1`](#rfc9012-4.1-1), [`RFC9012-4.1-3`](#rfc9012-4.1-3), [`RFC9012-4.2-1`](#rfc9012-4.2-1), [`RFC9012-4.3-1`](#rfc9012-4.3-1), [`RFC9012-10-1`](#rfc9012-10-1), [`RFC9012-13-6`](#rfc9012-13-6)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the gated population |
| Annotated (including scoped evidence) | 60 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **74** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (14):** [`RFC9012-3.1-3`](#rfc9012-3.1-3), [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2), [`RFC9012-3.5-3`](#rfc9012-3.5-3), [`RFC9012-4.3-2`](#rfc9012-4.3-2), [`RFC9012-13-3`](#rfc9012-13-3), [`RFC9012-13-5`](#rfc9012-13-5), [`RFC9012-13-8`](#rfc9012-13-8), [`RFC9012-13-9`](#rfc9012-13-9), [`RFC9012-13-10`](#rfc9012-13-10), [`RFC9012-13-11`](#rfc9012-13-11), [`RFC9012-13-12`](#rfc9012-13-12), [`RFC9012-13-16`](#rfc9012-13-16), [`RFC9012-13-18`](#rfc9012-13-18), [`RFC9012-13-19`](#rfc9012-13-19)

**Annotated (including scoped evidence) (60):** [`RFC9012-3.1-2`](#rfc9012-3.1-2), [`RFC9012-3.1-4`](#rfc9012-3.1-4), [`RFC9012-3.1-5`](#rfc9012-3.1-5), [`RFC9012-3.1-7`](#rfc9012-3.1-7), [`RFC9012-3.1-8`](#rfc9012-3.1-8), [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3), [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1), [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3), [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4), [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5), [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6), [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1), [`RFC9012-3.3-1`](#rfc9012-3.3-1), [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1), [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2), [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3), [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4), [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2), [`RFC9012-3.5-1`](#rfc9012-3.5-1), [`RFC9012-3.5-2`](#rfc9012-3.5-2), [`RFC9012-3.5-4`](#rfc9012-3.5-4), [`RFC9012-3.6-1`](#rfc9012-3.6-1), [`RFC9012-3.6-2`](#rfc9012-3.6-2), [`RFC9012-3.6-3`](#rfc9012-3.6-3), [`RFC9012-3.6-4`](#rfc9012-3.6-4), [`RFC9012-3.6-5`](#rfc9012-3.6-5), [`RFC9012-3.6-6`](#rfc9012-3.6-6), [`RFC9012-3.6-7`](#rfc9012-3.6-7), [`RFC9012-3.6-8`](#rfc9012-3.6-8), [`RFC9012-3.6-13`](#rfc9012-3.6-13), [`RFC9012-3.7-2`](#rfc9012-3.7-2), [`RFC9012-3.7-3`](#rfc9012-3.7-3), [`RFC9012-3.7-4`](#rfc9012-3.7-4), [`RFC9012-4.1-1`](#rfc9012-4.1-1), [`RFC9012-4.1-2`](#rfc9012-4.1-2), [`RFC9012-4.1-3`](#rfc9012-4.1-3), [`RFC9012-4.2-1`](#rfc9012-4.2-1), [`RFC9012-4.3-1`](#rfc9012-4.3-1), [`RFC9012-6-2`](#rfc9012-6-2), [`RFC9012-7.1-1`](#rfc9012-7.1-1), [`RFC9012-8-1`](#rfc9012-8-1), [`RFC9012-8-2`](#rfc9012-8-2), [`RFC9012-10-1`](#rfc9012-10-1), [`RFC9012-11-1`](#rfc9012-11-1), [`RFC9012-11-2`](#rfc9012-11-2), [`RFC9012-11-5`](#rfc9012-11-5), [`RFC9012-11-6`](#rfc9012-11-6), [`RFC9012-11-8`](#rfc9012-11-8), [`RFC9012-11-9`](#rfc9012-11-9), [`RFC9012-11-10`](#rfc9012-11-10), [`RFC9012-11-11`](#rfc9012-11-11), [`RFC9012-13-1`](#rfc9012-13-1), [`RFC9012-13-2`](#rfc9012-13-2), [`RFC9012-13-4`](#rfc9012-13-4), [`RFC9012-13-6`](#rfc9012-13-6), [`RFC9012-13-13`](#rfc9012-13-13), [`RFC9012-13-14`](#rfc9012-13-14), [`RFC9012-13-15`](#rfc9012-13-15), [`RFC9012-13-17`](#rfc9012-13-17), [`RFC9012-15-1`](#rfc9012-15-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9012-3.1-2` | It MUST be disregarded on receipt (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code decodes the Tunnel Egress Endpoint sub-TLV, so ze holds no Reserved subfield to disregard: SubTLVs returns raw type and value pairs (internal/core/bgp/attribute/tunnel_encap.go:104) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.1-3` | it MUST be propagated unchanged (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L333). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L174). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L336). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L175) |
| `RFC9012-3.1-4` | If the Address Family subfield contains the value for IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix). (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv4 Address Family carrying an address of any other width is accepted unchanged |
| `RFC9012-3.1-5` | If the Address Family subfield contains the value for IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix). (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv6 Address Family carrying an address of any other width is accepted unchanged |
| `RFC9012-3.1-7` | In this case, the Length field of Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06). (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Length of a Tunnel Egress Endpoint sub-TLV is never checked against its Address Family: SubTLVs reads the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.1-8` | When the Tunnel Encapsulation attribute is carried in an UPDATE message of one of the AFI/SAFIs specified in this document (see the first paragraph of Section 6), each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing counts Tunnel Egress Endpoint sub-TLVs per TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), so a TLV with none or with several is accepted identically |
| `RFC9012-3.1.1-3` | Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-3.2.1-1` | They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no VXLAN or NVGRE Encapsulation sub-TLV, so it sets no R bits: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encapsulation sub-?tlv\|buildEncap' over internal, pkg and cmd finds no Encapsulation sub-TLV builder |
| `RFC9012-3.2.1-2` | Intermediate routers MUST propagate them without modification. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L344). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L176). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L347). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L177) |
| `RFC9012-3.2.1-3` | Any receiving routers MUST ignore these bits upon receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-4` | If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-5` | If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-6` | Reserved: MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.4-1` | Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4, §3.2.5) | MUST NOT | 3.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no GRE or MPLS-in-GRE Encapsulation sub-TLV and has no GRE key to advertise in BGP: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331); the only GRE key in ze belongs to interface tunnels (internal/plugins/iface/netlink/tunnel_linux.go:35) |
| `RFC9012-3.3-1` | If an outer Encapsulation sub-TLV occurs in a TLV for a tunnel type that does not use the corresponding outer encapsulation, the sub-TLV MUST be treated as if it were an unrecognized type of sub-TLV. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze recognizes no Encapsulation sub-TLV under any tunnel type, so it never makes the tunnel-type-specific determination this requires: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.3.2-1` | If the reserved value zero is received, the sub-TLV MUST be treated as malformed, according to the rules of Section 13. (§3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no UDP Destination Port sub-TLV decoder exists, so the reserved value zero is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-2` | Packets with other payload types MUST NOT be encapsulated in the relevant tunnel. (§3.4.1) | MUST NOT | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no tunnel encapsulation from the attribute, so no payload type is ever checked against a Protocol Type sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the FIB receives no tunnel from BGP (grep for SubTLVs over internal/plugins/fib matches nothing) |
| `RFC9012-3.4.1-3` | If the reserved value 0xFFFF is received, the sub-TLV MUST be treated as malformed according to the rules of Section 13. (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Protocol Type sub-TLV decoder exists, so the reserved value 0xFFFF is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-4` | Also, for "X-in-Y" type tunnels, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored; this is discussed further in Section 13. (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Protocol Type sub-TLV and forms no X-in-Y tunnel, so a mismatched payload type is never identified: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.4.2-2` | If the Length field of a Color sub-TLV has a value other than 8, or the first two octets of its Value field are not 0x030b, the sub-TLV MUST be treated as if it were an unrecognized sub-TLV (see Section 13). (§3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV decoder exists, so neither its Length nor its leading 0x030b octets are checked: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-1` | If the Tunnel Encapsulation attribute is attached to an UPDATE of a non-labeled address family, then the sub-TLV MUST be disregarded. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Embedded Label Handling sub-TLV is never decoded and the attribute is never correlated with the address family of the UPDATE carrying it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-2` | If the sub-TLV is contained in a TLV whose tunnel type does not have a virtual network identifier in its encapsulation header, the sub-TLV MUST be disregarded. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code relates a sub-TLV to whether its tunnel type uses a virtual network identifier: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-3` | In those cases where the sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated. (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L362). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L178). **negative:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L370). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L179) |
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
| `RFC9012-3.7-2` | If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Prefix-SID sub-TLV of the Tunnel Encapsulation attribute is never decoded, so it is neither used nor deliberately ignored per address family: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.7-3` | If an Originator SRGB is specified in the sub-TLV, that SRGB MUST be interpreted to be the SRGB used by the tunnel's egress endpoint. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| `RFC9012-3.7-4` | If a Label-Index is present in the Prefix-SID sub-TLV, then when a packet is sent through the tunnel identified by the TLV, if that tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-4.1-1` | In situations where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| `RFC9012-4.1-2` | Notwithstanding, an implementation MUST be prepared to process a tunnel received encoded as a barebones TLV. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| `RFC9012-4.1-3` | Packets with other payload types MUST NOT be carried through such tunnels. (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| `RFC9012-4.2-1` | In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes neither side of the conflict into a MAC address, so the two can never disagree. An extended community reaches ze only as 8 opaque octets that are parsed in and re-encoded out unchanged (internal/core/bgp/attribute/community.go:275, :250) with no Router's MAC decoder -- grep -rniE "router.?s? mac\|RouterMAC" over internal, pkg and cmd matches only the VRRP virtual MAC (internal/plugins/vrrp/packet/packet.go:94) -- and the only Tunnel Encapsulation sub-TLV ze decodes is Preference (TunnelTLV.Preference, internal/core/bgp/attribute/tunnel_encap.go:145), never a VXLAN or NVGRE Encapsulation sub-TLV |
| `RFC9012-4.3-1` | No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| `RFC9012-4.3-2` | the value MUST NOT be changed when propagating this extended community. (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L394). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L180). **negative:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L397). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L181) |
| `RFC9012-6-2` | Then router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| `RFC9012-7.1-1` | If a route includes the Tunnel Encapsulation attribute, and if that attribute includes no tunnel that is feasible, then that route MUST NOT be considered resolvable for the purposes of the route resolvability condition ([RFC4271], Section 9.1.2.1). (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-8-1` | Then packet P MUST be sent through one of the tunnels identified in the Tunnel Encapsulation attribute of UPDATE U2. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-8-2` | In that case, packet P MUST NOT be sent through the tunnel contained in that TLV, unless U1 is carrying a Color Extended Community that is identified in one of U2's Color sub-TLVs. (§8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| `RFC9012-10-1` | Any document specifying such joint use MUST provide details as to how interactions should be handled. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| `RFC9012-11-1` | However, the Tunnel Encapsulation attribute MUST be used only within a well-defined scope, for example, within a set of ASes that belong to a single administrative entity. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze enforces no scope for the attribute: it is parsed from any peer and re-advertised unchanged (a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151)), and no configuration bounds where it may travel (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-2` | To prevent the Tunnel Encapsulation attribute from being distributed beyond its intended scope, any BGP speaker that understands the attribute MUST be able to filter the attribute from incoming BGP UPDATE messages. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filter can drop attribute 23 on ingress: setBlockAllowedKeys is a closed set of modifier keys with no tunnel-encapsulation entry (internal/component/bgp/plugins/filter_modify/config.go:30) and communityDirectives covers only the three community attributes (internal/component/bgp/reactor/filter_delta.go:268) |
| `RFC9012-11-5` | For each external BGP (EBGP) session, filtering of the attribute on incoming UPDATEs MUST be enabled by default. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-6` | In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filter can drop attribute 23 on egress: the egress path rebuilds attributes through AttrModHandlers keyed by attribute code (internal/component/bgp/reactor/forward_build.go:211) and no handler or directive names code 23 (internal/component/bgp/reactor/filter_delta.go:268) |
| `RFC9012-11-8` | For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no outgoing attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-9` | * Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the community filter removes an extended community only by exact 8-octet value (parseExtendedWire, internal/component/bgp/plugins/filter_community/config.go:230, applied by genericCommunityHandler, handler.go:29), so the Encapsulation Extended Community cannot be filtered as a type |
| `RFC9012-11-10` | It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same exact-value restriction applies on egress: extended-community-remove takes one 8-octet value (internal/component/bgp/reactor/filter_delta.go:268) and no rule matches the Encapsulation Extended Community by its type and sub-type |
| `RFC9012-11-11` | * In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filtering of the Encapsulation Extended Community is configured by default on EBGP sessions: every community operation comes from an explicit policy entry (parseModifyDefs, internal/component/bgp/plugins/filter_modify/config.go:50) |
| `RFC9012-13-1` | The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never checks that a TLV ends where its final sub-TLV ends: ParseTunnelEncap walks the outer TLVs only and copies each value whole (internal/core/bgp/attribute/tunnel_encap.go:39), and SubTLVs is called on demand by display code rather than at parse time (internal/test/decode/decode_tunnel_encap.go:34) |
| `RFC9012-13-2` | If this is not the case, the TLV MUST be considered to be malformed, and the "Treat-as- withdraw" procedure of [RFC7606] is applied. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a TLV whose final sub-TLV overruns it produces no treat-as-withdraw: the misalignment is not detected at parse time (internal/core/bgp/attribute/tunnel_encap.go:39) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so the UPDATE is accepted in full |
| `RFC9012-13-3` | If a Tunnel Encapsulation attribute can be parsed correctly but contains a TLV whose tunnel type is not recognized by a particular BGP speaker, that BGP speaker MUST NOT consider the attribute to be malformed. (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L154). **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L114). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L149). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L155) |
| `RFC9012-13-4` | Rather, it MUST interpret the attribute as if that TLV had not been present. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code interprets a tunnel type at all, so an unrecognized one is not deliberately treated as absent: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and nothing branches on TunnelTLV.TunnelType outside display (internal/test/decode/decode_tunnel_encap.go:28) |
| `RFC9012-13-5` | If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized TLV MUST remain in the attribute. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L156). **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L123). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L157). **negative:** `unit/verify` [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L137) |
| `RFC9012-13-6` | The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| `RFC9012-13-8` | However, the Tunnel TLV containing them MUST NOT be considered to be malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L267). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L158). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L159). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L159) |
| `RFC9012-13-9` | all the sub-TLVs MUST be propagated if the route carrying the Tunnel Encapsulation attribute is propagated. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L178). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L160). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L192). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L161) |
| `RFC9012-13-10` | If a TLV of a Tunnel Encapsulation attribute contains a sub-TLV that is not recognized by a particular BGP speaker, the BGP speaker MUST process that TLV as if the unrecognized sub-TLV had not been present. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L162). **positive:** `unit/verify` [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L211). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L163). **negative:** `unit/verify` [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L219) |
| `RFC9012-13-11` | If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized sub-TLV MUST remain in the attribute. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L185). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L164). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L195). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L165) |
| `RFC9012-13-12` | In general, if a TLV contains a sub-TLV that is malformed, the sub- TLV MUST be treated as if it were an unrecognized sub-TLV. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L238). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L166). **negative:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L247). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L167) |
| `RFC9012-13-13` | if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a malformed Tunnel Egress Endpoint sub-TLV is never identified, so the TLV containing it is not ignored: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-13-14` | if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation attribute before the route carrying that attribute is distributed. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a TLV is never removed from the attribute before distribution: WriteTo re-emits every parsed TLV verbatim (internal/core/bgp/attribute/tunnel_encap.go:71) and no code inspects a Tunnel Egress Endpoint sub-TLV to decide otherwise |
| `RFC9012-13-15` | Within a Tunnel Encapsulation attribute that is carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV. (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the count of Tunnel Egress Endpoint sub-TLVs in a TLV is never taken, so a TLV without exactly one is not treated as carrying a malformed one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-13-16` | A TLV identifying a particular tunnel type may contain a sub-TLV that is meaningless for that tunnel type. For example, perhaps the TLV contains a UDP Destination Port sub-TLV, but the identified tunnel type does not use UDP encapsulation at all, or a tunnel of the form "X-in-Y" contains a Protocol Type sub-TLV that specifies something other than "X". Sub-TLVs of this sort MUST be disregarded. (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L300). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L168). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L309). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L169) |
| `RFC9012-13-17` | That is, they MUST NOT affect the creation of the encapsulation header. (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze creates no encapsulation header from the attribute, so no sub-TLV meaningless or otherwise contributes to one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib matches nothing |
| `RFC9012-13-18` | However, the sub-TLV MUST NOT be considered to be malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L293). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L170). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L154). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L171) |
| `RFC9012-13-19` | the sub-TLV MUST NOT be considered to be malformed and MUST NOT be removed from the TLV before the route carrying the Tunnel Encapsulation attribute is distributed. (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L316). **positive:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L172). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L319). **negative:** `unit/verify` [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L173) |
| `RFC9012-15-1` | RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze filters no traffic and no attribute at an SR domain boundary: there is no attribute-23 filter to apply (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) and the Prefix-SID sub-TLV of the attribute is never decoded (internal/core/bgp/attribute/tunnel_encap.go:145) |
| `RFC9012-3.1-1` | The Reserved subfield SHOULD be originated as zero. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-9` | The Traffic Class (TC) field [RFC3270][RFC5129] of each label stack entry SHOULD be set to 0, unless changed by policy at the originator of the sub-TLV. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-10` | When pushing the label stack onto a packet, the TC of each label stack SHOULD be preserved, unless local policy results in a modification. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-11` | The TTL (Time to Live) field of each label stack entry SHOULD be set to 255, unless changed to some other non-zero value by policy at the originator of the sub-TLV. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-12` | When pushing the label stack onto a packet, the TTL of each label stack entry SHOULD be preserved, unless local policy results in a modification to some other non-zero value. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.7-1` | similar limitations exist for the Prefix-SID sub-TLV: it SHOULD only be included in a BGP UPDATE message for one of the address families for which [RFC8669] has a defined behavior, namely BGP IPv4/IPv6 Labeled Unicast [RFC4760] [RFC8277]. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-3` | When the attribute is filtered from an incoming UPDATE, the attribute is neither processed nor distributed. This filtering SHOULD be possible on a per-BGP-session basis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-7` | In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. This filtering SHOULD be possible on a per-BGP-session basis. (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
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
| [`RFC9012-3.1-2`](#rfc9012-3.1-2) It MUST be disregarded on receipt (§3.1) | {gap}, no test | no code decodes the Tunnel Egress Endpoint sub-TLV, so ze holds no Reserved subfield to disregard: SubTLVs returns raw type and value pairs (internal/core/bgp/attribute/tunnel_encap.go:104) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.1-4`](#rfc9012-3.1-4) If the Address Family subfield contains the value for IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix). (§3.1) | {gap}, no test | ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv4 Address Family carrying an address of any other width is accepted unchanged |
| [`RFC9012-3.1-5`](#rfc9012-3.1-5) If the Address Family subfield contains the value for IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix). (§3.1) | {gap}, no test | ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv6 Address Family carrying an address of any other width is accepted unchanged |
| [`RFC9012-3.1-7`](#rfc9012-3.1-7) In this case, the Length field of Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06). (§3.1) | {gap}, no test | the Length of a Tunnel Egress Endpoint sub-TLV is never checked against its Address Family: SubTLVs reads the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.1-8`](#rfc9012-3.1-8) When the Tunnel Encapsulation attribute is carried in an UPDATE message of one of the AFI/SAFIs specified in this document (see the first paragraph of Section 6), each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV. (§3.1) | {gap}, no test | nothing counts Tunnel Egress Endpoint sub-TLVs per TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), so a TLV with none or with several is accepted identically |
| [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3) Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1) They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no VXLAN or NVGRE Encapsulation sub-TLV, so it sets no R bits: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encapsulation sub-?tlv\|buildEncap' over internal, pkg and cmd finds no Encapsulation sub-TLV builder |
| [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3) Any receiving routers MUST ignore these bits upon receipt. (§3.2.1, §3.2.2) | {gap}, no test | ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4) If the V bit is set to 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5) If the M bit is set to 0, this field MUST be set to all zeroes on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6) Reserved: MUST be set to zero on transmission and disregarded on receipt. (§3.2.1, §3.2.2) | {gap}, no test | the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1) Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4, §3.2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no GRE or MPLS-in-GRE Encapsulation sub-TLV and has no GRE key to advertise in BGP: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331); the only GRE key in ze belongs to interface tunnels (internal/plugins/iface/netlink/tunnel_linux.go:35) |
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
| [`RFC9012-3.7-2`](#rfc9012-3.7-2) If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7) | {gap}, no test | the Prefix-SID sub-TLV of the Tunnel Encapsulation attribute is never decoded, so it is neither used nor deliberately ignored per address family: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.7-3`](#rfc9012-3.7-3) If an Originator SRGB is specified in the sub-TLV, that SRGB MUST be interpreted to be the SRGB used by the tunnel's egress endpoint. (§3.7) | {gap}, no test | no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| [`RFC9012-3.7-4`](#rfc9012-3.7-4) If a Label-Index is present in the Prefix-SID sub-TLV, then when a packet is sent through the tunnel identified by the TLV, if that tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack. (§3.7) | {gap}, no test | no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-4.1-1`](#rfc9012-4.1-1) In situations where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| [`RFC9012-4.1-2`](#rfc9012-4.1-2) Notwithstanding, an implementation MUST be prepared to process a tunnel received encoded as a barebones TLV. (§4.1) | {gap}, no test | a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| [`RFC9012-4.1-3`](#rfc9012-4.1-3) Packets with other payload types MUST NOT be carried through such tunnels. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| [`RFC9012-4.2-1`](#rfc9012-4.2-1) In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes neither side of the conflict into a MAC address, so the two can never disagree. An extended community reaches ze only as 8 opaque octets that are parsed in and re-encoded out unchanged (internal/core/bgp/attribute/community.go:275, :250) with no Router's MAC decoder -- grep -rniE "router.?s? mac\|RouterMAC" over internal, pkg and cmd matches only the VRRP virtual MAC (internal/plugins/vrrp/packet/packet.go:94) -- and the only Tunnel Encapsulation sub-TLV ze decodes is Preference (TunnelTLV.Preference, internal/core/bgp/attribute/tunnel_encap.go:145), never a VXLAN or NVGRE Encapsulation sub-TLV |
| [`RFC9012-4.3-1`](#rfc9012-4.3-1) No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| [`RFC9012-6-2`](#rfc9012-6-2) Then router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U. (§6) | {gap}, no test | ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| [`RFC9012-7.1-1`](#rfc9012-7.1-1) If a route includes the Tunnel Encapsulation attribute, and if that attribute includes no tunnel that is feasible, then that route MUST NOT be considered resolvable for the purposes of the route resolvability condition ([RFC4271], Section 9.1.2.1). (§7.1) | {gap}, no test | tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-8-1`](#rfc9012-8-1) Then packet P MUST be sent through one of the tunnels identified in the Tunnel Encapsulation attribute of UPDATE U2. (§8) | {gap}, no test | a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-8-2`](#rfc9012-8-2) In that case, packet P MUST NOT be sent through the tunnel contained in that TLV, unless U1 is carrying a Color Extended Community that is identified in one of U2's Color sub-TLVs. (§8) | {gap}, no test | no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| [`RFC9012-10-1`](#rfc9012-10-1) Any document specifying such joint use MUST provide details as to how interactions should be handled. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| [`RFC9012-11-1`](#rfc9012-11-1) However, the Tunnel Encapsulation attribute MUST be used only within a well-defined scope, for example, within a set of ASes that belong to a single administrative entity. (§11) | {gap}, no test | ze enforces no scope for the attribute: it is parsed from any peer and re-advertised unchanged (a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151)), and no configuration bounds where it may travel (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-2`](#rfc9012-11-2) To prevent the Tunnel Encapsulation attribute from being distributed beyond its intended scope, any BGP speaker that understands the attribute MUST be able to filter the attribute from incoming BGP UPDATE messages. (§11) | {gap}, no test | no filter can drop attribute 23 on ingress: setBlockAllowedKeys is a closed set of modifier keys with no tunnel-encapsulation entry (internal/component/bgp/plugins/filter_modify/config.go:30) and communityDirectives covers only the three community attributes (internal/component/bgp/reactor/filter_delta.go:268) |
| [`RFC9012-11-5`](#rfc9012-11-5) For each external BGP (EBGP) session, filtering of the attribute on incoming UPDATEs MUST be enabled by default. (§11) | {gap}, no test | there is no attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-6`](#rfc9012-11-6) In addition, any BGP speaker that understands the attribute MUST be able to filter the attribute from outgoing BGP UPDATE messages. (§11) | {gap}, no test | no filter can drop attribute 23 on egress: the egress path rebuilds attributes through AttrModHandlers keyed by attribute code (internal/component/bgp/reactor/forward_build.go:211) and no handler or directive names code 23 (internal/component/bgp/reactor/filter_delta.go:268) |
| [`RFC9012-11-8`](#rfc9012-11-8) For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | {gap}, no test | there is no outgoing attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-9`](#rfc9012-11-9) * Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. (§11) | {gap}, no test | the community filter removes an extended community only by exact 8-octet value (parseExtendedWire, internal/component/bgp/plugins/filter_community/config.go:230, applied by genericCommunityHandler, handler.go:29), so the Encapsulation Extended Community cannot be filtered as a type |
| [`RFC9012-11-10`](#rfc9012-11-10) It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) | {gap}, no test | the same exact-value restriction applies on egress: extended-community-remove takes one 8-octet value (internal/component/bgp/reactor/filter_delta.go:268) and no rule matches the Encapsulation Extended Community by its type and sub-type |
| [`RFC9012-11-11`](#rfc9012-11-11) * In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11) | {gap}, no test | no filtering of the Encapsulation Extended Community is configured by default on EBGP sessions: every community operation comes from an explicit policy entry (parseModifyDefs, internal/component/bgp/plugins/filter_modify/config.go:50) |
| [`RFC9012-13-1`](#rfc9012-13-1) The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13) | {gap}, no test | ze never checks that a TLV ends where its final sub-TLV ends: ParseTunnelEncap walks the outer TLVs only and copies each value whole (internal/core/bgp/attribute/tunnel_encap.go:39), and SubTLVs is called on demand by display code rather than at parse time (internal/test/decode/decode_tunnel_encap.go:34) |
| [`RFC9012-13-2`](#rfc9012-13-2) If this is not the case, the TLV MUST be considered to be malformed, and the "Treat-as- withdraw" procedure of [RFC7606] is applied. (§13) | {gap}, no test | a TLV whose final sub-TLV overruns it produces no treat-as-withdraw: the misalignment is not detected at parse time (internal/core/bgp/attribute/tunnel_encap.go:39) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so the UPDATE is accepted in full |
| [`RFC9012-13-4`](#rfc9012-13-4) Rather, it MUST interpret the attribute as if that TLV had not been present. (§13) | {gap}, no test | no code interprets a tunnel type at all, so an unrecognized one is not deliberately treated as absent: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and nothing branches on TunnelTLV.TunnelType outside display (internal/test/decode/decode_tunnel_encap.go:28) |
| [`RFC9012-13-6`](#rfc9012-13-6) The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| [`RFC9012-13-13`](#rfc9012-13-13) if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored (§13) | {gap}, no test | a malformed Tunnel Egress Endpoint sub-TLV is never identified, so the TLV containing it is not ignored: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-13-14`](#rfc9012-13-14) if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation attribute before the route carrying that attribute is distributed. (§13) | {gap}, no test | a TLV is never removed from the attribute before distribution: WriteTo re-emits every parsed TLV verbatim (internal/core/bgp/attribute/tunnel_encap.go:71) and no code inspects a Tunnel Egress Endpoint sub-TLV to decide otherwise |
| [`RFC9012-13-15`](#rfc9012-13-15) Within a Tunnel Encapsulation attribute that is carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV. (§13) | {gap}, no test | the count of Tunnel Egress Endpoint sub-TLVs in a TLV is never taken, so a TLV without exactly one is not treated as carrying a malformed one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-13-17`](#rfc9012-13-17) That is, they MUST NOT affect the creation of the encapsulation header. (§13) | {gap}, no test | ze creates no encapsulation header from the attribute, so no sub-TLV meaningless or otherwise contributes to one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib matches nothing |
| [`RFC9012-15-1`](#rfc9012-15-1) RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). (§15) | {gap}, no test | ze filters no traffic and no attribute at an SR domain boundary: there is no attribute-23 filter to apply (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) and the Prefix-SID sub-TLV of the attribute is never decoded (internal/core/bgp/attribute/tunnel_encap.go:145) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9012-3.1-2`](#rfc9012-3.1-2)

It MUST be disregarded on receipt (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-2, so no unit is bound to it.

### [`RFC9012-3.1-3`](#rfc9012-3.1-3)

it MUST be propagated unchanged (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: non-zero Tunnel Egress Endpoint Reserved octets reach the wire unchanged raw (+) and are not zeroed on rebuild (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L175) | unit/verify | revert, verified |
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L336) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L333) | unit/verify | unproven |

### [`RFC9012-3.1-4`](#rfc9012-3.1-4)

If the Address Family subfield contains the value for IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix). (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-4, so no unit is bound to it.

### [`RFC9012-3.1-5`](#rfc9012-3.1-5)

If the Address Family subfield contains the value for IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix). (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-5, so no unit is bound to it.

### [`RFC9012-3.1-7`](#rfc9012-3.1-7)

In this case, the Length field of Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06). (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-7, so no unit is bound to it.

### [`RFC9012-3.1-8`](#rfc9012-3.1-8)

When the Tunnel Encapsulation attribute is carried in an UPDATE message of one of the AFI/SAFIs specified in this document (see the first paragraph of Section 6), each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-8, so no unit is bound to it.

### [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3)

Note that if the forwarding route changes, this procedure MUST be reapplied. (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1.1-3, so no unit is bound to it.

### [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1)

They MUST always be set to 0 by the originator of the sub-TLV. (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-1, so no unit is bound to it.

### [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2)

Intermediate routers MUST propagate them without modification. (§3.2.1, §3.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: VXLAN R bits (all ones in the dirty run) reach the wire unmodified raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L177) | unit/verify | revert, verified |
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L347) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L344) | unit/verify | unproven |

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

Unless a key value is being advertised, the GRE Encapsulation sub-TLV MUST NOT be present. (§3.2.4, §3.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.4-1, so no unit is bound to it.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the Embedded Label Handling sub-TLV, which Ze ignores, is not stripped raw (+) or rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L179) | unit/verify | revert, verified |
| negative | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L370) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L362) | unit/verify | unproven |

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

If included in a BGP UPDATE for any other address family, it MUST be ignored. (§3.7)

Audit verdict: not audited: no reader has judged these tests

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

In case of such a conflict, the information in the Router's MAC Extended Community MUST be used. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.2-1, so no unit is bound to it.

### [`RFC9012-4.3-1`](#rfc9012-4.3-1)

No flags are defined in this document; this field MUST be set to zero by the originator and ignored by the receiver (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.3-1, so no unit is bound to it.

### [`RFC9012-4.3-2`](#rfc9012-4.3-2)

the value MUST NOT be changed when propagating this extended community. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the Color Extended Community value reaches the wire unchanged raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L181) | unit/verify | revert, verified |
| negative | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L397) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L394) | unit/verify | unproven |

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

* Any BGP speaker that understands it MUST be able to filter it from incoming BGP UPDATE messages. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-9, so no unit is bound to it.

### [`RFC9012-11-10`](#rfc9012-11-10)

It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-10, so no unit is bound to it.

### [`RFC9012-11-11`](#rfc9012-11-11)

* In both cases, this filtering MUST be enabled by default for EBGP sessions. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-11, so no unit is bound to it.

### [`RFC9012-13-1`](#rfc9012-13-1)

The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-1, so no unit is bound to it.

### [`RFC9012-13-2`](#rfc9012-13-2)

If this is not the case, the TLV MUST be considered to be malformed, and the "Treat-as- withdraw" procedure of [RFC7606] is applied. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-2, so no unit is bound to it.

### [`RFC9012-13-3`](#rfc9012-13-3)

If a Tunnel Encapsulation attribute can be parsed correctly but contains a TLV whose tunnel type is not recognized by a particular BGP speaker, that BGP speaker MUST NOT consider the attribute to be malformed. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the 0xFFFE TLV gets no RFC 7606 action (+) and the rebuilt UPDATE still carries it (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L155) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L149) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L114) | unit/verify | unproven |

### [`RFC9012-13-4`](#rfc9012-13-4)

Rather, it MUST interpret the attribute as if that TLV had not been present. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-4, so no unit is bound to it.

### [`RFC9012-13-5`](#rfc9012-13-5)

If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized TLV MUST remain in the attribute. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the unrecognized-type TLV stays in the attribute forwarded raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L157) | unit/verify | revert, verified |
| negative | [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L137) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L156) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L123) | unit/verify | unproven |

### [`RFC9012-13-6`](#rfc9012-13-6)

The following sub-TLVs defined in this document MUST NOT occur more than once in a given Tunnel TLV: Tunnel Egress Endpoint (discussed below), Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-6, so no unit is bound to it.

### [`RFC9012-13-8`](#rfc9012-13-8)

However, the Tunnel TLV containing them MUST NOT be considered to be malformed (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the Tunnel TLV with two Tunnel Egress Endpoint sub-TLVs (one of the section 13 single-instance types, answering the earlier objection) is not malformed (+) and stays whole on rebuild (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L159) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L267) | unit/verify | unproven |

### [`RFC9012-13-9`](#rfc9012-13-9)

all the sub-TLVs MUST be propagated if the route carrying the Tunnel Encapsulation attribute is propagated. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: both Tunnel Egress Endpoint occurrences reach the wire raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L161) | unit/verify | revert, verified |
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L178) | unit/verify | unproven |

### [`RFC9012-13-10`](#rfc9012-13-10)

If a TLV of a Tunnel Encapsulation attribute contains a sub-TLV that is not recognized by a particular BGP speaker, the BGP speaker MUST process that TLV as if the unrecognized sub-TLV had not been present. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the TLV with an unrecognized sub-TLV gets the same verdict and handling as the clean TLV (+, - differential of the two runs).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L163) | unit/verify | revert, verified |
| negative | [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L162) | unit/verify | revert, verified |
| positive | [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L211) | unit/verify | unproven |

### [`RFC9012-13-11`](#rfc9012-13-11)

If the route carrying the Tunnel Encapsulation attribute is propagated with the attribute, the unrecognized sub-TLV MUST remain in the attribute. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the unrecognized sub-TLV remains in the attribute raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L165) | unit/verify | revert, verified |
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L195) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L164) | unit/verify | revert, verified |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L185) | unit/verify | unproven |

### [`RFC9012-13-12`](#rfc9012-13-12)

In general, if a TLV contains a sub-TLV that is malformed, the sub- TLV MUST be treated as if it were an unrecognized sub-TLV. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the UDP Destination Port sub-TLV of Length 3 is handled as an unrecognized one: no action, carried; dirty and clean verdicts equal. The codec unit TestRFC9012MalformedSubTLVTreatedAsUnrecognized covers the one sub-TLV Ze reads (Preference).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L167) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L247) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L166) | unit/verify | revert, verified |
| positive | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L238) | unit/verify | unproven |

### [`RFC9012-13-13`](#rfc9012-13-13)

if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-13, so no unit is bound to it.

### [`RFC9012-13-14`](#rfc9012-13-14)

if a TLV contains a malformed Tunnel Egress Endpoint sub-TLV (as defined in Section 3.1), the entire TLV MUST be ignored and MUST be removed from the Tunnel Encapsulation attribute before the route carrying that attribute is distributed. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-14, so no unit is bound to it.

### [`RFC9012-13-15`](#rfc9012-13-15)

Within a Tunnel Encapsulation attribute that is carried by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-15, so no unit is bound to it.

### [`RFC9012-13-16`](#rfc9012-13-16)

A TLV identifying a particular tunnel type may contain a sub-TLV that is meaningless for that tunnel type. For example, perhaps the TLV contains a UDP Destination Port sub-TLV, but the identified tunnel type does not use UDP encapsulation at all, or a tunnel of the form "X-in-Y" contains a Protocol Type sub-TLV that specifies something other than "X". Sub-TLVs of this sort MUST be disregarded. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: a VXLAN sub-TLV inside an SR Policy TLV changes neither verdict nor handling.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L169) | unit/verify | revert, verified |
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L309) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L300) | unit/verify | unproven |

### [`RFC9012-13-17`](#rfc9012-13-17)

That is, they MUST NOT affect the creation of the encapsulation header. (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-17, so no unit is bound to it.

### [`RFC9012-13-18`](#rfc9012-13-18)

However, the sub-TLV MUST NOT be considered to be malformed (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the TLV holding the meaningless sub-TLV gets no RFC 7606 action (+) and is carried on rebuild (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L171) | unit/verify | revert, verified |
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L170) | unit/verify | revert, verified |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L293) | unit/verify | unproven |

### [`RFC9012-13-19`](#rfc9012-13-19)

the sub-TLV MUST NOT be considered to be malformed and MUST NOT be removed from the TLV before the route carrying the Tunnel Encapsulation attribute is distributed. (§13)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong: a clean and a dirty Tunnel Encapsulation attribute (dirty: all-ones reserved fields, unrecognized sub-TLV 200, UDP Destination Port of Length 3, VXLAN sub-TLV in an SR Policy TLV, second Tunnel Egress Endpoint, TLV of unrecognized type 0xFFFE) go through enforceRFC7606 (ActionNone, no error, octet-equal) and to a peer's socket both forwarded raw and rebuilt by buildModifiedPayload for next-hop-self and a new MED; attribute 23 and the Color extended community reach the wire octet-equal in every run. Ze reads no received sub-TLV (only TunnelTLV.Preference, test decode only), so the carrier's obligation is observed as verdict plus carry. Judge break: a receive validator acting on Preference Flags made the dirty run red. Here: the meaningless sub-TLV is not malformed and is not removed, raw (+) and rebuilt (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L173) | unit/verify | revert, verified |
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L319) | unit/verify | unproven |
| positive | [`TestRFC9012TunnelEncapReceivedIgnoredAndPassedAlong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go#L172) | unit/verify | revert, verified |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L316) | unit/verify | unproven |

### [`RFC9012-15-1`](#rfc9012-15-1)

RFC 8402 specifies that "SR domain boundary routers MUST filter any external traffic" ([RFC8402], Section 8.1). (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-15-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc9012.txt |
| Source fingerprint | 0d9a9771610007d7 |
| Record | rfc/extraction/rfc9012.json |
| Mapped sentences | 68 |
| Declined as scope | 8 |
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
| `3.2.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2.5 repeats Section 3.2.4's rule for the MPLS-in-GRE Encapsulation sub-TLV; the mapped row states the obligation for both sub-TLVs and cites both sections. | Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present. |
| `13:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds the speaker that interprets the listed sub-TLVs (Tunnel Egress Endpoint, Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, Prefix-SID) to build a tunnel, which Ze does not implement: no product code reads any sub-TLV of a received Tunnel Encapsulation attribute (TunnelTLV.Preference's only non-test caller is internal/test/decode), and Ze only carries the attribute. Row RFC9012-13-7 is retired (rfc/corrections/rfc9012.md) | If a Tunnel TLV has more than one of any of these sub-TLVs, all but the first occurrence of each such sub-TLV type MUST be disregarded. |

## Superseded

No document obsoletes RFC 9012, so its obligations are stated where they were written.
