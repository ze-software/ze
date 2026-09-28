# RFC 9830 - BGP Extensions for the Advertisement of Segment Routing (SR) Policies

Partial. Every requirement this repository extracted from RFC 9830, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 62.5% | 60 of 96 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.2% | 4 of 96 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 96 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 125 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 96 | of 137 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 12 | of 96 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 12.5% | 12 of 96 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 96 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 96 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.8% | 20 of 96 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 59 | of 96 gated MUSTs judged | 25 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 96 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 137 |
| Gated MUST-level | 96 |
| Not applicable, so out of scope | 12 |
| Declared gaps | 20 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 125 |
| Tagged units | 125 |
| Recorded audit verdicts | 59 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9830.md` |
| Requirement shard | `rfc/requirements/rfc9830.md` |
| RFC text | `rfc/full/rfc9830.txt` |

## Enrolment

Enrolled: BGP Extensions for SR Policy

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze originates and carries SR Policy: the SAFI 73 NLRI is written with the mandated 96-bit or 192-bit length for AFI 1 and AFI 2 ([`internal/component/bgp/plugins/nlri/srpolicy/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/types.go)), parsed back (types.go) and split on the wire (split.go), and every candidate-path sub-TLV is encoded into one Tunnel Type 15 TLV of the Tunnel Encapsulation attribute -- preference, MPLS and SRv6 binding SID, priority, weighted segment lists of Type A and Type B segments with the SRv6 Endpoint Behavior and SID Structure, and the policy and candidate-path names ([`internal/component/bgp/plugins/nlri/srpolicy/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/config.go)). Each sub-TLV carries its mandated value length with zero Flags and RESERVED octets, and the MPLS TC, S and TTL bits are zero. On receipt the attribute is kept as raw TLV bytes and re-advertised octet for octet, with the Preference sub-TLV decoded at its mandated 6-octet length ([`internal/core/bgp/attribute/tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/tunnel_encap.go), :104, :145). Requirements bound per line in [`rfc/short/rfc9830.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9830.md).

**What the ledger says remains**

20 MUST-level gaps annotated in [`rfc/short/rfc9830.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9830.md).

- **Encoding defects:** the Binding SID Flags octet is written as 0x10, a bit Section 2.4.2 leaves unassigned ([`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5)); a reserved MPLS label value 0-15 is accepted as a binding SID ([`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11)); SID-structure lengths totalling more than 128 are accepted ([`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4)); and an SR Policy advertisement carries neither a route target nor NO_ADVERTISE ([`RFC9830-4.1-2`](#rfc9830-4.1-2)).
- **Receive-side validation is absent:** SAFI 73 is skipped by the RFC 7606 NLRI check and attribute 23 has no validator, so nothing is treated as withdraw for a wrong or duplicated tunnel type, a bad NLRI length, a missing route target or NO_ADVERTISE, a malformed sub-TLV or a malformed attribute ([`RFC9830-2.2-1`](#rfc9830-2.2-1), [`RFC9830-2.2-3`](#rfc9830-2.2-3), [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1), [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3), [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4), [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5), [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8), [`RFC9830-5-1`](#rfc9830-5-1), [`RFC9830-5-2`](#rfc9830-5-2), [`RFC9830-5-4`](#rfc9830-5-4), [`RFC9830-5-5`](#rfc9830-5-5), [`RFC9830-5-6`](#rfc9830-5-6), [`RFC9830-5-7`](#rfc9830-5-7), [`RFC9830-5-8`](#rfc9830-5-8)).
- **Propagation is family-generic:** NO_ADVERTISE is not honored on egress and there is no eBGP-by-default block for SAFI 73 ([`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1), [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2)). Twelve further MUSTs are annotated not-applicable: ze implements no ENLP sub-TLV, no color-based steering and no SRPM, so it never instantiates, selects or deletes a candidate path.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 60 | one part of the gated population |
| Annotated instead of tested | 36 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **96** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (60):** [`RFC9830-2.1-1`](#rfc9830-2.1-1), [`RFC9830-2.1-2`](#rfc9830-2.1-2), [`RFC9830-2.1-3`](#rfc9830-2.1-3), [`RFC9830-2.3-1`](#rfc9830-2.3-1), [`RFC9830-2.3-3`](#rfc9830-2.3-3), [`RFC9830-2.4-1`](#rfc9830-2.4-1), [`RFC9830-2.4-2`](#rfc9830-2.4-2), [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2), [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3), [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4), [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5), [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6), [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7), [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2), [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4), [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6), [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7), [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8), [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9), [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10), [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4), [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5), [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6), [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7), [`RFC9830-2.4.4-4`](#rfc9830-2.4.4-4), [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5), [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2), [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3), [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4), [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5), [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6), [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7), [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1), [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2), [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3), [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4), [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5), [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1), [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2), [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3), [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4), [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1), [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2), [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3), [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2), [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3), [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3), [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4), [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5), [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6), [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5), [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6), [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7), [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5), [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6), [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7), [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2), [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7), [`RFC9830-4.2.3-6`](#rfc9830-4.2.3-6), [`RFC9830-5-9`](#rfc9830-5-9)

**Annotated instead of tested (36):** [`RFC9830-2.2-1`](#rfc9830-2.2-1), [`RFC9830-2.2-2`](#rfc9830-2.2-2), [`RFC9830-2.2-3`](#rfc9830-2.2-3), [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5), [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11), [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3), [`RFC9830-2.4.3-9`](#rfc9830-2.4.3-9), [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4), [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2), [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3), [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4), [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5), [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6), [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7), [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8), [`RFC9830-3-3`](#rfc9830-3-3), [`RFC9830-4.1-2`](#rfc9830-4.1-2), [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1), [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3), [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4), [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5), [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6), [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8), [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9), [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1), [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2), [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5), [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1), [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2), [`RFC9830-5-1`](#rfc9830-5-1), [`RFC9830-5-2`](#rfc9830-5-2), [`RFC9830-5-4`](#rfc9830-5-4), [`RFC9830-5-5`](#rfc9830-5-5), [`RFC9830-5-6`](#rfc9830-5-6), [`RFC9830-5-7`](#rfc9830-5-7), [`RFC9830-5-8`](#rfc9830-5-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9830-2.1-1` | The AFI used MUST be IPv4(1) or IPv6(2) (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L92). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L120) |
| `RFC9830-2.1-2` | When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) |
| `RFC9830-2.1-3` | A BGP UPDATE message that carries the MP_REACH_NLRI or MP_UNREACH_NLRI attribute with the SR Policy SAFI MUST also carry the BGP mandatory attributes. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178). **negative:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) |
| `RFC9830-2.2-1` | This document specifies the use of the Tunnel Encapsulation Attribute with the SR Policy Tunnel Type and the use of any other Tunnel Type with the SR Policy SAFI MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing rejects a non-SR-Policy tunnel type under SAFI 73. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no attribute-level check ever runs on a received Tunnel Encapsulation attribute, and the attribute itself is kept as raw TLV bytes parsed only on demand (internal/core/bgp/attribute/wire.go:346 and :418, internal/core/bgp/attribute/tunnel_encap.go:39). ze parses and re-advertises the attribute; it applies no treat-as-withdraw |
| `RFC9830-2.2-2` | A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2) | MUST NOT | 2.2 | **positive:** `unit/verify` [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194). **negative:** no negative test. **{single-polarity}:** buildTunnelEncap assembles every configured sub-TLV into a single Tunnel Type 15 TLV and has no path that appends a second one (internal/component/bgp/plugins/nlri/srpolicy/config.go:365-370), so no input to ze's encoder produces the forbidden encoding for a negative to assert. The receive-side obligation to treat two such TLVs as malformed is the separate RFC9830-2.2-3, which is annotated as a gap |
| `RFC9830-2.2-3` | A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy"; such updates MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a Tunnel Encapsulation attribute holding two SR Policy TLVs is accepted. ParseTunnelEncap walks every TLV and appends each one without counting types (internal/core/bgp/attribute/tunnel_encap.go:41-54), and there is no RFC 7606 validator for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no treat-as-withdraw is applied. ze's own encoder emits exactly one such TLV, which is the separate RFC9830-2.2-2 |
| `RFC9830-2.3-1` | If these sub-TLVs are present, a BGP speaker MUST ignore them (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L282). **negative:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293) |
| `RFC9830-2.3-3` | Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L303). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L312) |
| `RFC9830-2.4-1` | For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263). **negative:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L278) |
| `RFC9830-2.4-2` | For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored and MUST NOT considered to be malformed. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155) |
| `RFC9830-2.4.1-2` | The Preference sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.1) | MUST NOT | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) |
| `RFC9830-2.4.1-3` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L262) |
| `RFC9830-2.4.1-4` | The Flags field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) |
| `RFC9830-2.4.1-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L244). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L254) |
| `RFC9830-2.4.1-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) |
| `RFC9830-2.4.1-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L249). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255) |
| `RFC9830-2.4.2-2` | The Binding SID sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.2) | MUST NOT | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) |
| `RFC9830-2.4.2-4` | The value MUST be 18 when a SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) |
| `RFC9830-2.4.2-5` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildBindingSIDSubTLV writes 0x10 into the Binding SID Flags octet (internal/component/bgp/plugins/nlri/srpolicy/config.go:385). Section 2.4.2 assigns only S (bit 0, 0x80) and I (bit 1, 0x40) in that field, so bit 3 is unassigned and is set on transmission. The value is pinned as ExaBGP-interoperable in TestSRPolicyInteropExaBGPSubTLVBytes (internal/component/bgp/plugins/nlri/srpolicy/encode_test.go:118-124), which emits the same octet, so the two implementations agree with each other and not with the RFC |
| `RFC9830-2.4.2-6` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L112). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L194) |
| `RFC9830-2.4.2-7` | RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) |
| `RFC9830-2.4.2-8` | RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L114). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L195) |
| `RFC9830-2.4.2-9` | Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) |
| `RFC9830-2.4.2-10` | Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero and MUST be ignored. (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L116). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L196) |
| `RFC9830-2.4.2-11` | The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2) | MUST NOT | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| `RFC9830-2.4.3-3` | The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315). **negative:** no negative test. **{single-polarity}:** buildSRv6BindingSIDSubTLV always writes the 18-octet form and never appends the SRv6 Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the 26-octet case has no producer and there is no contrasting length to assert. The same 18-versus-26 rule for the Type B segment sub-TLV, where ze DOES produce both, is covered with both polarities under RFC9830-2.4.4.2.2-1 |
| `RFC9830-2.4.3-4` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) |
| `RFC9830-2.4.3-5` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L197) |
| `RFC9830-2.4.3-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) |
| `RFC9830-2.4.3-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L120). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L198) |
| `RFC9830-2.4.3-9` | The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.3) | MUST NOT | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L318). **negative:** no negative test. **{single-polarity}:** the SRv6 Binding SID sub-TLV ze writes always carries the 16-octet SID and never the Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the forbidden combination has no producer to drive a negative from. The parallel Type B rule, where an endpoint-behavior token without a preceding SRv6 SID IS refused, is covered with both polarities under RFC9830-2.4.4.2.2-4 |
| `RFC9830-2.4.4-4` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) |
| `RFC9830-2.4.4-5` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L199) |
| `RFC9830-2.4.4.1-2` | The Weight sub-TLV is OPTIONAL; it MUST NOT appear more than once inside the Segment List sub-TLV. (§2.4.4.1) | MUST NOT | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) |
| `RFC9830-2.4.4.1-3` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) |
| `RFC9830-2.4.4.1-4` | The Flags field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) |
| `RFC9830-2.4.4.1-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L126). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L200) |
| `RFC9830-2.4.4.1-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) |
| `RFC9830-2.4.4.1-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L201) |
| `RFC9830-2.4.4.2.1-1` | Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) |
| `RFC9830-2.4.4.2.1-2` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) |
| `RFC9830-2.4.4.2.1-3` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L202) |
| `RFC9830-2.4.4.2.1-4` | The S bit MUST be zero upon transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) |
| `RFC9830-2.4.4.2.1-5` | The S bit MUST be zero upon transmission and MUST be ignored upon reception. (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L203) |
| `RFC9830-2.4.4.2.2-1` | The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) |
| `RFC9830-2.4.4.2.2-2` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) |
| `RFC9830-2.4.4.2.2-3` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L204) |
| `RFC9830-2.4.4.2.2-4` | The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included. (§2.4.4.2.2) | MUST NOT | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) |
| `RFC9830-2.4.4.2.3-1` | The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425). **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) |
| `RFC9830-2.4.4.2.3-2` | The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) |
| `RFC9830-2.4.4.2.3-3` | If B-Flag appears with Segment Type A, it MUST be ignored. (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206) |
| `RFC9830-2.4.4.2.4-2` | Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) |
| `RFC9830-2.4.4.2.4-3` | Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L154). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207) |
| `RFC9830-2.4.4.2.4-4` | The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| `RFC9830-2.4.5-2` | The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5) | MUST NOT | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| `RFC9830-2.4.5-3` | Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-4` | The Flags field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-5` | The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| `RFC9830-2.4.5-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| `RFC9830-2.4.5-8` | Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| `RFC9830-2.4.6-3` | The Priority sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.6) | MUST NOT | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) |
| `RFC9830-2.4.6-4` | Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 2. (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339). **negative:** `unit/verify` [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) |
| `RFC9830-2.4.6-5` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) |
| `RFC9830-2.4.6-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L158). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208) |
| `RFC9830-2.4.7-5` | The SR Policy Candidate Path Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.7) | MUST NOT | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) |
| `RFC9830-2.4.7-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) |
| `RFC9830-2.4.7-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L160). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209) |
| `RFC9830-2.4.8-5` | The SR Policy Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.8) | MUST NOT | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) |
| `RFC9830-2.4.8-6` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) |
| `RFC9830-2.4.8-7` | RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L162). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210) |
| `RFC9830-3-3` | Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| `RFC9830-4.1-2` | In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| `RFC9830-4.2.1-1` | When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| `RFC9830-4.2.1-2` | The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143). **negative:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) |
| `RFC9830-4.2.1-3` | the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| `RFC9830-4.2.1-4` | * The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| `RFC9830-4.2.1-5` | If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| `RFC9830-4.2.1-6` | The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215). **negative:** no negative test. **{single-polarity}:** parseConfigRoute attaches the Tunnel Encapsulation attribute to every SR Policy route it builds -- buildTunnelEncap always returns at least the 4-octet TLV header, so the len(tunnelEncapValue) > 0 guard never fails (internal/component/bgp/plugins/nlri/srpolicy/config.go:209-225, :365-370) -- leaving no update-without-the-attribute for a negative to observe. The receive-side obligation to call such an update malformed is the separate RFC9830-4.2.1-8, which is annotated as a gap |
| `RFC9830-4.2.1-7` | The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15). (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226). **negative:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) |
| `RFC9830-4.2.1-8` | A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-4.2.1-9` | A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1) | MUST NOT | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| `RFC9830-4.2.2-1` | If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| `RFC9830-4.2.2-2` | The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| `RFC9830-4.2.2-5` | When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| `RFC9830-4.2.3-1` | SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| `RFC9830-4.2.3-2` | By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| `RFC9830-4.2.3-6` | A BGP node MUST NOT alter the SR Policy information carried in the Tunnel Encapsulation Attribute during propagation (§4.2.3) | MUST NOT | 4.2.3 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L179). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L193) |
| `RFC9830-5-1` | A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| `RFC9830-5-2` | When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| `RFC9830-5-4` | Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-5` | The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| `RFC9830-5-6` | The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-5-7` | In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| `RFC9830-5-8` | An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-9` | A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L325). **negative:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L337) |
| `RFC9830-2.1-5` | It has to be noted that if several CPs of the same SR Policy (Endpoint, Color) are signaled via BGP to a headend, then it is RECOMMENDED that each NLRI use a different distinguisher. (§2.1) | RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-3` | It is RECOMMENDED that the SRv6 Binding SID sub-TLV, as defined in Section 2.4.3, be used when signaling an SRv6 BSID for an SR Policy CP. (§2.4.2) | RECOMMENDED | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-2` | It is RECOMMENDED that the size of the symbolic name for the CP be limited to 255 bytes (§2.4.7) | RECOMMENDED | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-2` | It is RECOMMENDED that the size of the symbolic name for the SR Policy be limited to 255 bytes (§2.4.8) | RECOMMENDED | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-2` | Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.1-1` | Moreover, one or more route targets SHOULD be attached to the advertisement, where each route target identifies one or more intended headends for the advertised SR Policy update. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-3` | When the SR Policy tunnel type includes any sub-TLV that is unrecognized or unsupported, the update SHOULD NOT be considered usable. (§4.2.2) | SHOULD NOT | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-4` | By default, a BGP node receiving an SR Policy NLRI SHOULD NOT remove the Route Target extended community before propagation (§4.2.3) | SHOULD NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-3` | In other cases, where the error in the NLRI encoding results in the inability to process the BGP UPDATE message (e.g., length-related encoding errors), then the router SHOULD handle such malformed NLRIs as "AFI/SAFI disable" when other AFI/SAFIs besides SR Policy are being advertised over the same session. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-10` | An implementation SHOULD log any errors found during the above validation (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.1-4` | In addition, the BGP UPDATE message MAY also contain any of the BGP optional attributes. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-2` | If these sub-TLVs are present, a BGP speaker MUST ignore them and MAY remove them from the Tunnel Encapsulation Attribute during propagation. (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-4` | Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker and MAY be removed from the Tunnel Encapsulation Attribute during propagation. (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.1-1` | The Preference sub-TLV is OPTIONAL (§2.4.1) | OPTIONAL | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-1` | The Binding SID sub-TLV is OPTIONAL (§2.4.2) | OPTIONAL | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-1` | The SRv6 Binding SID sub-TLV is OPTIONAL (§2.4.3) | OPTIONAL | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-2` | More than one SRv6 Binding SID sub-TLV MAY be signaled in the same SR Policy encoding (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-8` | SRv6 Binding SID: Contains a 16-octet SRv6 SID. The value 0 MAY be used when the controller wants to indicate the desired SRv6 Endpoint Behavior, SID Structure, or flags without specifying the BSID. (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-1` | The Segment List sub-TLV is OPTIONAL (§2.4.4) | OPTIONAL | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-2` | The Segment List sub-TLV is OPTIONAL and MAY appear multiple times in the SR Policy encoding. (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-3` | The Segment List sub-TLV contains zero or more Segment sub-TLVs and MAY contain a Weight sub-TLV. (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.1-1` | The Weight sub-TLV is OPTIONAL (§2.4.4.1) | OPTIONAL | 2.4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-1` | The Segment sub-TLVs are OPTIONAL (§2.4.4.2) | OPTIONAL | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-2` | The Segment sub-TLVs are OPTIONAL and MAY appear multiple times in the Segment List sub-TLV. (§2.4.4.2) | MAY | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.1-6` | The receiver MAY override the originator's values for these fields. This would be determined by local policy at the receiver. (§2.4.4.2.1) | MAY | 2.4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.4-1` | The Segment Type sub-TLVs described above MAY contain the SRv6 Endpoint Behavior and SID Structure [RFC8986] encoding as described below (§2.4.4.2.4) | MAY | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-1` | The ENLP sub-TLV is OPTIONAL (§2.4.5) | OPTIONAL | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-9` | The behavior signaled in this sub-TLV MAY be overridden by local configuration by the network operator based on their deployment requirements. (§2.4.5) | MAY | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-1` | An operator MAY set the SR Policy Priority sub-TLV to indicate the order in which the SR policies are recomputed upon topological change. (§2.4.6) | MAY | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-2` | The Priority sub-TLV is OPTIONAL (§2.4.6) | OPTIONAL | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-1` | An operator MAY set the SR Policy Candidate Path Name sub-TLV to attach a symbolic name to the SR Policy CP (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-3` | Implementations MAY choose to truncate long names to 255 bytes when signaling via BGP. (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-4` | The SR Policy Candidate Path Name sub-TLV is OPTIONAL (§2.4.7) | OPTIONAL | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-1` | An operator MAY set the SR Policy Name sub-TLV to associate a symbolic name with the SR Policy (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-3` | Implementations MAY choose to truncate long names to 255 bytes when signaling via BGP. (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-4` | The SR Policy Name sub-TLV is OPTIONAL (§2.4.8) | OPTIONAL | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-1` | The Color Extended Community MAY be carried in any BGP UPDATE message whose AFI/SAFI is 1/1 (IPv4 Unicast), 2/1 (IPv6 Unicast), 1/4 (IPv4 Labeled Unicast), 2/4 (IPv6 Labeled Unicast), 1/128 (VPN-IPv4 Labeled Unicast), 2/128 (VPN-IPv6 Labeled Unicast), or 25/70 (Ethernet VPN, usually known as EVPN). (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-4` | One or more Color Extended Communities MAY be associated with a BGP route update (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-4` | An implementation MAY provide an option for ignoring unsupported sub-TLVs (§4.2.2) | MAY | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-3` | An implementation MAY provide an explicit configuration to override this and enable the propagation of valid SR Policy NLRIs to specific EBGP neighbors where the SR domain comprises multiple ASes within a single service provider domain (see Section 7 for details). (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-5` | An implementation MAY provide support for configuration to filter and/or remove the Route Target extended community before propagation (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9830-2.2-1`](#rfc9830-2.2-1) This document specifies the use of the Tunnel Encapsulation Attribute with the SR Policy Tunnel Type and the use of any other Tunnel Type with the SR Policy SAFI MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | {gap}, no test | nothing rejects a non-SR-Policy tunnel type under SAFI 73. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no attribute-level check ever runs on a received Tunnel Encapsulation attribute, and the attribute itself is kept as raw TLV bytes parsed only on demand (internal/core/bgp/attribute/wire.go:346 and :418, internal/core/bgp/attribute/tunnel_encap.go:39). ze parses and re-advertises the attribute; it applies no treat-as-withdraw |
| [`RFC9830-2.2-3`](#rfc9830-2.2-3) A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy"; such updates MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2) | {gap}, no test | a Tunnel Encapsulation attribute holding two SR Policy TLVs is accepted. ParseTunnelEncap walks every TLV and appends each one without counting types (internal/core/bgp/attribute/tunnel_encap.go:41-54), and there is no RFC 7606 validator for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no treat-as-withdraw is applied. ze's own encoder emits exactly one such TLV, which is the separate RFC9830-2.2-2 |
| [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5) The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.2) | {gap}, no test | buildBindingSIDSubTLV writes 0x10 into the Binding SID Flags octet (internal/component/bgp/plugins/nlri/srpolicy/config.go:385). Section 2.4.2 assigns only S (bit 0, 0x80) and I (bit 1, 0x40) in that field, so bit 3 is unassigned and is set on transmission. The value is pinned as ExaBGP-interoperable in TestSRPolicyInteropExaBGPSubTLVBytes (internal/component/bgp/plugins/nlri/srpolicy/encode_test.go:118-124), which emits the same octet, so the two implementations agree with each other and not with the RFC |
| [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11) The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2) | {gap}, no test | the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4) The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | {gap}, no test | the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2) The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3) Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4) The Flags field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5) The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6) RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7) RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8) Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| [`RFC9830-3-3`](#rfc9830-3-3) Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| [`RFC9830-4.1-2`](#rfc9830-4.1-2) In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1) | {gap}, no test | ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1) When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1) | {gap}, no test | no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3) the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1) | {gap}, no test | SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4) * The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1) | {gap}, no test | nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5) If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1) | {gap}, no test | an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8) A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1) | {gap}, no test | no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9) A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1) If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2) The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5) When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1) SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3) | {gap}, no test | the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2) By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3) | {gap}, no test | propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| [`RFC9830-5-1`](#rfc9830-5-1) A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. (§5) | {gap}, no test | the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| [`RFC9830-5-2`](#rfc9830-5-2) When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5) | {gap}, no test | no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| [`RFC9830-5-4`](#rfc9830-5-4) Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5) | {gap}, no test | no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| [`RFC9830-5-5`](#rfc9830-5-5) The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5) | {gap}, no test | only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| [`RFC9830-5-6`](#rfc9830-5-6) The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5) | {gap}, no test | the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-5-7`](#rfc9830-5-7) In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5) | {gap}, no test | an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| [`RFC9830-5-8`](#rfc9830-5-8) An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5) | {gap}, no test | the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9830-2.1-1`](#rfc9830-2.1-1)

The AFI used MUST be IPv4(1) or IPv6(2) (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L92) | unit/verify | unproven |

### [`RFC9830-2.1-2`](#rfc9830-2.1-2)

When AFI = 1, the value MUST be 96; when AFI = 2, the value MUST be 192. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NLRIAddressFamilies. Forbidden: a length octet other than 96 for AFI 1 or other than 192 for AFI 2. Red: assert.Equal(byte(96), v4[0]) and assert.Equal(byte(192), v6[0]) on EncodeNLRIHex output, with assert.Len on the 12/24-octet body. Negative: Parse(AFIIPv6, 12-octet body) require.ErrorIs ErrSRPolicyTruncated, and SplitSRPolicy refuses a 100-bit length.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114) | unit/verify | unproven |

### [`RFC9830-2.1-3`](#rfc9830-2.1-3)

A BGP UPDATE message that carries the MP_REACH_NLRI or MP_UNREACH_NLRI attribute with the SR Policy SAFI MUST also carry the BGP mandatory attributes. (§2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830UpdateCarriesMandatoryAttributes asserts ORIGIN ("40010100") and AS_PATH on an MP_REACH UPDATE only. The MP_UNREACH_NLRI half of the sentence has no assertion: no tagged unit builds an SR Policy withdrawal and checks its attributes. The iBGP AS_PATH check is a bare hex substring "4002", which can match elsewhere in the UPDATE. The negative (LOCAL_PREF only on iBGP) proves a neighbouring rule, not a missing mandatory attribute.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) | unit/verify | unproven |
| positive | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178) | unit/verify | unproven |

### [`RFC9830-2.2-1`](#rfc9830-2.2-1)

This document specifies the use of the Tunnel Encapsulation Attribute with the SR Policy Tunnel Type and the use of any other Tunnel Type with the SR Policy SAFI MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.2-1, so no unit is bound to it.

### [`RFC9830-2.2-2`](#rfc9830-2.2-2)

A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194) | unit/verify | unproven |

### [`RFC9830-2.2-3`](#rfc9830-2.2-3)

A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy"; such updates MUST be considered malformed and handled by the "treat-as-withdraw" strategy [RFC7606]. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.2-3, so no unit is bound to it.

### [`RFC9830-2.3-1`](#rfc9830-2.3-1)

If these sub-TLVs are present, a BGP speaker MUST ignore them (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830EgressEndpointAndColorSubTLVsIgnored measures 'ignored' through TunnelTLV.Preference, which filters on type 12 and can never read a type-6 or type-4 sub-TLV; ze has no receive-side consumer of either. A speaker that acted on the Color or Tunnel Egress Endpoint sub-TLV (the forbidden behaviour) leaves assert.Equal(want, got) green; only rejection of the attribute (teRoundTrip require.NoError) would go red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293) | unit/verify | unproven |
| positive | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L282) | unit/verify | unproven |

### [`RFC9830-2.3-3`](#rfc9830-2.3-3)

Similarly, any other sub-TLVs, including those specified in [RFC9012], that do not have explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012MeaninglessSubTLVIgnoredNotRemoved measures 'ignored' of a VXLAN sub-TLV inside Tunnel Type 15 through TunnelTLV.Preference, which reads only type 12 and cannot observe whether any other sub-TLV is used. A speaker that acted on the VXLAN sub-TLV leaves assert.Equal(without, withMeaningless) green; only parse failure goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L312) | unit/verify | unproven |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L303) | unit/verify | unproven |

### [`RFC9830-2.4-1`](#rfc9830-2.4-1)

For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012DuplicateSingleInstanceSubTLVs. Forbidden: using a later instance of a single-instance sub-TLV. Red: assert.Equal(uint32(100), pref) with Preference 100 then 200 fails if the last or any non-first instance is used. Negative: the 200 instance alone is returned, so the choice is positional. Preference is the only single-instance sub-TLV ze reads on receipt (tunnel_encap.go:145), so no other instance can be used.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L278) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4-2`](#rfc9830-2.4-2)

For the TLVs/sub-TLVs that are specified as single instance, only the first instance of that TLV/sub-TLV is used: the other instances MUST be ignored and MUST NOT considered to be malformed. (§2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012DuplicateSingleInstanceSubTLVs. Forbidden: treating a duplicate Preference as malformed, or using it. Red: require.NoError on ParseTunnelEncap and SubTLVs, require.Len(stlvs, 2), assert.Equal(raw, teRoundTrip) on the duplicate; 'ignored' by assert.Equal(uint32(100), pref) in the same unit. Negative (internal/core/bgp/attribute/rfc9012_test.go::TestRFC9012MalformedAttributeIsRejected): a truncated TLV header is refused, so acceptance is a carve-out.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269) | unit/verify | unproven |

### [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2)

The Preference sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: two Preference sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPreference), 1) for a config naming preference twice. Negative: a config with no preference emits none (assert.Empty), so the count is real.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a Preference length other than 6. Red: require.Len(value, 6) on a value srpSubs cut at the declared length (a wrong declared length also fails the walk's require.NoError), with the preference in value[2:6]. Negative (internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830PreferenceFlagsAndReservedIgnored): 4- and 8-octet values are not read.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L262) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250) | unit/verify | unproven |

### [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4)

The Flags field MUST be set to zero on transmission (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a non-zero Preference Flags octet on transmission. Red: assert.Equal(byte(0), value[0]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), value), preference 0xDEADBEEF beside it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254) | unit/verify | unproven |

### [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830PreferenceFlagsAndReservedIgnored proves the receipt half: Preference() reads this sub-TLV, and a receiver that read or refused Flags 0xFF turns assert.Equal(want, got) red; the control (pref 9999) shows the value octets are read. WEAK because the transmission clause (Flags set to zero) has no assertion in any tagged unit; it is proven under RFC9830-2.4.1-4, not tagged here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L254) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L244) | unit/verify | unproven |

### [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PreferenceSubTLV. Forbidden: a non-zero Preference RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), value).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256) | unit/verify | unproven |

### [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830PreferenceFlagsAndReservedIgnored proves the receipt half: a receiver that read or refused RESERVED 0xFF turns assert.Equal(want, got) red. WEAK because the transmission clause (RESERVED set to zero) has no assertion in any tagged unit; it is proven under RFC9830-2.4.1-6, not tagged here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L249) | unit/verify | unproven |

### [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2)

The Binding SID sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: two Binding SID sub-TLVs. Red: assert.Len(srpAll(repeated, subTLVBindingSID), 1) for 'binding-sid mpls 24000 binding-sid null'. Negative: none configured, none emitted (assert.Empty).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300) | unit/verify | unproven |

### [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4)

The value MUST be 18 when a SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present. (§2.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV asserts require.Len(withBSID, 6) for SR-MPLS and require.Len(noBSID, 2) for null, both red on a wrong length. The 18-octet clause (SRv6 BSID in sub-TLV 13) has no assertion: ze has no producer for it (buildBindingSIDSubTLV, buildBindingSIDNullSubTLV in config.go), but the row carries no single-polarity or scope annotation saying so, so one of three listed cases is unproven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280) | unit/verify | unproven |

### [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.2-5, so no unit is bound to it.

### [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Binding SID Flags unassigned bits to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under no row: RFC9830-2.4.2-5 is a {gap}, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Binding SID Flags unassigned bits; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation. CODE DEFECT: the transmission clause is violated, buildBindingSIDSubTLV writes 0x10 (unassigned bit 3) into the Flags octet (srpolicy/config.go:385).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L112) | unit/verify | unproven |

### [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7)

RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: a non-zero Binding SID RESERVED octet. Red: assert.Equal(byte(0), withBSID[1]) and assert.Equal(byte(0), noBSID[1]). Negative: assert.NotEqual(make([]byte, 6), withBSID) beside label 0xFFFFF.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285) | unit/verify | unproven |

### [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8)

RESERVED: 1 octet of reserved bits. MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Binding SID RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.2-7, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Binding SID RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L195) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L114) | unit/verify | unproven |

### [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9)

Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero (§2.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830BindingSIDSubTLV. Forbidden: non-zero TC, S or TTL in the transmitted label stack entry. Red: assert.Equal(byte(0), withBSID[4]&0x0E) (TC), withBSID[4]&0x01 (S), withBSID[5] (TTL). Negative: the 20-bit label reads 0xFFFFF, so the zeroes are field layout, not a small label.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289) | unit/verify | unproven |

### [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10)

Traffic Class (TC), S, and TTL (Total of 12 bits) are RESERVED and MUST be set to zero and MUST be ignored. (§2.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Binding SID TC, S and TTL bits to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.2-9, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Binding SID TC, S and TTL bits; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L196) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L116) | unit/verify | unproven |

### [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11)

The Label field is validated by the SRPM but MUST NOT contain the reserved MPLS label values (0-15). (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.2-11, so no unit is bound to it.

### [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3)

The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: a length other than 18 without the Endpoint Behavior and SID Structure. Red: require.Len(value, 18). The 26 case has no producer (buildSRv6BindingSIDSubTLV never appends the structure), carried by the row's single-polarity annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315) | unit/verify | unproven |

### [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: non-zero unassigned Flags bits on transmission (ze sets no assigned flag). Red: assert.Equal(byte(0), value[0]). Negative: assert.NotEqual(make([]byte, 18), value), SID octets non-zero.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322) | unit/verify | unproven |

### [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the SRv6 Binding SID Flags unassigned bits to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.3-4, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the SRv6 Binding SID Flags unassigned bits; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L197) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118) | unit/verify | unproven |

### [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SRv6BindingSIDSubTLV. Forbidden: a non-zero RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual(make([]byte, 18), value).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324) | unit/verify | unproven |

### [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the SRv6 Binding SID RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.3-6, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the SRv6 Binding SID RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L120) | unit/verify | unproven |

### [`RFC9830-2.4.3-9`](#rfc9830-2.4.3-9)

The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L318) | unit/verify | unproven |

### [`RFC9830-2.4.4-4`](#rfc9830-2.4.4-4)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Segment List RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]). Negative: the following sub-TLVs are non-empty (require.NotEmpty(inner)) and the value is not blank.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364) | unit/verify | unproven |

### [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Segment List RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4-4, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Segment List RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L199) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2)

The Weight sub-TLV is OPTIONAL; it MUST NOT appear more than once inside the Segment List sub-TLV. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a second Weight sub-TLV in one Segment List. Red: require.Len(weights, 1) on the srpDirty encoding, and require.Error on parseConfigRoute for 'segment-list weight 1 weight 2 ...', so the config path cannot produce a second one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a Weight length other than 6 (e.g. the 8-octet total). Red: require.Len(weight, 6) on a value cut at the declared length, srpSegSubs require.NoError on the walk, and assert.Len(value, 1+8+8+28) on the whole segment list.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4)

The Flags field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Weight Flags octet. Red: assert.Equal(byte(0), weight[0]). Negative: assert.NotEqual(make([]byte, 6), weight), weight 0xCAFEBABE.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Weight Flags octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.1-4, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Weight Flags octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L200) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L126) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentListSubTLV. Forbidden: a non-zero Weight RESERVED octet on transmission. Red: assert.Equal(byte(0), weight[1]) on the srpDirty encoding. Negative: assert.NotEqual(make([]byte, 6), weight) with weight[2:6] pinned to 0xCAFEBABE, so the zero is written, not a blank buffer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Weight RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.1-6, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Weight RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L201) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1)

Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 6. (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: a Type A length other than 6 (e.g. the 8-octet total). Red: require.Len(value, 6) on a value srpSegSubs cut at the declared length. Negative: assert.Len(inner, 3), which a length of 8 breaks by swallowing the Type B sub-TLV header that follows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: a non-zero Type A RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual(make([]byte, 6), value) with the label pinned to 0xFFFFF.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Type A segment RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.2.1-2, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Type A segment RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4)

The S bit MUST be zero upon transmission (§2.4.4.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV. Forbidden: S bit set in the transmitted Type A label stack entry. Red: assert.Equal(byte(0), value[4]&0x01). Negative: assert.Equal(uint32(0xFFFFF), label), so every label bit is set beside the clear S bit and the zero is not a blank low octet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5)

The S bit MUST be zero upon transmission and MUST be ignored upon reception. (§2.4.4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Type A label stack entry S bit to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.2.1-4, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Type A label stack entry S bit; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1)

The value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present; else, it MUST be 18. (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: a length other than 26 with the Endpoint Behavior and SID Structure, or other than 18 without it. Red: require.Len(value, 26) on the srpDirty segment and require.Len(bare[0], 18) on the bare type-b segment; both clauses have an assertion and the pair shows the length tracks the content.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: a non-zero Type B RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]) and assert.Equal(byte(0), bare[0][1]), with and without the structure. Negative: assert.NotEqual(make([]byte, 26), value) and value[2] pinned to 0xFC.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Type B segment RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.2.2-2, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Type B segment RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L204) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4)

The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included. (§2.4.4.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: the Endpoint Behavior and SID Structure encoded without an SRv6 SID. Red: require.Error on parseSegmentList and on parseConfigRoute for 'segment endpoint-behavior ...' with no preceding type-b srv6 SID, the only route to a structure ze has. Positive: the structure follows a 16-octet SID (assert.Equal(byte(0xEF), value[17])).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1)

The unassigned bits in the Flags field MUST be set to zero upon transmission (§2.4.4.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeASubTLV and TestRFC9830SegmentTypeBSubTLV. Forbidden: any unassigned Segment Flags bit set on transmission. Red: assert.Equal(byte(0), value[0]) on Type A and assert.Equal(byte(0), value[0]&^byte(0x10)) on Type B. Negative: the Type B Flags octet is 0x10 (the assigned B-Flag, bit 3 per Figure 13) and not zero, and 0 without the structure, so the assertion is on which bits are set, not on a blank byte.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2)

The unassigned bits in the Flags field MUST be set to zero upon transmission and MUST be ignored upon receipt. (§2.4.4.2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the unassigned Segment Flags bits (0x6F on a Type A) to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.2.3-1, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the unassigned Segment Flags bits (0x6F on a Type A); ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3)

If B-Flag appears with Segment Type A, it MUST be ignored. (§2.4.4.2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the B-Flag on a Type A segment to ones. WEAK : the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the B-Flag on a Type A segment; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2)

Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission (§2.4.4.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830SegmentTypeBSubTLV. Forbidden: non-zero Reserved octets in the Endpoint Behavior and SID Structure on transmission. Red: assert.Equal([]byte{0x00, 0x00}, value[20:22]). Negative: endpoint behavior 0xFFFF at value[18:20] and structure 32/16/16/64 at value[22:26], so the zeros are written at the right offset.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3)

Reserved: 2 octets of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.4.2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the SRv6 Endpoint Behavior and SID Structure Reserved octets to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.4.2.4-2, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the SRv6 Endpoint Behavior and SID Structure Reserved octets; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L154) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4)

The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.4.2.4-4, so no unit is bound to it.

### [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2)

The ENLP sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-2, so no unit is bound to it.

### [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3)

Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 3. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-3, so no unit is bound to it.

### [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4)

The Flags field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-4, so no unit is bound to it.

### [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5)

The Flags field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-5, so no unit is bound to it.

### [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-6, so no unit is bound to it.

### [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-7, so no unit is bound to it.

### [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8)

Implementations adhering to this document MUST ignore the ENLP sub-TLV with unrecognized values (viz. other than 1 through 4). (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-8, so no unit is bound to it.

### [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3)

The Priority sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV. Forbidden: two Priority sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPriority), 1) for a config naming priority twice. Negative (control): a config with no priority emits none (assert.Empty), so the instance is counted rather than always written; a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348) | unit/verify | unproven |

### [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4)

Length: Specifies the length of the value field (i.e., not including Type and Length fields) in terms of octets. The value MUST be 2. (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV and TestRFC9830PriorityLengthIsValueLength. Forbidden: a Priority length other than 2 (e.g. the 4-octet total). Red: require.Len(value, 2) on a value cut at the declared length, and assert.Equal([]byte{15, 2, 255, 0}, only[4:]) pinning the length octet itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339) | unit/verify | unproven |

### [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830PrioritySubTLV. Forbidden: a non-zero Priority RESERVED octet on transmission. Red: assert.Equal(byte(0), value[1]). Negative: assert.NotEqual([]byte{0,0}, value) with value[0] pinned to 255, so the zero is written and the fields are not transposed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343) | unit/verify | unproven |

### [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Priority RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.6-5, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Priority RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L158) | unit/verify | unproven |

### [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5)

The SR Policy Candidate Path Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: two Candidate Path Name sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVCandidatePathNam), 1) for a config naming the candidate path twice. Negative (control): an unnamed candidate path emits none (assert.Empty); a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506) | unit/verify | unproven |

### [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: a non-zero Candidate Path Name RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]) in the loop case for 'primary'. Negative: require.Len(value, 1+len(want)) and assert.Equal(want, string(value[1:])), so the zero is a separate written octet ahead of the whole name.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495) | unit/verify | unproven |

### [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the Candidate Path Name RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.7-6, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the Candidate Path Name RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L160) | unit/verify | unproven |

### [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5)

The SR Policy Name sub-TLV is OPTIONAL; it MUST NOT appear more than once in the SR Policy encoding. (§2.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: two SR Policy Name sub-TLVs in one encoding. Red: assert.Len(srpAll(repeated, subTLVPolicyName), 1) for a config naming the policy twice. Negative (control): an unnamed policy emits none (assert.Empty); a violating output cannot be built from a conforming encoder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507) | unit/verify | unproven |

### [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission (§2.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NameSubTLVs. Forbidden: a non-zero SR Policy Name RESERVED octet on transmission. Red: assert.Equal(byte(0), value[0]) in the loop case for 'alpha'. Negative: assert.NotEqual(make([]byte, 6), ...) and assert.Equal(want, string(value[1:])), so the zero is written ahead of the whole name.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496) | unit/verify | unproven |

### [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7)

RESERVED: 1 octet of reserved bits. This field MUST be set to zero on transmission and MUST be ignored on receipt. (§2.4.8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830ReceivedFieldsAreIgnoredNotRead sets the SR Policy Name RESERVED octet to ones. WEAK on two counts: (1) the transmission clause (set to zero) has no assertion in any tagged unit (it is proven under RFC9830-2.4.8-6, not tagged here); (2) the receipt half is measured through TunnelTLV.Preference (tunnel_encap.go:145), which reads only a type-12 sub-TLV of length 6 and so never reads the SR Policy Name RESERVED octet; ze has no receive-side consumer of that field, so a receiver that read it or acted on it leaves every Preference assertion green. Only the round trip (assert.Equal(raw, teRoundTrip)) goes red, and only on rejection or normalisation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L162) | unit/verify | unproven |

### [`RFC9830-3-3`](#rfc9830-3-3)

Type 3 (bits 11): Reserved for future use and SHOULD NOT be used. Upon reception, an implementation MUST treat it like Type 0. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-3-3, so no unit is bound to it.

### [`RFC9830-4.1-2`](#rfc9830-4.1-2)

In such a case, the NO_ADVERTISE community [RFC1997] MUST be attached to the SR Policy update (see further details in Section 4.2.3). (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.1-2, so no unit is bound to it.

### [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1)

When a BGP speaker receives an SR Policy NLRI from a neighbor, it MUST first perform validation based on the following rules in addition to the validation described in Section 5 (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-1, so no unit is bound to it.

### [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2)

The SR Policy NLRI MUST include a distinguisher, Color, and Endpoint field (§4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830NLRICarriesAllThreeFields. WEAK: the sentence is a receive-validation rule (Section 4.2.1 'When a BGP speaker receives'). The tagged unit proves the encoder writes Distinguisher, Color and Endpoint at their offsets and that Parse (require.ErrorIs ErrSRPolicyTruncated) and parseConfigRoute (errSRPolicyMissingFields) refuse a body or config missing one, but the session receive path splits with SplitSRPolicy (split.go), which accepts any byte-aligned length, so a received body lacking the Endpoint is carried; no tagged unit drives a received UPDATE, and no assertion goes red on a receiver that accepts it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143) | unit/verify | unproven |

### [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3)

the length of the NLRI MUST be either 12 or 24 octets (depending on the address family of the Endpoint). (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-3, so no unit is bound to it.

### [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4)

* The SR Policy update MUST have either the NO_ADVERTISE community, at least one Route Target extended community in IPv4-address format, or both. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-4, so no unit is bound to it.

### [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5)

If a router supporting this specification receives an SR Policy update with no Route Target extended communities and no NO_ADVERTISE community, the update MUST be considered to be malformed. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-5, so no unit is bound to it.

### [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6)

The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215) | unit/verify | unproven |

### [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7)

The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message and MUST have a Tunnel Type TLV set to SR Policy (code point is 15). (§4.2.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go::TestRFC9830TunnelTypeIsSRPolicy. WEAK: the sentence is a receive-validation rule. The tagged unit proves only that ze's own encoder wraps the sub-TLVs in a Tunnel Type 15 TLV (assert.Equal([]byte{0x00,0x0F}, value[0:2])); nothing checks a received SR Policy update for a type-15 TLV or for the attribute at all (attribute 23 has no validator, per the RFC9830-4.2.1-8 gap). The negative is conditional: when ParseTunnelEncap errors on the bare payload it asserts nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) | unit/verify | unproven |
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226) | unit/verify | unproven |

### [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8)

A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-8, so no unit is bound to it.

### [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9)

A router that receives an SR Policy update that is not valid according to these criteria MUST treat the update as malformed, and the SR Policy CP MUST NOT be passed to the SRPM. (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-9, so no unit is bound to it.

### [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1)

If one or more route targets are present, then at least one route target MUST match the BGP Identifier of the receiver for the update to be considered usable. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-1, so no unit is bound to it.

### [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2)

The BGP Identifier is defined in [RFC4271] as a 4-octet IPv4 address and is updated by [RFC6286] as a 4-octet, unsigned, non-zero integer. Therefore, the Route Target extended community MUST be of the same format. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-2, so no unit is bound to it.

### [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5)

When an update for an SR Policy NLRI results in its becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-5, so no unit is bound to it.

### [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1)

SR Policy NLRIs that have the NO_ADVERTISE community attached to them MUST NOT be propagated. (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.3-1, so no unit is bound to it.

### [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2)

By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any External BGP (EBGP) neighbor. (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.3-2, so no unit is bound to it.

### [`RFC9830-4.2.3-6`](#rfc9830-4.2.3-6)

A BGP node MUST NOT alter the SR Policy information carried in the Tunnel Encapsulation Attribute during propagation (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L193) | unit/verify | unproven |
| positive | [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L179) | unit/verify | unproven |

### [`RFC9830-5-1`](#rfc9830-5-1)

A BGP speaker MUST perform the following syntactic validation of the SR Policy NLRI to determine if it is malformed. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-1, so no unit is bound to it.

### [`RFC9830-5-2`](#rfc9830-5-2)

When the error determined allows for the router to skip the malformed NLRI(s) and continue the processing of the rest of the BGP UPDATE message, then it MUST handle such malformed NLRIs as 'treat-as- withdraw'. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-2, so no unit is bound to it.

### [`RFC9830-5-4`](#rfc9830-5-4)

Alternately, the router MUST perform "session reset" when the session is only being used for SR Policy or when a "AFI/SAFI disable" action is not possible. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-4, so no unit is bound to it.

### [`RFC9830-5-5`](#rfc9830-5-5)

The validation of the TLVs/sub-TLVs introduced in this document and defined in their respective subsections of Section 2.4 MUST be performed to determine if they are malformed or invalid. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-5, so no unit is bound to it.

### [`RFC9830-5-6`](#rfc9830-5-6)

The validation of the Tunnel Encapsulation Attribute itself and the other TLVs/sub-TLVs specified in Section 13 of [RFC9012] MUST be done as described in that document. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-6, so no unit is bound to it.

### [`RFC9830-5-7`](#rfc9830-5-7)

In case of any error detected, either at the attribute or its TLV/sub-TLV level, the "treat-as-withdraw" strategy MUST be applied. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-7, so no unit is bound to it.

### [`RFC9830-5-8`](#rfc9830-5-8)

An SR Policy update that is determined not to be valid (and, therefore, malformed) based on the rules described in Section 4.2.1 MUST be handled by the "treat-as-withdraw" strategy. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-8, so no unit is bound to it.

### [`RFC9830-5-9`](#rfc9830-5-9)

A BGP implementation MUST NOT perform semantic verification of such fields nor consider the SR Policy update to be invalid or not usable based on such validation. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. internal/core/bgp/attribute/rfc9830_test.go::TestRFC9830NoSemanticVerification. ParseTunnelEncap is the only receive parser of attribute 23 (wire.go knownAttrParsers) and Preference the only field ze consumes. Forbidden: refusing, or not using, an SR Policy update because a field value is semantically wrong. Red: require.NoError(ParseTunnelEncap), require.NoError(SubTLVs) with assert.Len(stlvs, 5), assert.Equal(uint32(4242), pref) and the byte-identical round trip, over weight 0, reserved label 3, a SID structure totalling 255, ENLP 99 and an empty segment list. Negative: framing errors are still refused (require.Error twice), so the skip is limited to field semantics. A verification with no observable effect cannot be asserted; every effect it could have is.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L337) | unit/verify | unproven |
| positive | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L325) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9830.txt |
| Source fingerprint | 35ee1db5cdbf6fc4 |
| Record | rfc/extraction/rfc9830.json |
| Mapped sentences | 71 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 3 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `2.3` | not stated | 2 | walked | not stated |
| `2.4` | not stated | 1 | walked | not stated |
| `2.4.1` | not stated | 4 | walked | not stated |
| `2.4.2` | not stated | 6 | walked | not stated |
| `2.4.3` | not stated | 4 | walked | not stated |
| `2.4.4` | not stated | 2 | walked | not stated |
| `2.4.4.1` | not stated | 4 | walked | not stated |
| `2.4.4.2` | not stated | 0 | walked | not stated |
| `2.4.4.2.1` | not stated | 3 | walked | not stated |
| `2.4.4.2.2` | not stated | 3 | walked | not stated |
| `2.4.4.2.3` | not stated | 2 | walked | not stated |
| `2.4.4.2.4` | not stated | 2 | walked | not stated |
| `2.4.5` | not stated | 6 | walked | not stated |
| `2.4.6` | not stated | 3 | walked | not stated |
| `2.4.7` | not stated | 3 | walked | not stated |
| `2.4.8` | not stated | 3 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 6 | walked | not stated |
| `4.2.2` | not stated | 3 | walked | not stated |
| `4.2.3` | not stated | 3 | walked | not stated |
| `5` | not stated | 8 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `6.7` | not stated | 0 | walked | not stated |
| `6.8` | not stated | 0 | walked | not stated |
| `6.9` | not stated | 0 | walked | not stated |
| `6.10` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Revised BSD License for extracted Code Components; it states no protocol obligation. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `2.4.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. The Segment List sub-TLV length field width is the diagram's, not an obligation on a speaker. | A 2-octet length is thus required. |
| `2.4.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introductory description of what the ENLP sub-TLV indicates; "must" is lowercase and is not an RFC 2119 keyword. The normative ENLP obligations are the sites 2.4.5:2 through 2.4.5:6. | The Explicit NULL Label Policy (ENLP) sub-TLV is used to indicate whether an Explicit NULL Label [RFC3032] must be pushed on an unlabeled IP packet before any other labels. |
| `2.4.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. | A 2-octet length is thus required. |
| `2.4.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose explaining the format diagram: "required" is lowercase and is not an RFC 2119 keyword. | A 2-octet length is thus required. |

## Superseded

No document obsoletes RFC 9830, so its obligations are stated where they were written.
