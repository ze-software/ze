# RFC 9012 - The BGP Tunnel Encapsulation Attribute

Partial. Every requirement this repository extracted from RFC 9012, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 22.7% | 15 of 66 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 66 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 66 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 30 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 75 | of 97 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 75 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 77.3% | 51 of 66 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 66 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 97 |
| Gated MUST-level | 75 |
| Obligations that bind Ze | 66 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 51 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tagged units | 30 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 60 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **75** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC9012-3.1-3`](#rfc9012-3.1-3), [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2), [`RFC9012-3.5-3`](#rfc9012-3.5-3), [`RFC9012-4.3-2`](#rfc9012-4.3-2), [`RFC9012-13-3`](#rfc9012-13-3), [`RFC9012-13-5`](#rfc9012-13-5), [`RFC9012-13-7`](#rfc9012-13-7), [`RFC9012-13-8`](#rfc9012-13-8), [`RFC9012-13-9`](#rfc9012-13-9), [`RFC9012-13-10`](#rfc9012-13-10), [`RFC9012-13-11`](#rfc9012-13-11), [`RFC9012-13-12`](#rfc9012-13-12), [`RFC9012-13-16`](#rfc9012-13-16), [`RFC9012-13-18`](#rfc9012-13-18), [`RFC9012-13-19`](#rfc9012-13-19)

**Annotated instead of tested (60):** [`RFC9012-3.1-2`](#rfc9012-3.1-2), [`RFC9012-3.1-4`](#rfc9012-3.1-4), [`RFC9012-3.1-5`](#rfc9012-3.1-5), [`RFC9012-3.1-7`](#rfc9012-3.1-7), [`RFC9012-3.1-8`](#rfc9012-3.1-8), [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3), [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1), [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3), [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4), [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5), [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6), [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1), [`RFC9012-3.3-1`](#rfc9012-3.3-1), [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1), [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2), [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3), [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4), [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2), [`RFC9012-3.5-1`](#rfc9012-3.5-1), [`RFC9012-3.5-2`](#rfc9012-3.5-2), [`RFC9012-3.5-4`](#rfc9012-3.5-4), [`RFC9012-3.6-1`](#rfc9012-3.6-1), [`RFC9012-3.6-2`](#rfc9012-3.6-2), [`RFC9012-3.6-3`](#rfc9012-3.6-3), [`RFC9012-3.6-4`](#rfc9012-3.6-4), [`RFC9012-3.6-5`](#rfc9012-3.6-5), [`RFC9012-3.6-6`](#rfc9012-3.6-6), [`RFC9012-3.6-7`](#rfc9012-3.6-7), [`RFC9012-3.6-8`](#rfc9012-3.6-8), [`RFC9012-3.6-13`](#rfc9012-3.6-13), [`RFC9012-3.7-2`](#rfc9012-3.7-2), [`RFC9012-3.7-3`](#rfc9012-3.7-3), [`RFC9012-3.7-4`](#rfc9012-3.7-4), [`RFC9012-4.1-1`](#rfc9012-4.1-1), [`RFC9012-4.1-2`](#rfc9012-4.1-2), [`RFC9012-4.1-3`](#rfc9012-4.1-3), [`RFC9012-4.2-1`](#rfc9012-4.2-1), [`RFC9012-4.3-1`](#rfc9012-4.3-1), [`RFC9012-6-2`](#rfc9012-6-2), [`RFC9012-7.1-1`](#rfc9012-7.1-1), [`RFC9012-8-1`](#rfc9012-8-1), [`RFC9012-8-2`](#rfc9012-8-2), [`RFC9012-10-1`](#rfc9012-10-1), [`RFC9012-11-1`](#rfc9012-11-1), [`RFC9012-11-2`](#rfc9012-11-2), [`RFC9012-11-5`](#rfc9012-11-5), [`RFC9012-11-6`](#rfc9012-11-6), [`RFC9012-11-8`](#rfc9012-11-8), [`RFC9012-11-9`](#rfc9012-11-9), [`RFC9012-11-10`](#rfc9012-11-10), [`RFC9012-11-11`](#rfc9012-11-11), [`RFC9012-13-1`](#rfc9012-13-1), [`RFC9012-13-2`](#rfc9012-13-2), [`RFC9012-13-4`](#rfc9012-13-4), [`RFC9012-13-6`](#rfc9012-13-6), [`RFC9012-13-13`](#rfc9012-13-13), [`RFC9012-13-14`](#rfc9012-13-14), [`RFC9012-13-15`](#rfc9012-13-15), [`RFC9012-13-17`](#rfc9012-13-17), [`RFC9012-15-1`](#rfc9012-15-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9012-3.1-2` | The Reserved subfield of the Tunnel Egress Endpoint sub-TLV MUST be disregarded on receipt (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code decodes the Tunnel Egress Endpoint sub-TLV, so ze holds no Reserved subfield to disregard: SubTLVs returns raw type and value pairs (internal/core/bgp/attribute/tunnel_encap.go:104) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.1-3` | The Reserved subfield of the Tunnel Egress Endpoint sub-TLV MUST be propagated unchanged (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L335). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L338) |
| `RFC9012-3.1-4` | If the Address Family subfield is IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix) (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv4 Address Family carrying an address of any other width is accepted unchanged |
| `RFC9012-3.1-5` | If the Address Family subfield is IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix) (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv6 Address Family carrying an address of any other width is accepted unchanged |
| `RFC9012-3.1-7` | If the Address Family subfield contains 0, the Length field of the Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06) (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Length of a Tunnel Egress Endpoint sub-TLV is never checked against its Address Family: SubTLVs reads the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.1-8` | When the attribute is carried in an UPDATE of one of the AFI/SAFIs of Section 6, each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing counts Tunnel Egress Endpoint sub-TLVs per TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), so a TLV with none or with several is accepted identically |
| `RFC9012-3.1.1-3` | If the forwarding route changes, the address-subfield validation procedure MUST be reapplied (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-3.2.1-1` | The reserved (R) Flags bits of the VXLAN/NVGRE Encapsulation sub-TLV MUST always be set to 0 by the originator (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no VXLAN or NVGRE Encapsulation sub-TLV, so it sets no R bits: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encapsulation sub-?tlv\|buildEncap' over internal, pkg and cmd finds no Encapsulation sub-TLV builder |
| `RFC9012-3.2.1-2` | Intermediate routers MUST propagate the reserved (R) Flags bits without modification (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L346). **negative:** `unit/verify` [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L349) |
| `RFC9012-3.2.1-3` | Any receiving router MUST ignore the reserved (R) Flags bits upon receipt (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-4` | If the V bit is 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-5` | If the M bit is 0, the MAC Address field MUST be set to all zeroes on transmission and disregarded on receipt (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.1-6` | The Reserved field of the VXLAN/NVGRE Encapsulation sub-TLV MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.2.4-1` | Unless a key value is being advertised, the GRE or MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present (§3.2.4, §3.2.5) | MUST NOT | 3.2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no GRE or MPLS-in-GRE Encapsulation sub-TLV and has no GRE key to advertise in BGP: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331); the only GRE key in ze belongs to interface tunnels (internal/plugins/iface/netlink/tunnel_linux.go:35) |
| `RFC9012-3.3-1` | An outer Encapsulation sub-TLV in a TLV whose tunnel type does not use the corresponding outer encapsulation MUST be treated as an unrecognized type of sub-TLV (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze recognizes no Encapsulation sub-TLV under any tunnel type, so it never makes the tunnel-type-specific determination this requires: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.3.2-1` | If the reserved value zero is received in a UDP Destination Port sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no UDP Destination Port sub-TLV decoder exists, so the reserved value zero is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-2` | Packets with payload types other than the one signaled by the Protocol Type sub-TLV MUST NOT be encapsulated in the relevant tunnel (§3.4.1) | MUST NOT | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no tunnel encapsulation from the attribute, so no payload type is ever checked against a Protocol Type sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the FIB receives no tunnel from BGP (grep for SubTLVs over internal/plugins/fib matches nothing) |
| `RFC9012-3.4.1-3` | If the reserved value 0xFFFF is received in a Protocol Type sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Protocol Type sub-TLV decoder exists, so the reserved value 0xFFFF is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.4.1-4` | For an "X-in-Y" tunnel, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Protocol Type sub-TLV and forms no X-in-Y tunnel, so a mismatched payload type is never identified: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.4.2-2` | If a Color sub-TLV's Length is other than 8, or the first two octets of its Value are not 0x030b, the sub-TLV MUST be treated as an unrecognized sub-TLV (§3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV decoder exists, so neither its Length nor its leading 0x030b octets are checked: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-1` | If the attribute is attached to an UPDATE of a non-labeled address family, the Embedded Label Handling sub-TLV MUST be disregarded (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Embedded Label Handling sub-TLV is never decoded and the attribute is never correlated with the address family of the UPDATE carrying it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-2` | If the Embedded Label Handling sub-TLV is in a TLV whose tunnel type has no virtual network identifier, it MUST be disregarded (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code relates a sub-TLV to whether its tunnel type uses a virtual network identifier: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.5-3` | When the Embedded Label Handling sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated (§3.5) | MUST NOT | 3.5 | **positive:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L364). **negative:** `unit/verify` [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L372) |
| `RFC9012-3.5-4` | If any Embedded Label Handling value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed per Section 13 (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Embedded Label Handling value is never read, so a value outside 1 and 2 is not detected and not treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| `RFC9012-3.6-1` | When the MPLS label stack is pushed onto a packet, its topmost-to-bottommost ordering MUST be preserved (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no ordering is preserved or lost: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/mpls matches nothing |
| `RFC9012-3.6-2` | If a TLV contains an MPLS Label Stack sub-TLV, that label stack MUST be pushed onto the packet before any other labels (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no MPLS Label Stack sub-TLV is decoded and no push order is established: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-3` | For a labeled address family, the MPLS Label Stack sub-TLV contents MUST be pushed before the label embedded in the NLRI (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never combines an MPLS Label Stack sub-TLV with the label embedded in a labeled-unicast NLRI: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-4` | If the TLV identifies a tunnel type using virtual network identifiers, the MPLS Label Stack sub-TLV contents MUST be pushed before the Section 9 procedures are applied (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies no virtual network identifier procedure, so nothing is sequenced against it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-5` | The number of label stack entries MUST be determined from the Sub-TLV Length field (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no label stack entries are ever counted: SubTLVs uses the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and no caller reads sub-TLV type 10 |
| `RFC9012-3.6-6` | When pushed onto a packet that already has a label stack, the S bits of all pushed entries MUST be cleared (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no S bit is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-7` | When pushed onto a packet with no existing label stack, the S bit of the bottommost pushed entry MUST be set (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no bottom-of-stack bit is set: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-8` | When pushed onto a packet with no existing label stack, the S bit of every other pushed entry MUST be cleared (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so no S bit of a non-bottom entry is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.6-13` | If any label stack entry has a TTL of zero, the router pushing the stack MUST change it to a non-zero value (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no label stack from the attribute, so a zero TTL in a received entry is never rewritten: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.7-2` | If a Prefix-SID sub-TLV is included in a BGP UPDATE for an address family other than IPv4/IPv6 Labeled Unicast, it MUST be ignored (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Prefix-SID sub-TLV of the Tunnel Encapsulation attribute is never decoded, so it is neither used nor deliberately ignored per address family: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-3.7-3` | If an Originator SRGB is specified in the Prefix-SID sub-TLV, that SRGB MUST be interpreted as the SRGB used by the tunnel's egress endpoint (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| `RFC9012-3.7-4` | If a Label-Index is present and the tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-4.1-1` | Where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| `RFC9012-4.1-2` | An implementation MUST be prepared to process a tunnel received encoded as a barebones TLV (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| `RFC9012-4.1-3` | For an "X-in-Y" tunnel signaled via the Encapsulation Extended Community, packets with other payload types MUST NOT be carried through the tunnel (§4.1) | MUST NOT | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| `RFC9012-4.2-1` | If a Router's MAC Extended Community and a VXLAN/NVGRE Encapsulation sub-TLV carry conflicting MACs, the Router's MAC Extended Community value MUST be used (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes neither side of the conflict into a MAC address, so the two can never disagree. An extended community reaches ze only as 8 opaque octets that are parsed in and re-encoded out unchanged (internal/core/bgp/attribute/community.go:275, :250) with no Router's MAC decoder -- grep -rniE "router.?s? mac\|RouterMAC" over internal, pkg and cmd matches only the VRRP virtual MAC (internal/plugins/vrrp/packet/packet.go:94) -- and the only Tunnel Encapsulation sub-TLV ze decodes is Preference (TunnelTLV.Preference, internal/core/bgp/attribute/tunnel_encap.go:145), never a VXLAN or NVGRE Encapsulation sub-TLV |
| `RFC9012-4.3-1` | The Flags field of the Color Extended Community MUST be set to zero by the originator and ignored by the receiver (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| `RFC9012-4.3-2` | The Color Extended Community value MUST NOT be changed when propagating the extended community (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L396). **negative:** `unit/verify` [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L399) |
| `RFC9012-6-2` | Router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| `RFC9012-7.1-1` | A route whose Tunnel Encapsulation attribute includes no feasible tunnel MUST NOT be considered resolvable for the route resolvability condition (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-8-1` | When U1 has no attribute and its next hop resolves to U2 which has the attribute, packet P MUST be sent through one of U2's tunnels (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| `RFC9012-8-2` | Packet P MUST NOT be sent through a tunnel whose TLV has Color sub-TLVs unless U1 carries a matching Color Extended Community (§8) | MUST NOT | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| `RFC9012-10-1` | Any document specifying joint use of the Tunnel Encapsulation attribute with other tunnel-signaling mechanisms MUST provide details on how interactions are handled (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| `RFC9012-11-1` | The Tunnel Encapsulation attribute MUST be used only within a well-defined scope (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze enforces no scope for the attribute: it is parsed from any peer and re-advertised unchanged (a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151)), and no configuration bounds where it may travel (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-2` | Any BGP speaker that understands the attribute MUST be able to filter it from incoming BGP UPDATE messages (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filter can drop attribute 23 on ingress: setBlockAllowedKeys is a closed set of modifier keys with no tunnel-encapsulation entry (internal/component/bgp/plugins/filter_modify/config.go:30) and communityDirectives covers only the three community attributes (internal/component/bgp/reactor/filter_delta.go:268) |
| `RFC9012-11-5` | For each EBGP session, filtering of the attribute on incoming UPDATEs MUST be enabled by default (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-6` | Any BGP speaker that understands the attribute MUST be able to filter it from outgoing BGP UPDATE messages (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filter can drop attribute 23 on egress: the egress path rebuilds attributes through AttrModHandlers keyed by attribute code (internal/component/bgp/reactor/forward_build.go:211) and no handler or directive names code 23 (internal/component/bgp/reactor/filter_delta.go:268) |
| `RFC9012-11-8` | For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no outgoing attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| `RFC9012-11-9` | Any BGP speaker that understands the Encapsulation Extended Community MUST be able to filter it from incoming BGP UPDATE messages (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the community filter removes an extended community only by exact 8-octet value (parseExtendedWire, internal/component/bgp/plugins/filter_community/config.go:230, applied by genericCommunityHandler, handler.go:29), so the Encapsulation Extended Community cannot be filtered as a type |
| `RFC9012-11-10` | It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same exact-value restriction applies on egress: extended-community-remove takes one 8-octet value (internal/component/bgp/reactor/filter_delta.go:268) and no rule matches the Encapsulation Extended Community by its type and sub-type |
| `RFC9012-11-11` | Filtering of the Encapsulation Extended Community MUST be enabled by default for EBGP sessions (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no filtering of the Encapsulation Extended Community is configured by default on EBGP sessions: every community operation comes from an explicit policy entry (parseModifyDefs, internal/component/bgp/plugins/filter_modify/config.go:50) |
| `RFC9012-13-1` | The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never checks that a TLV ends where its final sub-TLV ends: ParseTunnelEncap walks the outer TLVs only and copies each value whole (internal/core/bgp/attribute/tunnel_encap.go:39), and SubTLVs is called on demand by display code rather than at parse time (internal/test/decode/decode_tunnel_encap.go:34) |
| `RFC9012-13-2` | If the final octet of a TLV is not the final octet of its final sub-TLV, the TLV MUST be considered malformed and the RFC 7606 "treat-as-withdraw" procedure applied (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a TLV whose final sub-TLV overruns it produces no treat-as-withdraw: the misalignment is not detected at parse time (internal/core/bgp/attribute/tunnel_encap.go:39) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so the UPDATE is accepted in full |
| `RFC9012-13-3` | A TLV whose tunnel type is unrecognized MUST NOT cause the attribute to be considered malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L114). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L149) |
| `RFC9012-13-4` | An unrecognized tunnel type MUST be interpreted as if that TLV had not been present (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code interprets a tunnel type at all, so an unrecognized one is not deliberately treated as absent: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and nothing branches on TunnelTLV.TunnelType outside display (internal/test/decode/decode_tunnel_encap.go:28) |
| `RFC9012-13-5` | If the route is propagated with the attribute, an unrecognized TLV MUST remain in the attribute (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L123). **negative:** `unit/verify` [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L137) |
| `RFC9012-13-6` | Tunnel Egress Endpoint, Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID sub-TLVs MUST NOT occur more than once in a given Tunnel TLV (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| `RFC9012-13-7` | If such a single-instance sub-TLV occurs more than once, all but the first occurrence of each type MUST be disregarded (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L262). **negative:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L277) |
| `RFC9012-13-8` | A Tunnel TLV carrying duplicate single-instance sub-TLVs MUST NOT be considered malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L268). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L159) |
| `RFC9012-13-9` | All the sub-TLVs MUST be propagated if the route carrying the attribute is propagated (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L178). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L192) |
| `RFC9012-13-10` | If a TLV contains an unrecognized sub-TLV, the BGP speaker MUST process the TLV as if the unrecognized sub-TLV had not been present (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L211). **negative:** `unit/verify` [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L219) |
| `RFC9012-13-11` | If the route is propagated with the attribute, an unrecognized sub-TLV MUST remain in the attribute (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L185). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L195) |
| `RFC9012-13-12` | A malformed sub-TLV MUST be treated as if it were an unrecognized sub-TLV (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L238). **negative:** `unit/verify` [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L247) |
| `RFC9012-13-13` | A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be ignored in its entirety (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a malformed Tunnel Egress Endpoint sub-TLV is never identified, so the TLV containing it is not ignored: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-13-14` | A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be removed from the attribute before the route is distributed (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a TLV is never removed from the attribute before distribution: WriteTo re-emits every parsed TLV verbatim (internal/core/bgp/attribute/tunnel_encap.go:71) and no code inspects a Tunnel Egress Endpoint sub-TLV to decide otherwise |
| `RFC9012-13-15` | Within an UPDATE of an AFI/SAFI listed in Section 6, a TLV not containing exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV (§13) | MUST | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the count of Tunnel Egress Endpoint sub-TLVs in a TLV is never taken, so a TLV without exactly one is not treated as carrying a malformed one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| `RFC9012-13-16` | A sub-TLV that is meaningless for the identified tunnel type MUST be disregarded (§13) | MUST | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L302). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L311) |
| `RFC9012-13-17` | A meaningless sub-TLV MUST NOT affect the creation of the encapsulation header (§13) | MUST NOT | 13 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze creates no encapsulation header from the attribute, so no sub-TLV meaningless or otherwise contributes to one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib matches nothing |
| `RFC9012-13-18` | A meaningless sub-TLV MUST NOT be considered malformed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L295). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L154) |
| `RFC9012-13-19` | A meaningless sub-TLV MUST NOT be removed from the TLV before the route is distributed (§13) | MUST NOT | 13 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L318). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L321) |
| `RFC9012-15-1` | SR domain boundary routers MUST filter any external traffic, so the duty to filter extends to all routers participating in Prefix-SID tunnels (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze filters no traffic and no attribute at an SR domain boundary: there is no attribute-23 filter to apply (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) and the Prefix-SID sub-TLV of the attribute is never decoded (internal/core/bgp/attribute/tunnel_encap.go:145) |
| `RFC9012-3.1-1` | The Reserved subfield of the Tunnel Egress Endpoint sub-TLV SHOULD be originated as zero (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-9` | The TC field of each label stack entry SHOULD be set to 0, unless changed by policy at the originator (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-10` | When pushing the label stack onto a packet, the TC of each entry SHOULD be preserved, unless local policy modifies it (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-11` | The TTL field of each label stack entry SHOULD be set to 255, unless changed to another non-zero value by policy at the originator (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-12` | When pushing the label stack onto a packet, the TTL of each entry SHOULD be preserved, unless local policy modifies it to another non-zero value (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.7-1` | The Prefix-SID sub-TLV SHOULD only be included in a BGP UPDATE for an address family for which RFC 8669 defines behavior, namely IPv4/IPv6 Labeled Unicast (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-3` | Filtering of the attribute on incoming UPDATEs SHOULD be possible on a per-BGP-session basis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-7` | Filtering of the attribute on outgoing UPDATEs SHOULD be possible on a per-BGP-session basis (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-1.5-1` | The Load-Balancing Block sub-TLV MAY be included in any Tunnel Encapsulation attribute where load balancing is desired (§1.5) | MAY | 1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1-6` | The Tunnel Egress Endpoint sub-TLV MAY have a Value field whose Address Family subfield contains 0, meaning the egress endpoint is the next hop (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1-9` | The restriction rejecting a "Martian" tunnel egress address MAY be relaxed by explicit configuration (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1.1-1` | The procedure to validate that the Address subfield belongs to the route's origin AS MAY be applied (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.1.1-2` | Configuration MAY allow a tunnel egress endpoint to reside in an AS other than Route_AS, within a configured set of permitted AS numbers (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.3.1-1` | An implementation MAY provide a facility to use policy to filter or modify the DS field (§3.3.1) | MAY | 3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.4.1-1` | The Protocol Type sub-TLV MAY be included in a TLV to indicate the payload types allowed to be encapsulated (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.4.2-1` | The Color sub-TLV MAY be used to "color" the corresponding Tunnel TLV (§3.4.2) | MAY | 3.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-3.6-14` | If an invalid or unsupported label stack is received, the tunnel MAY be treated as not feasible per Section 6 (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-1` | The BGP Tunnel Encapsulation attribute MAY be carried in UPDATEs of AFI/SAFI 1/1, 2/1, 1/4, 2/4, 1/128, 2/128, or 25/70 (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-3` | A BGP speaker MAY have local policy that influences the choice of tunnel and how the encapsulation is formed (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-6-4` | A BGP speaker MAY have local policy telling it to ignore the Tunnel Encapsulation attribute entirely or in part (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-11-4` | Finer filtering granularities (per route and/or per attribute TLV) MAY be supported (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9012-13-20` | An implementation MAY log a message when it encounters a sub-TLV that is meaningless for the tunnel type (§13) | MAY | 13 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9012-3.1-2`](#rfc9012-3.1-2) The Reserved subfield of the Tunnel Egress Endpoint sub-TLV MUST be disregarded on receipt (§3.1) | {gap}, no test | no code decodes the Tunnel Egress Endpoint sub-TLV, so ze holds no Reserved subfield to disregard: SubTLVs returns raw type and value pairs (internal/core/bgp/attribute/tunnel_encap.go:104) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.1-4`](#rfc9012-3.1-4) If the Address Family subfield is IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix) (§3.1) | {gap}, no test | ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv4 Address Family carrying an address of any other width is accepted unchanged |
| [`RFC9012-3.1-5`](#rfc9012-3.1-5) If the Address Family subfield is IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix) (§3.1) | {gap}, no test | ze validates no Tunnel Egress Endpoint sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so an IPv6 Address Family carrying an address of any other width is accepted unchanged |
| [`RFC9012-3.1-7`](#rfc9012-3.1-7) If the Address Family subfield contains 0, the Length field of the Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06) (§3.1) | {gap}, no test | the Length of a Tunnel Egress Endpoint sub-TLV is never checked against its Address Family: SubTLVs reads the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.1-8`](#rfc9012-3.1-8) When the attribute is carried in an UPDATE of one of the AFI/SAFIs of Section 6, each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV (§3.1) | {gap}, no test | nothing counts Tunnel Egress Endpoint sub-TLVs per TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), so a TLV with none or with several is accepted identically |
| [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3) If the forwarding route changes, the address-subfield validation procedure MUST be reapplied (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: the Section 3.1.1 origin-AS validation procedure is itself optional and ze applies none, so there is no procedure to reapply; grep -rniE 'route_as\|egress.?endpoint' over internal, pkg and cmd matches only this RFC's own tests, and the best-path comparison consults no attribute 23 (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1) The reserved (R) Flags bits of the VXLAN/NVGRE Encapsulation sub-TLV MUST always be set to 0 by the originator (§3.2.1, §3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no VXLAN or NVGRE Encapsulation sub-TLV, so it sets no R bits: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encapsulation sub-?tlv\|buildEncap' over internal, pkg and cmd finds no Encapsulation sub-TLV builder |
| [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3) Any receiving router MUST ignore the reserved (R) Flags bits upon receipt (§3.2.1, §3.2.2) | {gap}, no test | ze decodes no VXLAN or NVGRE Encapsulation sub-TLV, so no code reads the Flags octet at all: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4) If the V bit is 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2) | {gap}, no test | the V bit is never read and the VN-ID field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5) If the M bit is 0, the MAC Address field MUST be set to all zeroes on transmission and disregarded on receipt (§3.2.1, §3.2.2) | {gap}, no test | the M bit is never read and the MAC Address field never located: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6) The Reserved field of the VXLAN/NVGRE Encapsulation sub-TLV MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2) | {gap}, no test | the Reserved field of the Encapsulation sub-TLV is never located, because the sub-TLV itself is never decoded: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1) Unless a key value is being advertised, the GRE or MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present (§3.2.4, §3.2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no GRE or MPLS-in-GRE Encapsulation sub-TLV and has no GRE key to advertise in BGP: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331); the only GRE key in ze belongs to interface tunnels (internal/plugins/iface/netlink/tunnel_linux.go:35) |
| [`RFC9012-3.3-1`](#rfc9012-3.3-1) An outer Encapsulation sub-TLV in a TLV whose tunnel type does not use the corresponding outer encapsulation MUST be treated as an unrecognized type of sub-TLV (§3.3) | {gap}, no test | ze recognizes no Encapsulation sub-TLV under any tunnel type, so it never makes the tunnel-type-specific determination this requires: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1) If the reserved value zero is received in a UDP Destination Port sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.3.2) | {gap}, no test | no UDP Destination Port sub-TLV decoder exists, so the reserved value zero is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2) Packets with payload types other than the one signaled by the Protocol Type sub-TLV MUST NOT be encapsulated in the relevant tunnel (§3.4.1) | {gap}, no test | ze builds no tunnel encapsulation from the attribute, so no payload type is ever checked against a Protocol Type sub-TLV: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the FIB receives no tunnel from BGP (grep for SubTLVs over internal/plugins/fib matches nothing) |
| [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3) If the reserved value 0xFFFF is received in a Protocol Type sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.4.1) | {gap}, no test | no Protocol Type sub-TLV decoder exists, so the reserved value 0xFFFF is neither detected nor treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4) For an "X-in-Y" tunnel, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored (§3.4.1) | {gap}, no test | ze reads no Protocol Type sub-TLV and forms no X-in-Y tunnel, so a mismatched payload type is never identified: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2) If a Color sub-TLV's Length is other than 8, or the first two octets of its Value are not 0x030b, the sub-TLV MUST be treated as an unrecognized sub-TLV (§3.4.2) | {gap}, no test | no Color sub-TLV decoder exists, so neither its Length nor its leading 0x030b octets are checked: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-1`](#rfc9012-3.5-1) If the attribute is attached to an UPDATE of a non-labeled address family, the Embedded Label Handling sub-TLV MUST be disregarded (§3.5) | {gap}, no test | the Embedded Label Handling sub-TLV is never decoded and the attribute is never correlated with the address family of the UPDATE carrying it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-2`](#rfc9012-3.5-2) If the Embedded Label Handling sub-TLV is in a TLV whose tunnel type has no virtual network identifier, it MUST be disregarded (§3.5) | {gap}, no test | no code relates a sub-TLV to whether its tunnel type uses a virtual network identifier: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.5-4`](#rfc9012-3.5-4) If any Embedded Label Handling value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed per Section 13 (§3.5) | {gap}, no test | the Embedded Label Handling value is never read, so a value outside 1 and 2 is not detected and not treated as malformed: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414) |
| [`RFC9012-3.6-1`](#rfc9012-3.6-1) When the MPLS label stack is pushed onto a packet, its topmost-to-bottommost ordering MUST be preserved (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no ordering is preserved or lost: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/mpls matches nothing |
| [`RFC9012-3.6-2`](#rfc9012-3.6-2) If a TLV contains an MPLS Label Stack sub-TLV, that label stack MUST be pushed onto the packet before any other labels (§3.6) | {gap}, no test | no MPLS Label Stack sub-TLV is decoded and no push order is established: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-3`](#rfc9012-3.6-3) For a labeled address family, the MPLS Label Stack sub-TLV contents MUST be pushed before the label embedded in the NLRI (§3.6) | {gap}, no test | ze never combines an MPLS Label Stack sub-TLV with the label embedded in a labeled-unicast NLRI: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-4`](#rfc9012-3.6-4) If the TLV identifies a tunnel type using virtual network identifiers, the MPLS Label Stack sub-TLV contents MUST be pushed before the Section 9 procedures are applied (§3.6) | {gap}, no test | ze applies no virtual network identifier procedure, so nothing is sequenced against it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-5`](#rfc9012-3.6-5) The number of label stack entries MUST be determined from the Sub-TLV Length field (§3.6) | {gap}, no test | no label stack entries are ever counted: SubTLVs uses the length field only to walk to the next sub-TLV (internal/core/bgp/attribute/tunnel_encap.go:125) and no caller reads sub-TLV type 10 |
| [`RFC9012-3.6-6`](#rfc9012-3.6-6) When pushed onto a packet that already has a label stack, the S bits of all pushed entries MUST be cleared (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no S bit is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-7`](#rfc9012-3.6-7) When pushed onto a packet with no existing label stack, the S bit of the bottommost pushed entry MUST be set (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no bottom-of-stack bit is set: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-8`](#rfc9012-3.6-8) When pushed onto a packet with no existing label stack, the S bit of every other pushed entry MUST be cleared (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so no S bit of a non-bottom entry is cleared: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.6-13`](#rfc9012-3.6-13) If any label stack entry has a TTL of zero, the router pushing the stack MUST change it to a non-zero value (§3.6) | {gap}, no test | ze pushes no label stack from the attribute, so a zero TTL in a received entry is never rewritten: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.7-2`](#rfc9012-3.7-2) If a Prefix-SID sub-TLV is included in a BGP UPDATE for an address family other than IPv4/IPv6 Labeled Unicast, it MUST be ignored (§3.7) | {gap}, no test | the Prefix-SID sub-TLV of the Tunnel Encapsulation attribute is never decoded, so it is neither used nor deliberately ignored per address family: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-3.7-3`](#rfc9012-3.7-3) If an Originator SRGB is specified in the Prefix-SID sub-TLV, that SRGB MUST be interpreted as the SRGB used by the tunnel's egress endpoint (§3.7) | {gap}, no test | no Originator SRGB is read from the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the only SRGB handling in ze belongs to OSPF segment routing, where the label is computed from the next-hop router's advertised SRGB rather than from any BGP attribute (internal/plugins/ospf/sr_install.go:69, :100, :139) |
| [`RFC9012-3.7-4`](#rfc9012-3.7-4) If a Label-Index is present and the tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack (§3.7) | {gap}, no test | no Label-Index is read from the attribute and no label is pushed from it: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-4.1-1`](#rfc9012-4.1-1) Where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no barebones Tunnel TLV and no Encapsulation Extended Community, so the choice this requires never arises: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), and grep -rniE 'encap.*extended.?communit\|extended.?communit.*encap' over internal, pkg and cmd matches nothing |
| [`RFC9012-4.1-2`](#rfc9012-4.1-2) An implementation MUST be prepared to process a tunnel received encoded as a barebones TLV (§4.1) | {gap}, no test | a barebones TLV parses and is re-advertised, but nothing processes it as a tunnel: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and no forwarding consumer reads the attribute (grep for TunnelEncap over internal/plugins/fib matches nothing) |
| [`RFC9012-4.1-3`](#rfc9012-4.1-3) For an "X-in-Y" tunnel signaled via the Encapsulation Extended Community, packets with other payload types MUST NOT be carried through the tunnel (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no tunnel dataplane keyed on the Encapsulation Extended Community, so no payload of any type is carried through a tunnel signaled by one. Ze does READ the extended community -- ParseExtendedCommunities copies every 8-octet value in and ExtendedCommunities.WriteTo copies them back out (internal/core/bgp/attribute/community.go:275, :250) -- but the carriage is opaque: nothing decodes type 0x03 sub-type 0x0c into a tunnel type, and grep -rniE 'encap.*extended.?communit' over internal, pkg and cmd matches no such decoder. With no tunnel established from the community, the payload-type restriction has no behavior to constrain |
| [`RFC9012-4.2-1`](#rfc9012-4.2-1) If a Router's MAC Extended Community and a VXLAN/NVGRE Encapsulation sub-TLV carry conflicting MACs, the Router's MAC Extended Community value MUST be used (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes neither side of the conflict into a MAC address, so the two can never disagree. An extended community reaches ze only as 8 opaque octets that are parsed in and re-encoded out unchanged (internal/core/bgp/attribute/community.go:275, :250) with no Router's MAC decoder -- grep -rniE "router.?s? mac\|RouterMAC" over internal, pkg and cmd matches only the VRRP virtual MAC (internal/plugins/vrrp/packet/packet.go:94) -- and the only Tunnel Encapsulation sub-TLV ze decodes is Preference (TunnelTLV.Preference, internal/core/bgp/attribute/tunnel_encap.go:145), never a VXLAN or NVGRE Encapsulation sub-TLV |
| [`RFC9012-4.3-1`](#rfc9012-4.3-1) The Flags field of the Color Extended Community MUST be set to zero by the originator and ignored by the receiver (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: neither half of this requirement has a producer that could violate it. Ze originates no Color Extended Community -- grep -rniE '030b\|color.?extended' over internal, pkg and cmd matches no writer -- so nothing sets the Flags field at all. On receipt ze does read the extended community, but only as 8 opaque octets: ParseExtendedCommunities copies each value whole (internal/core/bgp/attribute/community.go:275) and ExtendedCommunities.WriteTo copies it back (:250) without ever addressing the Flags subfield, which is exactly the "ignored by the receiver" behavior, reached by having no field decoder rather than by a check |
| [`RFC9012-6-2`](#rfc9012-6-2) Router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U (§6) | {gap}, no test | ze sends no packet through a tunnel named by the attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib and internal/component/iface matches nothing, so no feasible tunnel is ever selected |
| [`RFC9012-7.1-1`](#rfc9012-7.1-1) A route whose Tunnel Encapsulation attribute includes no feasible tunnel MUST NOT be considered resolvable for the route resolvability condition (§7.1) | {gap}, no test | tunnel feasibility takes no part in route resolvability: comparePair implements the decision process with no tunnel step (internal/component/bgp/plugins/rib/bestpath.go:307) and a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-8-1`](#rfc9012-8-1) When U1 has no attribute and its next hop resolves to U2 which has the attribute, packet P MUST be sent through one of U2's tunnels (§8) | {gap}, no test | a route's next hop is never followed to another route's Tunnel Encapsulation attribute: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and the RIB stores the attribute without linking it to a resolving route (internal/component/bgp/plugins/rib/bestpath.go:307) |
| [`RFC9012-8-2`](#rfc9012-8-2) Packet P MUST NOT be sent through a tunnel whose TLV has Color sub-TLVs unless U1 carries a matching Color Extended Community (§8) | {gap}, no test | no Color sub-TLV is decoded and no Color Extended Community is matched against one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and extended communities are carried as opaque 8-octet values, parsed in and re-encoded out unchanged with no type-aware decoder (internal/core/bgp/attribute/community.go:275, :250) |
| [`RFC9012-10-1`](#rfc9012-10-1) Any document specifying joint use of the Tunnel Encapsulation attribute with other tunnel-signaling mechanisms MUST provide details on how interactions are handled (§10) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the authors of a specification that combines the attribute with another tunnel-signaling mechanism, not an implementation; ze ships no such specification and there is no code that could satisfy or violate it |
| [`RFC9012-11-1`](#rfc9012-11-1) The Tunnel Encapsulation attribute MUST be used only within a well-defined scope (§11) | {gap}, no test | ze enforces no scope for the attribute: it is parsed from any peer and re-advertised unchanged (a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151)), and no configuration bounds where it may travel (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-2`](#rfc9012-11-2) Any BGP speaker that understands the attribute MUST be able to filter it from incoming BGP UPDATE messages (§11) | {gap}, no test | no filter can drop attribute 23 on ingress: setBlockAllowedKeys is a closed set of modifier keys with no tunnel-encapsulation entry (internal/component/bgp/plugins/filter_modify/config.go:30) and communityDirectives covers only the three community attributes (internal/component/bgp/reactor/filter_delta.go:268) |
| [`RFC9012-11-5`](#rfc9012-11-5) For each EBGP session, filtering of the attribute on incoming UPDATEs MUST be enabled by default (§11) | {gap}, no test | there is no attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-6`](#rfc9012-11-6) Any BGP speaker that understands the attribute MUST be able to filter it from outgoing BGP UPDATE messages (§11) | {gap}, no test | no filter can drop attribute 23 on egress: the egress path rebuilds attributes through AttrModHandlers keyed by attribute code (internal/component/bgp/reactor/forward_build.go:211) and no handler or directive names code 23 (internal/component/bgp/reactor/filter_delta.go:268) |
| [`RFC9012-11-8`](#rfc9012-11-8) For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11) | {gap}, no test | there is no outgoing attribute-23 filter to enable, so none is on by default for EBGP sessions: setBlockAllowedKeys has no tunnel-encapsulation key (internal/component/bgp/plugins/filter_modify/config.go:30) |
| [`RFC9012-11-9`](#rfc9012-11-9) Any BGP speaker that understands the Encapsulation Extended Community MUST be able to filter it from incoming BGP UPDATE messages (§11) | {gap}, no test | the community filter removes an extended community only by exact 8-octet value (parseExtendedWire, internal/component/bgp/plugins/filter_community/config.go:230, applied by genericCommunityHandler, handler.go:29), so the Encapsulation Extended Community cannot be filtered as a type |
| [`RFC9012-11-10`](#rfc9012-11-10) It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11) | {gap}, no test | the same exact-value restriction applies on egress: extended-community-remove takes one 8-octet value (internal/component/bgp/reactor/filter_delta.go:268) and no rule matches the Encapsulation Extended Community by its type and sub-type |
| [`RFC9012-11-11`](#rfc9012-11-11) Filtering of the Encapsulation Extended Community MUST be enabled by default for EBGP sessions (§11) | {gap}, no test | no filtering of the Encapsulation Extended Community is configured by default on EBGP sessions: every community operation comes from an explicit policy entry (parseModifyDefs, internal/component/bgp/plugins/filter_modify/config.go:50) |
| [`RFC9012-13-1`](#rfc9012-13-1) The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13) | {gap}, no test | ze never checks that a TLV ends where its final sub-TLV ends: ParseTunnelEncap walks the outer TLVs only and copies each value whole (internal/core/bgp/attribute/tunnel_encap.go:39), and SubTLVs is called on demand by display code rather than at parse time (internal/test/decode/decode_tunnel_encap.go:34) |
| [`RFC9012-13-2`](#rfc9012-13-2) If the final octet of a TLV is not the final octet of its final sub-TLV, the TLV MUST be considered malformed and the RFC 7606 "treat-as-withdraw" procedure applied (§13) | {gap}, no test | a TLV whose final sub-TLV overruns it produces no treat-as-withdraw: the misalignment is not detected at parse time (internal/core/bgp/attribute/tunnel_encap.go:39) and attrValidators carries no entry for attribute code 23 (internal/component/bgp/message/rfc7606.go:414), so the UPDATE is accepted in full |
| [`RFC9012-13-4`](#rfc9012-13-4) An unrecognized tunnel type MUST be interpreted as if that TLV had not been present (§13) | {gap}, no test | no code interprets a tunnel type at all, so an unrecognized one is not deliberately treated as absent: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and nothing branches on TunnelTLV.TunnelType outside display (internal/test/decode/decode_tunnel_encap.go:28) |
| [`RFC9012-13-6`](#rfc9012-13-6) Tunnel Egress Endpoint, Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID sub-TLVs MUST NOT occur more than once in a given Tunnel TLV (§13) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates none of the seven sub-TLVs this names: buildTunnelEncap emits tunnel type 15 alone, carrying only the RFC 9830 sub-TLVs preference, binding SID, priority, segment list and the two names (internal/component/bgp/plugins/nlri/srpolicy/config.go:331), so no origination of ze's can repeat one |
| [`RFC9012-13-13`](#rfc9012-13-13) A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be ignored in its entirety (§13) | {gap}, no test | a malformed Tunnel Egress Endpoint sub-TLV is never identified, so the TLV containing it is not ignored: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-13-14`](#rfc9012-13-14) A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be removed from the attribute before the route is distributed (§13) | {gap}, no test | a TLV is never removed from the attribute before distribution: WriteTo re-emits every parsed TLV verbatim (internal/core/bgp/attribute/tunnel_encap.go:71) and no code inspects a Tunnel Egress Endpoint sub-TLV to decide otherwise |
| [`RFC9012-13-15`](#rfc9012-13-15) Within an UPDATE of an AFI/SAFI listed in Section 6, a TLV not containing exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV (§13) | {gap}, no test | the count of Tunnel Egress Endpoint sub-TLVs in a TLV is never taken, so a TLV without exactly one is not treated as carrying a malformed one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151) |
| [`RFC9012-13-17`](#rfc9012-13-17) A meaningless sub-TLV MUST NOT affect the creation of the encapsulation header (§13) | {gap}, no test | ze creates no encapsulation header from the attribute, so no sub-TLV meaningless or otherwise contributes to one: a received Tunnel Encapsulation attribute is kept as raw TLV bytes (internal/core/bgp/attribute/tunnel_encap.go:39) and the only sub-TLV ze decodes is Preference (TunnelTLV.Preference, tunnel_encap.go:145, whose type/length gate is at :151), and grep for TunnelEncap over internal/plugins/fib matches nothing |
| [`RFC9012-15-1`](#rfc9012-15-1) SR domain boundary routers MUST filter any external traffic, so the duty to filter extends to all routers participating in Prefix-SID tunnels (§15) | {gap}, no test | ze filters no traffic and no attribute at an SR domain boundary: there is no attribute-23 filter to apply (setBlockAllowedKeys, internal/component/bgp/plugins/filter_modify/config.go:30) and the Prefix-SID sub-TLV of the attribute is never decoded (internal/core/bgp/attribute/tunnel_encap.go:145) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9012-3.1-2`](#rfc9012-3.1-2)

The Reserved subfield of the Tunnel Egress Endpoint sub-TLV MUST be disregarded on receipt (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-2, so no unit is bound to it.

### [`RFC9012-3.1-3`](#rfc9012-3.1-3)

The Reserved subfield of the Tunnel Egress Endpoint sub-TLV MUST be propagated unchanged (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L338) | unit/verify | unproven |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L335) | unit/verify | unproven |

### [`RFC9012-3.1-4`](#rfc9012-3.1-4)

If the Address Family subfield is IPv4, the Address subfield MUST contain an IPv4 address (a /32 IPv4 prefix) (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-4, so no unit is bound to it.

### [`RFC9012-3.1-5`](#rfc9012-3.1-5)

If the Address Family subfield is IPv6, the Address subfield MUST contain an IPv6 address (a /128 IPv6 prefix) (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-5, so no unit is bound to it.

### [`RFC9012-3.1-7`](#rfc9012-3.1-7)

If the Address Family subfield contains 0, the Length field of the Tunnel Egress Endpoint sub-TLV MUST contain the value 6 (0x06) (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-7, so no unit is bound to it.

### [`RFC9012-3.1-8`](#rfc9012-3.1-8)

When the attribute is carried in an UPDATE of one of the AFI/SAFIs of Section 6, each TLV MUST have one, and only one, Tunnel Egress Endpoint sub-TLV (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1-8, so no unit is bound to it.

### [`RFC9012-3.1.1-3`](#rfc9012-3.1.1-3)

If the forwarding route changes, the address-subfield validation procedure MUST be reapplied (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.1.1-3, so no unit is bound to it.

### [`RFC9012-3.2.1-1`](#rfc9012-3.2.1-1)

The reserved (R) Flags bits of the VXLAN/NVGRE Encapsulation sub-TLV MUST always be set to 0 by the originator (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-1, so no unit is bound to it.

### [`RFC9012-3.2.1-2`](#rfc9012-3.2.1-2)

Intermediate routers MUST propagate the reserved (R) Flags bits without modification (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L349) | unit/verify | unproven |
| positive | [`TestRFC9012ReservedOctetsPropagateUnchanged`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L346) | unit/verify | unproven |

### [`RFC9012-3.2.1-3`](#rfc9012-3.2.1-3)

Any receiving router MUST ignore the reserved (R) Flags bits upon receipt (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-3, so no unit is bound to it.

### [`RFC9012-3.2.1-4`](#rfc9012-3.2.1-4)

If the V bit is 0, the VN-ID field MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-4, so no unit is bound to it.

### [`RFC9012-3.2.1-5`](#rfc9012-3.2.1-5)

If the M bit is 0, the MAC Address field MUST be set to all zeroes on transmission and disregarded on receipt (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-5, so no unit is bound to it.

### [`RFC9012-3.2.1-6`](#rfc9012-3.2.1-6)

The Reserved field of the VXLAN/NVGRE Encapsulation sub-TLV MUST be set to zero on transmission and disregarded on receipt (§3.2.1, §3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.1-6, so no unit is bound to it.

### [`RFC9012-3.2.4-1`](#rfc9012-3.2.4-1)

Unless a key value is being advertised, the GRE or MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present (§3.2.4, §3.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.2.4-1, so no unit is bound to it.

### [`RFC9012-3.3-1`](#rfc9012-3.3-1)

An outer Encapsulation sub-TLV in a TLV whose tunnel type does not use the corresponding outer encapsulation MUST be treated as an unrecognized type of sub-TLV (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.3-1, so no unit is bound to it.

### [`RFC9012-3.3.2-1`](#rfc9012-3.3.2-1)

If the reserved value zero is received in a UDP Destination Port sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.3.2-1, so no unit is bound to it.

### [`RFC9012-3.4.1-2`](#rfc9012-3.4.1-2)

Packets with payload types other than the one signaled by the Protocol Type sub-TLV MUST NOT be encapsulated in the relevant tunnel (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-2, so no unit is bound to it.

### [`RFC9012-3.4.1-3`](#rfc9012-3.4.1-3)

If the reserved value 0xFFFF is received in a Protocol Type sub-TLV, the sub-TLV MUST be treated as malformed per the rules of Section 13 (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-3, so no unit is bound to it.

### [`RFC9012-3.4.1-4`](#rfc9012-3.4.1-4)

For an "X-in-Y" tunnel, a Protocol Type sub-TLV specifying anything other than "X" MUST be ignored (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.1-4, so no unit is bound to it.

### [`RFC9012-3.4.2-2`](#rfc9012-3.4.2-2)

If a Color sub-TLV's Length is other than 8, or the first two octets of its Value are not 0x030b, the sub-TLV MUST be treated as an unrecognized sub-TLV (§3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.4.2-2, so no unit is bound to it.

### [`RFC9012-3.5-1`](#rfc9012-3.5-1)

If the attribute is attached to an UPDATE of a non-labeled address family, the Embedded Label Handling sub-TLV MUST be disregarded (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-1, so no unit is bound to it.

### [`RFC9012-3.5-2`](#rfc9012-3.5-2)

If the Embedded Label Handling sub-TLV is in a TLV whose tunnel type has no virtual network identifier, it MUST be disregarded (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-2, so no unit is bound to it.

### [`RFC9012-3.5-3`](#rfc9012-3.5-3)

When the Embedded Label Handling sub-TLV is ignored, it MUST NOT be stripped from the TLV before the route is propagated (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L372) | unit/verify | unproven |
| positive | [`TestRFC9012EmbeddedLabelHandlingNotStripped`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L364) | unit/verify | unproven |

### [`RFC9012-3.5-4`](#rfc9012-3.5-4)

If any Embedded Label Handling value other than 1 or 2 is carried, the sub-TLV MUST be considered malformed per Section 13 (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.5-4, so no unit is bound to it.

### [`RFC9012-3.6-1`](#rfc9012-3.6-1)

When the MPLS label stack is pushed onto a packet, its topmost-to-bottommost ordering MUST be preserved (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-1, so no unit is bound to it.

### [`RFC9012-3.6-2`](#rfc9012-3.6-2)

If a TLV contains an MPLS Label Stack sub-TLV, that label stack MUST be pushed onto the packet before any other labels (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-2, so no unit is bound to it.

### [`RFC9012-3.6-3`](#rfc9012-3.6-3)

For a labeled address family, the MPLS Label Stack sub-TLV contents MUST be pushed before the label embedded in the NLRI (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-3, so no unit is bound to it.

### [`RFC9012-3.6-4`](#rfc9012-3.6-4)

If the TLV identifies a tunnel type using virtual network identifiers, the MPLS Label Stack sub-TLV contents MUST be pushed before the Section 9 procedures are applied (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-4, so no unit is bound to it.

### [`RFC9012-3.6-5`](#rfc9012-3.6-5)

The number of label stack entries MUST be determined from the Sub-TLV Length field (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-5, so no unit is bound to it.

### [`RFC9012-3.6-6`](#rfc9012-3.6-6)

When pushed onto a packet that already has a label stack, the S bits of all pushed entries MUST be cleared (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-6, so no unit is bound to it.

### [`RFC9012-3.6-7`](#rfc9012-3.6-7)

When pushed onto a packet with no existing label stack, the S bit of the bottommost pushed entry MUST be set (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-7, so no unit is bound to it.

### [`RFC9012-3.6-8`](#rfc9012-3.6-8)

When pushed onto a packet with no existing label stack, the S bit of every other pushed entry MUST be cleared (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-8, so no unit is bound to it.

### [`RFC9012-3.6-13`](#rfc9012-3.6-13)

If any label stack entry has a TTL of zero, the router pushing the stack MUST change it to a non-zero value (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.6-13, so no unit is bound to it.

### [`RFC9012-3.7-2`](#rfc9012-3.7-2)

If a Prefix-SID sub-TLV is included in a BGP UPDATE for an address family other than IPv4/IPv6 Labeled Unicast, it MUST be ignored (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.7-2, so no unit is bound to it.

### [`RFC9012-3.7-3`](#rfc9012-3.7-3)

If an Originator SRGB is specified in the Prefix-SID sub-TLV, that SRGB MUST be interpreted as the SRGB used by the tunnel's egress endpoint (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.7-3, so no unit is bound to it.

### [`RFC9012-3.7-4`](#rfc9012-3.7-4)

If a Label-Index is present and the tunnel is from a labeled address family, the corresponding MPLS label MUST be pushed on the packet's label stack (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-3.7-4, so no unit is bound to it.

### [`RFC9012-4.1-1`](#rfc9012-4.1-1)

Where a tunnel could be encoded using a barebones TLV, it MUST be encoded using the corresponding Encapsulation Extended Community (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-1, so no unit is bound to it.

### [`RFC9012-4.1-2`](#rfc9012-4.1-2)

An implementation MUST be prepared to process a tunnel received encoded as a barebones TLV (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-2, so no unit is bound to it.

### [`RFC9012-4.1-3`](#rfc9012-4.1-3)

For an "X-in-Y" tunnel signaled via the Encapsulation Extended Community, packets with other payload types MUST NOT be carried through the tunnel (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.1-3, so no unit is bound to it.

### [`RFC9012-4.2-1`](#rfc9012-4.2-1)

If a Router's MAC Extended Community and a VXLAN/NVGRE Encapsulation sub-TLV carry conflicting MACs, the Router's MAC Extended Community value MUST be used (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.2-1, so no unit is bound to it.

### [`RFC9012-4.3-1`](#rfc9012-4.3-1)

The Flags field of the Color Extended Community MUST be set to zero by the originator and ignored by the receiver (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-4.3-1, so no unit is bound to it.

### [`RFC9012-4.3-2`](#rfc9012-4.3-2)

The Color Extended Community value MUST NOT be changed when propagating the extended community (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L399) | unit/verify | unproven |
| positive | [`TestRFC9012ColorExtendedCommunityUnchangedOnPropagation`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L396) | unit/verify | unproven |

### [`RFC9012-6-2`](#rfc9012-6-2)

Router R MUST send packet P through one of the feasible tunnels identified in the Tunnel Encapsulation attribute of UPDATE U (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-6-2, so no unit is bound to it.

### [`RFC9012-7.1-1`](#rfc9012-7.1-1)

A route whose Tunnel Encapsulation attribute includes no feasible tunnel MUST NOT be considered resolvable for the route resolvability condition (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-7.1-1, so no unit is bound to it.

### [`RFC9012-8-1`](#rfc9012-8-1)

When U1 has no attribute and its next hop resolves to U2 which has the attribute, packet P MUST be sent through one of U2's tunnels (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-8-1, so no unit is bound to it.

### [`RFC9012-8-2`](#rfc9012-8-2)

Packet P MUST NOT be sent through a tunnel whose TLV has Color sub-TLVs unless U1 carries a matching Color Extended Community (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-8-2, so no unit is bound to it.

### [`RFC9012-10-1`](#rfc9012-10-1)

Any document specifying joint use of the Tunnel Encapsulation attribute with other tunnel-signaling mechanisms MUST provide details on how interactions are handled (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-10-1, so no unit is bound to it.

### [`RFC9012-11-1`](#rfc9012-11-1)

The Tunnel Encapsulation attribute MUST be used only within a well-defined scope (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-1, so no unit is bound to it.

### [`RFC9012-11-2`](#rfc9012-11-2)

Any BGP speaker that understands the attribute MUST be able to filter it from incoming BGP UPDATE messages (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-2, so no unit is bound to it.

### [`RFC9012-11-5`](#rfc9012-11-5)

For each EBGP session, filtering of the attribute on incoming UPDATEs MUST be enabled by default (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-5, so no unit is bound to it.

### [`RFC9012-11-6`](#rfc9012-11-6)

Any BGP speaker that understands the attribute MUST be able to filter it from outgoing BGP UPDATE messages (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-6, so no unit is bound to it.

### [`RFC9012-11-8`](#rfc9012-11-8)

For each EBGP session, filtering of the attribute on outgoing UPDATEs MUST be enabled by default (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-8, so no unit is bound to it.

### [`RFC9012-11-9`](#rfc9012-11-9)

Any BGP speaker that understands the Encapsulation Extended Community MUST be able to filter it from incoming BGP UPDATE messages (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-9, so no unit is bound to it.

### [`RFC9012-11-10`](#rfc9012-11-10)

It MUST be possible to filter the Encapsulation Extended Community from outgoing messages (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-10, so no unit is bound to it.

### [`RFC9012-11-11`](#rfc9012-11-11)

Filtering of the Encapsulation Extended Community MUST be enabled by default for EBGP sessions (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-11-11, so no unit is bound to it.

### [`RFC9012-13-1`](#rfc9012-13-1)

The final octet of a TLV MUST also be the final octet of its final sub-TLV (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-1, so no unit is bound to it.

### [`RFC9012-13-2`](#rfc9012-13-2)

If the final octet of a TLV is not the final octet of its final sub-TLV, the TLV MUST be considered malformed and the RFC 7606 "treat-as-withdraw" procedure applied (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-2, so no unit is bound to it.

### [`RFC9012-13-3`](#rfc9012-13-3)

A TLV whose tunnel type is unrecognized MUST NOT cause the attribute to be considered malformed (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L149) | unit/verify | unproven |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L114) | unit/verify | unproven |

### [`RFC9012-13-4`](#rfc9012-13-4)

An unrecognized tunnel type MUST be interpreted as if that TLV had not been present (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-4, so no unit is bound to it.

### [`RFC9012-13-5`](#rfc9012-13-5)

If the route is propagated with the attribute, an unrecognized TLV MUST remain in the attribute (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012UnrecognizedTLVRemovalIsObservable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L137) | unit/verify | unproven |
| positive | [`TestRFC9012UnrecognizedTunnelTypeIsCarried`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L123) | unit/verify | unproven |

### [`RFC9012-13-6`](#rfc9012-13-6)

Tunnel Egress Endpoint, Encapsulation, DS, UDP Destination Port, Embedded Label Handling, MPLS Label Stack, and Prefix-SID sub-TLVs MUST NOT occur more than once in a given Tunnel TLV (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-6, so no unit is bound to it.

### [`RFC9012-13-7`](#rfc9012-13-7)

If such a single-instance sub-TLV occurs more than once, all but the first occurrence of each type MUST be disregarded (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L277) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L262) | unit/verify | unproven |

### [`RFC9012-13-8`](#rfc9012-13-8)

A Tunnel TLV carrying duplicate single-instance sub-TLVs MUST NOT be considered malformed (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L268) | unit/verify | unproven |

### [`RFC9012-13-9`](#rfc9012-13-9)

All the sub-TLVs MUST be propagated if the route carrying the attribute is propagated (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L178) | unit/verify | unproven |

### [`RFC9012-13-10`](#rfc9012-13-10)

If a TLV contains an unrecognized sub-TLV, the BGP speaker MUST process the TLV as if the unrecognized sub-TLV had not been present (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L219) | unit/verify | unproven |
| positive | [`TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L211) | unit/verify | unproven |

### [`RFC9012-13-11`](#rfc9012-13-11)

If the route is propagated with the attribute, an unrecognized sub-TLV MUST remain in the attribute (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L195) | unit/verify | unproven |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L185) | unit/verify | unproven |

### [`RFC9012-13-12`](#rfc9012-13-12)

A malformed sub-TLV MUST be treated as if it were an unrecognized sub-TLV (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L247) | unit/verify | unproven |
| positive | [`TestRFC9012MalformedSubTLVTreatedAsUnrecognized`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L238) | unit/verify | unproven |

### [`RFC9012-13-13`](#rfc9012-13-13)

A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be ignored in its entirety (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-13, so no unit is bound to it.

### [`RFC9012-13-14`](#rfc9012-13-14)

A TLV containing a malformed Tunnel Egress Endpoint sub-TLV MUST be removed from the attribute before the route is distributed (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-14, so no unit is bound to it.

### [`RFC9012-13-15`](#rfc9012-13-15)

Within an UPDATE of an AFI/SAFI listed in Section 6, a TLV not containing exactly one Tunnel Egress Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress Endpoint sub-TLV (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-15, so no unit is bound to it.

### [`RFC9012-13-16`](#rfc9012-13-16)

A sub-TLV that is meaningless for the identified tunnel type MUST be disregarded (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L311) | unit/verify | unproven |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L302) | unit/verify | unproven |

### [`RFC9012-13-17`](#rfc9012-13-17)

A meaningless sub-TLV MUST NOT affect the creation of the encapsulation header (§13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-13-17, so no unit is bound to it.

### [`RFC9012-13-18`](#rfc9012-13-18)

A meaningless sub-TLV MUST NOT be considered malformed (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L154) | unit/verify | unproven |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L295) | unit/verify | unproven |

### [`RFC9012-13-19`](#rfc9012-13-19)

A meaningless sub-TLV MUST NOT be removed from the TLV before the route is distributed (§13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L321) | unit/verify | unproven |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L318) | unit/verify | unproven |

### [`RFC9012-15-1`](#rfc9012-15-1)

SR domain boundary routers MUST filter any external traffic, so the duty to filter extends to all routers participating in Prefix-SID tunnels (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9012-15-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9012, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9012, so its obligations are stated where they were written.
