# RFC 9830 - BGP Extensions for the Advertisement of Segment Routing (SR) Policies

Partial. Every requirement this repository extracted from RFC 9830, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 71.4% | 60 of 84 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.8% | 4 of 84 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 84 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 125 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 96 | of 137 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 12 | of 96 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 23.8% | 20 of 84 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 84 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 137 |
| Gated MUST-level | 96 |
| Obligations that bind Ze | 84 |
| Not applicable, so out of scope | 12 |
| Declared gaps | 20 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 125 |
| Tagged units | 125 |
| Recorded audit verdicts | 0 |
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
| **Gated MUST-level requirements** | **96** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (60):** [`RFC9830-2.1-1`](#rfc9830-2.1-1), [`RFC9830-2.1-2`](#rfc9830-2.1-2), [`RFC9830-2.1-3`](#rfc9830-2.1-3), [`RFC9830-2.3-1`](#rfc9830-2.3-1), [`RFC9830-2.3-3`](#rfc9830-2.3-3), [`RFC9830-2.4-1`](#rfc9830-2.4-1), [`RFC9830-2.4-2`](#rfc9830-2.4-2), [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2), [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3), [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4), [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5), [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6), [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7), [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2), [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4), [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6), [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7), [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8), [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9), [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10), [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4), [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5), [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6), [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7), [`RFC9830-2.4.4-4`](#rfc9830-2.4.4-4), [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5), [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2), [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3), [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4), [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5), [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6), [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7), [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1), [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2), [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3), [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4), [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5), [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1), [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2), [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3), [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4), [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1), [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2), [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3), [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2), [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3), [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3), [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4), [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5), [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6), [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5), [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6), [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7), [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5), [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6), [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7), [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2), [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7), [`RFC9830-4.2.3-6`](#rfc9830-4.2.3-6), [`RFC9830-5-9`](#rfc9830-5-9)

**Annotated instead of tested (36):** [`RFC9830-2.2-1`](#rfc9830-2.2-1), [`RFC9830-2.2-2`](#rfc9830-2.2-2), [`RFC9830-2.2-3`](#rfc9830-2.2-3), [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5), [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11), [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3), [`RFC9830-2.4.3-9`](#rfc9830-2.4.3-9), [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4), [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2), [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3), [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4), [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5), [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6), [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7), [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8), [`RFC9830-3-3`](#rfc9830-3-3), [`RFC9830-4.1-2`](#rfc9830-4.1-2), [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1), [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3), [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4), [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5), [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6), [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8), [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9), [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1), [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2), [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5), [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1), [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2), [`RFC9830-5-1`](#rfc9830-5-1), [`RFC9830-5-2`](#rfc9830-5-2), [`RFC9830-5-4`](#rfc9830-5-4), [`RFC9830-5-5`](#rfc9830-5-5), [`RFC9830-5-6`](#rfc9830-5-6), [`RFC9830-5-7`](#rfc9830-5-7), [`RFC9830-5-8`](#rfc9830-5-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9830-2.1-1` | The AFI used MUST be IPv4(1) or IPv6(2) (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L92). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L120) |
| `RFC9830-2.1-2` | The NLRI Length value MUST be 96 when AFI = 1 and 192 when AFI = 2 (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114). **negative:** `unit/verify` [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) |
| `RFC9830-2.1-3` | A BGP UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with the SR Policy SAFI MUST also carry the BGP mandatory attributes (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178). **negative:** `unit/verify` [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) |
| `RFC9830-2.2-1` | Use of any Tunnel Type other than SR Policy with the SR Policy SAFI MUST be considered malformed and handled by treat-as-withdraw (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing rejects a non-SR-Policy tunnel type under SAFI 73. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no attribute-level check ever runs on a received Tunnel Encapsulation attribute, and the attribute itself is kept as raw TLV bytes parsed only on demand (internal/core/bgp/attribute/wire.go:346 and :418, internal/core/bgp/attribute/tunnel_encap.go:39). ze parses and re-advertises the attribute; it applies no treat-as-withdraw |
| `RFC9830-2.2-2` | A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2) | MUST NOT | 2.2 | **positive:** `unit/verify` [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194). **negative:** no negative test. **{single-polarity}:** buildTunnelEncap assembles every configured sub-TLV into a single Tunnel Type 15 TLV and has no path that appends a second one (internal/component/bgp/plugins/nlri/srpolicy/config.go:365-370), so no input to ze's encoder produces the forbidden encoding for a negative to assert. The receive-side obligation to treat two such TLVs as malformed is the separate RFC9830-2.2-3, which is annotated as a gap |
| `RFC9830-2.2-3` | Updates carrying more than one SR Policy TLV MUST be considered malformed and handled by treat-as-withdraw (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a Tunnel Encapsulation attribute holding two SR Policy TLVs is accepted. ParseTunnelEncap walks every TLV and appends each one without counting types (internal/core/bgp/attribute/tunnel_encap.go:41-54), and there is no RFC 7606 validator for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no treat-as-withdraw is applied. ze's own encoder emits exactly one such TLV, which is the separate RFC9830-2.2-2 |
| `RFC9830-2.3-1` | If the Tunnel Egress Endpoint and Color sub-TLVs are present, a BGP speaker MUST ignore them (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L282). **negative:** `unit/verify` [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293) |
| `RFC9830-2.3-3` | Any other sub-TLVs without explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L303). **negative:** `unit/verify` [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L312) |
| `RFC9830-2.4-1` | For single-instance TLVs/sub-TLVs, only the first instance is used and the other instances MUST be ignored (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263). **negative:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L278) |
| `RFC9830-2.4-2` | The other (duplicate) instances of a single-instance TLV/sub-TLV MUST NOT be considered malformed (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269). **negative:** `unit/verify` [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155) |
| `RFC9830-2.4.1-2` | The Preference sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.1) | MUST NOT | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) |
| `RFC9830-2.4.1-3` | The Preference sub-TLV Length value MUST be 6 (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L262) |
| `RFC9830-2.4.1-4` | The Preference Flags field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) |
| `RFC9830-2.4.1-5` | The Preference Flags field MUST be ignored on receipt (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L244). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L254) |
| `RFC9830-2.4.1-6` | The Preference RESERVED field MUST be set to zero on transmission (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256). **negative:** `unit/verify` [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) |
| `RFC9830-2.4.1-7` | The Preference RESERVED field MUST be ignored on receipt (§2.4.1) | MUST | 2.4.1 | **positive:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L249). **negative:** `unit/verify` [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255) |
| `RFC9830-2.4.2-2` | The Binding SID sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.2) | MUST NOT | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) |
| `RFC9830-2.4.2-4` | The Binding SID Length value MUST be 18 when an SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) |
| `RFC9830-2.4.2-5` | The unassigned bits in the Binding SID Flags field MUST be set to zero upon transmission (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildBindingSIDSubTLV writes 0x10 into the Binding SID Flags octet (internal/component/bgp/plugins/nlri/srpolicy/config.go:385). Section 2.4.2 assigns only S (bit 0, 0x80) and I (bit 1, 0x40) in that field, so bit 3 is unassigned and is set on transmission. The value is pinned as ExaBGP-interoperable in TestSRPolicyInteropExaBGPSubTLVBytes (internal/component/bgp/plugins/nlri/srpolicy/encode_test.go:118-124), which emits the same octet, so the two implementations agree with each other and not with the RFC |
| `RFC9830-2.4.2-6` | The unassigned bits in the Binding SID Flags field MUST be ignored upon receipt (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L112). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L194) |
| `RFC9830-2.4.2-7` | The Binding SID RESERVED field MUST be set to zero on transmission (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) |
| `RFC9830-2.4.2-8` | The Binding SID RESERVED field MUST be ignored on receipt (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L114). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L195) |
| `RFC9830-2.4.2-9` | The Binding SID Label TC, S, and TTL bits MUST be set to zero (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289). **negative:** `unit/verify` [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) |
| `RFC9830-2.4.2-10` | The Binding SID Label TC, S, and TTL bits MUST be ignored (§2.4.2) | MUST | 2.4.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L116). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L196) |
| `RFC9830-2.4.2-11` | The Binding SID Label field MUST NOT contain the reserved MPLS label values (0-15) (§2.4.2) | MUST NOT | 2.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| `RFC9830-2.4.3-3` | The SRv6 Binding SID Length value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present, else MUST be 18 (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315). **negative:** no negative test. **{single-polarity}:** buildSRv6BindingSIDSubTLV always writes the 18-octet form and never appends the SRv6 Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the 26-octet case has no producer and there is no contrasting length to assert. The same 18-versus-26 rule for the Type B segment sub-TLV, where ze DOES produce both, is covered with both polarities under RFC9830-2.4.4.2.2-1 |
| `RFC9830-2.4.3-4` | The unassigned bits in the SRv6 Binding SID Flags field MUST be set to zero upon transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) |
| `RFC9830-2.4.3-5` | The unassigned bits in the SRv6 Binding SID Flags field MUST be ignored upon receipt (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L197) |
| `RFC9830-2.4.3-6` | The SRv6 Binding SID RESERVED field MUST be set to zero on transmission (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324). **negative:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) |
| `RFC9830-2.4.3-7` | The SRv6 Binding SID RESERVED field MUST be ignored on receipt (§2.4.3) | MUST | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L120). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L198) |
| `RFC9830-2.4.3-9` | The SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.3) | MUST NOT | 2.4.3 | **positive:** `unit/verify` [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L318). **negative:** no negative test. **{single-polarity}:** the SRv6 Binding SID sub-TLV ze writes always carries the 16-octet SID and never the Endpoint Behavior and SID Structure (internal/component/bgp/plugins/nlri/srpolicy/config.go:409-416), so the forbidden combination has no producer to drive a negative from. The parallel Type B rule, where an endpoint-behavior token without a preceding SRv6 SID IS refused, is covered with both polarities under RFC9830-2.4.4.2.2-4 |
| `RFC9830-2.4.4-4` | The Segment List RESERVED field MUST be set to zero on transmission (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) |
| `RFC9830-2.4.4-5` | The Segment List RESERVED field MUST be ignored on receipt (§2.4.4) | MUST | 2.4.4 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L199) |
| `RFC9830-2.4.4.1-2` | The Weight sub-TLV MUST NOT appear more than once inside the Segment List sub-TLV (§2.4.4.1) | MUST NOT | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) |
| `RFC9830-2.4.4.1-3` | The Weight sub-TLV Length value MUST be 6 (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) |
| `RFC9830-2.4.4.1-4` | The Weight Flags field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) |
| `RFC9830-2.4.4.1-5` | The Weight Flags field MUST be ignored on receipt (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L126). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L200) |
| `RFC9830-2.4.4.1-6` | The Weight RESERVED field MUST be set to zero on transmission (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383). **negative:** `unit/verify` [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) |
| `RFC9830-2.4.4.1-7` | The Weight RESERVED field MUST be ignored on receipt (§2.4.4.1) | MUST | 2.4.4.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L201) |
| `RFC9830-2.4.4.2.1-1` | The Type A Segment sub-TLV Length value MUST be 6 (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) |
| `RFC9830-2.4.4.2.1-2` | The Type A Segment RESERVED field MUST be set to zero on transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) |
| `RFC9830-2.4.4.2.1-3` | The Type A Segment RESERVED field MUST be ignored on receipt (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L202) |
| `RFC9830-2.4.4.2.1-4` | The Type A Segment S bit MUST be zero upon transmission (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413). **negative:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) |
| `RFC9830-2.4.4.2.1-5` | The Type A Segment S bit MUST be ignored upon reception (§2.4.4.2.1) | MUST | 2.4.4.2.1 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L203) |
| `RFC9830-2.4.4.2.2-1` | The Type B Segment sub-TLV Length value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present, else MUST be 18 (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) |
| `RFC9830-2.4.4.2.2-2` | The Type B Segment RESERVED field MUST be set to zero on transmission (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) |
| `RFC9830-2.4.4.2.2-3` | The Type B Segment RESERVED field MUST be ignored on receipt (§2.4.4.2.2) | MUST | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L204) |
| `RFC9830-2.4.4.2.2-4` | The Type B SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.4.2.2) | MUST NOT | 2.4.4.2.2 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) |
| `RFC9830-2.4.4.2.3-1` | The unassigned bits in the Segment Flags field MUST be set to zero upon transmission (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425). **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) |
| `RFC9830-2.4.4.2.3-2` | The unassigned bits in the Segment Flags field MUST be ignored upon receipt (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) |
| `RFC9830-2.4.4.2.3-3` | If the B-Flag appears with Segment Type A, it MUST be ignored (§2.4.4.2.3) | MUST | 2.4.4.2.3 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206) |
| `RFC9830-2.4.4.2.4-2` | The SRv6 Endpoint Behavior and SID Structure Reserved field MUST be set to zero on transmission (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462). **negative:** `unit/verify` [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) |
| `RFC9830-2.4.4.2.4-3` | The SRv6 Endpoint Behavior and SID Structure Reserved field MUST be ignored on receipt (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L154). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207) |
| `RFC9830-2.4.4.2.4-4` | The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | MUST | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| `RFC9830-2.4.5-2` | The ENLP sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.5) | MUST NOT | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| `RFC9830-2.4.5-3` | The ENLP sub-TLV Length value MUST be 3 (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-4` | The ENLP Flags field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-5` | The ENLP Flags field MUST be ignored on receipt (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| `RFC9830-2.4.5-6` | The ENLP RESERVED field MUST be set to zero on transmission (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| `RFC9830-2.4.5-7` | The ENLP RESERVED field MUST be ignored on receipt (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| `RFC9830-2.4.5-8` | Implementations MUST ignore the ENLP sub-TLV with unrecognized values (other than 1 through 4) (§2.4.5) | MUST | 2.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| `RFC9830-2.4.6-3` | The Priority sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.6) | MUST NOT | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) |
| `RFC9830-2.4.6-4` | The Priority sub-TLV Length value MUST be 2 (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339). **negative:** `unit/verify` [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) |
| `RFC9830-2.4.6-5` | The Priority RESERVED field MUST be set to zero on transmission (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343). **negative:** `unit/verify` [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) |
| `RFC9830-2.4.6-6` | The Priority RESERVED field MUST be ignored on receipt (§2.4.6) | MUST | 2.4.6 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L158). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208) |
| `RFC9830-2.4.7-5` | The SR Policy Candidate Path Name sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.7) | MUST NOT | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) |
| `RFC9830-2.4.7-6` | The SR Policy Candidate Path Name RESERVED field MUST be set to zero on transmission (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) |
| `RFC9830-2.4.7-7` | The SR Policy Candidate Path Name RESERVED field MUST be ignored on receipt (§2.4.7) | MUST | 2.4.7 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L160). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209) |
| `RFC9830-2.4.8-5` | The SR Policy Name sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.8) | MUST NOT | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) |
| `RFC9830-2.4.8-6` | The SR Policy Name RESERVED field MUST be set to zero on transmission (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496). **negative:** `unit/verify` [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) |
| `RFC9830-2.4.8-7` | The SR Policy Name RESERVED field MUST be ignored on receipt (§2.4.8) | MUST | 2.4.8 | **positive:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L162). **negative:** `unit/verify` [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210) |
| `RFC9830-3-3` | Upon reception, an implementation MUST treat Color-Only Type 3 (bits 11) like Type 0 (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| `RFC9830-4.1-2` | If no route target is attached, the NO_ADVERTISE community MUST be attached to the SR Policy update (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| `RFC9830-4.2.1-1` | A BGP speaker MUST first perform validation based on the §4.2.1 rules in addition to the validation in §5 (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| `RFC9830-4.2.1-2` | The SR Policy NLRI MUST include a Distinguisher, Color, and Endpoint field (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143). **negative:** `unit/verify` [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) |
| `RFC9830-4.2.1-3` | The length of the NLRI MUST be either 12 or 24 octets depending on the Endpoint address family (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| `RFC9830-4.2.1-4` | The SR Policy update MUST have either the NO_ADVERTISE community, at least one IPv4-address-format Route Target extended community, or both (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| `RFC9830-4.2.1-5` | An SR Policy update with no Route Target extended communities and no NO_ADVERTISE community MUST be considered malformed (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| `RFC9830-4.2.1-6` | The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215). **negative:** no negative test. **{single-polarity}:** parseConfigRoute attaches the Tunnel Encapsulation attribute to every SR Policy route it builds -- buildTunnelEncap always returns at least the 4-octet TLV header, so the len(tunnelEncapValue) > 0 guard never fails (internal/component/bgp/plugins/nlri/srpolicy/config.go:209-225, :365-370) -- leaving no update-without-the-attribute for a negative to observe. The receive-side obligation to call such an update malformed is the separate RFC9830-4.2.1-8, which is annotated as a gap |
| `RFC9830-4.2.1-7` | The Tunnel Encapsulation Attribute MUST have a Tunnel Type TLV set to SR Policy (code point 15) (§4.2.1) | MUST | 4.2.1 | **positive:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226). **negative:** `unit/verify` [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) |
| `RFC9830-4.2.1-8` | A router receiving an update not valid according to these criteria MUST treat the update as malformed (§4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-4.2.1-9` | An invalid SR Policy CP MUST NOT be passed to the SRPM (§4.2.1) | MUST NOT | 4.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| `RFC9830-4.2.2-1` | If route targets are present, at least one MUST match the BGP Identifier of the receiver for the update to be usable (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| `RFC9830-4.2.2-2` | The Route Target extended community MUST be of the same format (4-octet, unsigned, non-zero) as the BGP Identifier (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| `RFC9830-4.2.2-5` | When an update results in the SR Policy NLRI becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| `RFC9830-4.2.3-1` | SR Policy NLRIs that have the NO_ADVERTISE community MUST NOT be propagated (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| `RFC9830-4.2.3-2` | By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any EBGP neighbor (§4.2.3) | MUST NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| `RFC9830-4.2.3-6` | A BGP node MUST NOT alter the SR Policy information carried in the Tunnel Encapsulation Attribute during propagation (§4.2.3) | MUST NOT | 4.2.3 | **positive:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L179). **negative:** `unit/verify` [`TestRFC9012AllSubTLVsPropagate`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L193) |
| `RFC9830-5-1` | A BGP speaker MUST perform syntactic validation of the SR Policy NLRI (per-NLRI length, total MP_REACH_NLRI/MP_UNREACH_NLRI length, and consistency of NLRI length with the AFI and endpoint) to determine if malformed (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| `RFC9830-5-2` | When the error allows skipping the malformed NLRI(s) and continuing, the router MUST handle such malformed NLRIs as treat-as-withdraw (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| `RFC9830-5-4` | The router MUST perform session reset when the session is only used for SR Policy or when AFI/SAFI disable is not possible (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-5` | The validation of the TLVs/sub-TLVs defined in Section 2.4 MUST be performed to determine if they are malformed or invalid (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| `RFC9830-5-6` | The validation of the Tunnel Encapsulation Attribute and other TLVs/sub-TLVs (RFC 9012 Section 13) MUST be done as described in that document (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| `RFC9830-5-7` | In case of any error detected at the attribute or its TLV/sub-TLV level, the treat-as-withdraw strategy MUST be applied (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| `RFC9830-5-8` | An SR Policy update determined not valid per Section 4.2.1 MUST be handled by treat-as-withdraw (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| `RFC9830-5-9` | A BGP implementation MUST NOT perform semantic verification of the individual TLV/sub-TLV fields, nor consider the SR Policy update invalid or not usable based on such validation (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L325). **negative:** `unit/verify` [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L337) |
| `RFC9830-2.1-5` | When several CPs of the same SR Policy are signaled via BGP, it is RECOMMENDED that each NLRI use a different distinguisher (§2.1) | RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-3` | It is RECOMMENDED that the SRv6 Binding SID sub-TLV be used when signaling an SRv6 BSID for an SR Policy CP (§2.4.2) | RECOMMENDED | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-2` | It is RECOMMENDED that the size of the symbolic name for the CP be limited to 255 bytes (§2.4.7) | RECOMMENDED | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-2` | It is RECOMMENDED that the size of the symbolic name for the SR Policy be limited to 255 bytes (§2.4.8) | RECOMMENDED | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-2` | Color-Only Type 3 (bits 11) is reserved for future use and SHOULD NOT be used (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.1-1` | One or more route targets SHOULD be attached to the advertisement (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-3` | When the SR Policy tunnel type includes any unrecognized or unsupported sub-TLV, the update SHOULD NOT be considered usable (§4.2.2) | SHOULD NOT | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-4` | By default, a BGP node receiving an SR Policy NLRI SHOULD NOT remove the Route Target extended community before propagation (§4.2.3) | SHOULD NOT | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-3` | Where the error prevents processing the UPDATE, the router SHOULD handle such malformed NLRIs as AFI/SAFI disable when other AFI/SAFIs share the session (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-5-10` | An implementation SHOULD log any errors found during the above validation (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.1-4` | The BGP UPDATE message MAY also contain any of the BGP optional attributes (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-2` | A BGP speaker MAY remove the Tunnel Egress Endpoint and Color sub-TLVs from the Tunnel Encapsulation Attribute during propagation (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.3-4` | Other sub-TLVs without defined applicability to the SR Policy SAFI MAY be removed from the Tunnel Encapsulation Attribute during propagation (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.1-1` | The Preference sub-TLV is OPTIONAL (§2.4.1) | OPTIONAL | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.2-1` | The Binding SID sub-TLV is OPTIONAL (§2.4.2) | OPTIONAL | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-1` | The SRv6 Binding SID sub-TLV is OPTIONAL (§2.4.3) | OPTIONAL | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-2` | More than one SRv6 Binding SID sub-TLV MAY be signaled in the same SR Policy encoding (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.3-8` | The SRv6 Binding SID value 0 MAY be used to indicate the desired behavior without specifying the BSID (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-1` | The Segment List sub-TLV is OPTIONAL (§2.4.4) | OPTIONAL | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-2` | The Segment List sub-TLV MAY appear multiple times in the SR Policy encoding (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4-3` | The Segment List sub-TLV MAY contain a Weight sub-TLV (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.1-1` | The Weight sub-TLV is OPTIONAL (§2.4.4.1) | OPTIONAL | 2.4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-1` | The Segment sub-TLVs are OPTIONAL (§2.4.4.2) | OPTIONAL | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2-2` | The Segment sub-TLVs MAY appear multiple times in the Segment List sub-TLV (§2.4.4.2) | MAY | 2.4.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.1-6` | The receiver MAY override the originator's TC and/or TTL values, as determined by local policy (§2.4.4.2.1) | MAY | 2.4.4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.4.2.4-1` | The Segment Type sub-TLVs MAY contain the SRv6 Endpoint Behavior and SID Structure encoding (§2.4.4.2.4) | MAY | 2.4.4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-1` | The ENLP sub-TLV is OPTIONAL (§2.4.5) | OPTIONAL | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.5-9` | The behavior signaled in the ENLP sub-TLV MAY be overridden by local configuration (§2.4.5) | MAY | 2.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-1` | An operator MAY set the SR Policy Priority sub-TLV to indicate recomputation order upon topological change (§2.4.6) | MAY | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.6-2` | The Priority sub-TLV is OPTIONAL (§2.4.6) | OPTIONAL | 2.4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-1` | An operator MAY set the SR Policy Candidate Path Name sub-TLV to attach a symbolic name to the SR Policy CP (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-3` | Implementations MAY choose to truncate long CP names to 255 bytes when signaling via BGP (§2.4.7) | MAY | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.7-4` | The SR Policy Candidate Path Name sub-TLV is OPTIONAL (§2.4.7) | OPTIONAL | 2.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-1` | An operator MAY set the SR Policy Name sub-TLV to associate a symbolic name with the SR Policy (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-3` | Implementations MAY choose to truncate long SR Policy names to 255 bytes when signaling via BGP (§2.4.8) | MAY | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-2.4.8-4` | The SR Policy Name sub-TLV is OPTIONAL (§2.4.8) | OPTIONAL | 2.4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-1` | The Color Extended Community MAY be carried in any BGP UPDATE message whose AFI/SAFI is one of the families listed (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-3-4` | One or more Color Extended Communities MAY be associated with a BGP route update (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.2-4` | An implementation MAY provide an option for ignoring unsupported sub-TLVs (§4.2.2) | MAY | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-3` | An implementation MAY provide explicit configuration to override the EBGP default and enable propagation to specific EBGP neighbors (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9830-4.2.3-5` | An implementation MAY provide support for configuration to filter and/or remove the Route Target extended community before propagation (§4.2.3) | MAY | 4.2.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9830-2.2-1`](#rfc9830-2.2-1) Use of any Tunnel Type other than SR Policy with the SR Policy SAFI MUST be considered malformed and handled by treat-as-withdraw (§2.2) | {gap}, no test | nothing rejects a non-SR-Policy tunnel type under SAFI 73. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no attribute-level check ever runs on a received Tunnel Encapsulation attribute, and the attribute itself is kept as raw TLV bytes parsed only on demand (internal/core/bgp/attribute/wire.go:346 and :418, internal/core/bgp/attribute/tunnel_encap.go:39). ze parses and re-advertises the attribute; it applies no treat-as-withdraw |
| [`RFC9830-2.2-3`](#rfc9830-2.2-3) Updates carrying more than one SR Policy TLV MUST be considered malformed and handled by treat-as-withdraw (§2.2) | {gap}, no test | a Tunnel Encapsulation attribute holding two SR Policy TLVs is accepted. ParseTunnelEncap walks every TLV and appends each one without counting types (internal/core/bgp/attribute/tunnel_encap.go:41-54), and there is no RFC 7606 validator for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so no treat-as-withdraw is applied. ze's own encoder emits exactly one such TLV, which is the separate RFC9830-2.2-2 |
| [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5) The unassigned bits in the Binding SID Flags field MUST be set to zero upon transmission (§2.4.2) | {gap}, no test | buildBindingSIDSubTLV writes 0x10 into the Binding SID Flags octet (internal/component/bgp/plugins/nlri/srpolicy/config.go:385). Section 2.4.2 assigns only S (bit 0, 0x80) and I (bit 1, 0x40) in that field, so bit 3 is unassigned and is set on transmission. The value is pinned as ExaBGP-interoperable in TestSRPolicyInteropExaBGPSubTLVBytes (internal/component/bgp/plugins/nlri/srpolicy/encode_test.go:118-124), which emits the same octet, so the two implementations agree with each other and not with the RFC |
| [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11) The Binding SID Label field MUST NOT contain the reserved MPLS label values (0-15) (§2.4.2) | {gap}, no test | the config parser accepts any 32-bit binding-sid label and range-checks nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:137-142), so a reserved MPLS label value 0-15 is encoded verbatim by buildBindingSIDSubTLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:387-390). The TC, S and TTL bits ARE forced to zero on the same path, which is the separate RFC9830-2.4.2-9 |
| [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4) The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4) | {gap}, no test | the four SID-structure lengths are parsed as independent octets with no sum check (internal/component/bgp/plugins/nlri/srpolicy/config.go:312-318) and written verbatim into the segment sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:478-481), so a configuration whose locator block, locator node, function and argument lengths total more than 128 is encoded rather than refused |
| [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2) The ENLP sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither encodes nor interprets the ENLP sub-TLV, so it never writes a second instance. The SR Policy sub-TLV constant set holds no type 14 (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29), buildTunnelEncap has no ENLP branch (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363), and grep -rniE '\\benlp\\b\|explicit.?null.?label' over the Go tree matches only the OSPF MPLS Explicit NULL label, an unrelated feature |
| [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3) The ENLP sub-TLV Length value MUST be 3 (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze writes no ENLP sub-TLV, so it declares no length for one. There is no type-14 constant in the SR Policy encoder (internal/component/bgp/plugins/nlri/srpolicy/config.go:23-29) and no ENLP branch in buildTunnelEncap (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4) The ENLP Flags field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP Flags field to zero; the config keyword set has no ENLP spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:72-187) and buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5) The ENLP Flags field MUST be ignored on receipt (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP Flags field. Preference is the only typed sub-TLV accessor (internal/core/bgp/attribute/tunnel_encap.go:145) and the sub-TLV type constants stop at Segment List (internal/core/bgp/attribute/tunnel_encap.go:87-92) |
| [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6) The ENLP RESERVED field MUST be set to zero on transmission (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze transmits no ENLP sub-TLV, so it has no ENLP RESERVED octet to zero; buildTunnelEncap emits no type-14 sub-TLV (internal/component/bgp/plugins/nlri/srpolicy/config.go:339-363) |
| [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7) The ENLP RESERVED field MUST be ignored on receipt (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze decodes no ENLP sub-TLV, so it reads no ENLP RESERVED octet; the only typed sub-TLV accessor is Preference (internal/core/bgp/attribute/tunnel_encap.go:145) |
| [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8) Implementations MUST ignore the ENLP sub-TLV with unrecognized values (other than 1 through 4) (§2.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze interprets no ENLP value, recognized or not. The requirement presupposes a receiver that acts on values 1 through 4; ze has no ENLP decoder at all (internal/core/bgp/attribute/tunnel_encap.go:87-92, :145) and no Explicit NULL push driven by an SR Policy |
| [`RFC9830-3-3`](#rfc9830-3-3) Upon reception, an implementation MUST treat Color-Only Type 3 (bits 11) like Type 0 (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no color-based steering, so it never compares a route's Color Extended Community with an SR Policy and the Color-Only bits are never read. The eight octets are carried as an opaque extended community (internal/core/bgp/attribute/community.go, ParseExtendedCommunities) and grep -rniE 'color.?only\|colorExtended' over the Go tree matches only test names, with no producer that decodes the CO field |
| [`RFC9830-4.1-2`](#rfc9830-4.1-2) If no route target is attached, the NO_ADVERTISE community MUST be attached to the SR Policy update (§4.1) | {gap}, no test | ze attaches neither a route target nor NO_ADVERTISE to an SR Policy advertisement. parseConfigRoute builds exactly one attribute, the Tunnel Encapsulation attribute (internal/component/bgp/plugins/nlri/srpolicy/config.go:218-225), and deliberately ignores the pre-parsed attribute block that carries communities for other families (internal/component/bgp/plugins/nlri/srpolicy/config.go:43-44), so an SR Policy route ze originates carries no community at all |
| [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1) A BGP speaker MUST first perform validation based on the §4.2.1 rules in addition to the validation in §5 (§4.2.1) | {gap}, no test | no SR Policy validation runs on receipt. validateMPNLRISyntax returns nil for every SAFI other than unicast and multicast, so SAFI 73 is skipped (internal/component/bgp/message/rfc7606.go:701-704), and the RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429). A received SR Policy update reaches the RIB without any of the Section 4.2.1 checks |
| [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3) The length of the NLRI MUST be either 12 or 24 octets depending on the Endpoint address family (§4.2.1) | {gap}, no test | SplitSRPolicy accepts any non-zero byte-aligned length that fits the buffer and never compares it with the AFI's mandated 12 or 24 octets (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33), and its error is discarded by the RIB walk (internal/component/bgp/plugins/rib/rib_structured.go:229). The encoder always writes the mandated length, which is the separate RFC9830-2.1-2 |
| [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4) The SR Policy update MUST have either the NO_ADVERTISE community, at least one IPv4-address-format Route Target extended community, or both (§4.2.1) | {gap}, no test | nothing inspects the communities of a received SR Policy update. There is no RFC 7606 validator for attribute 23 and none for the SAFI (internal/component/bgp/message/rfc7606.go:415-429, :701-704), and no code path reads NO_ADVERTISE or a route target for SAFI 73: grep for SAFISRPolicy outside the NLRI codec matches only the family registry and the next-hop length table (internal/core/bgp/attribute/mpnlri.go:277) |
| [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5) An SR Policy update with no Route Target extended communities and no NO_ADVERTISE community MUST be considered malformed (§4.2.1) | {gap}, no test | an SR Policy update with neither a route target nor NO_ADVERTISE is accepted like any other. The malformed decision would have to come from an RFC 7606 validator for attribute 23 or for SAFI 73, and neither exists (internal/component/bgp/message/rfc7606.go:415-429, :701-704) |
| [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8) A router receiving an update not valid according to these criteria MUST treat the update as malformed (§4.2.1) | {gap}, no test | no receive-side validity criteria are evaluated, so none can drive a malformed verdict. SAFI 73 is excluded from the NLRI syntax check (internal/component/bgp/message/rfc7606.go:701-704) and attribute 23 has no validator (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9) An invalid SR Policy CP MUST NOT be passed to the SRPM (§4.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no SRPM. grep -rni '\\bsrpm\\b' over the Go tree matches nothing, the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), and no candidate path is ever handed to a policy manager, valid or otherwise |
| [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1) If route targets are present, at least one MUST match the BGP Identifier of the receiver for the update to be usable (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never makes an SR Policy locally usable, so there is no eligibility decision for a route target to gate. There is no SRPM (grep -rni '\\bsrpm\\b' over the Go tree matches nothing) and no SAFI 73 consumer outside the NLRI codec and the family registry (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40, internal/core/family/family.go:93) |
| [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2) The Route Target extended community MUST be of the same format (4-octet, unsigned, non-zero) as the BGP Identifier (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze compares no route target against its BGP Identifier for SAFI 73, because it makes no SR Policy locally usable; there is no SRPM and no local-use path (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40), so the format constraint governs a comparison ze never performs |
| [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5) When an update results in the SR Policy NLRI becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze holds no SR Policy candidate path to delete. grep -rni '\\bsrpm\\b' over the Go tree matches nothing and the SR Policy plugin keeps no state beyond the NLRI codec (internal/component/bgp/plugins/nlri/srpolicy/register.go:29-40); withdrawal of a SAFI 73 route removes the RIB entry and nothing else |
| [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1) SR Policy NLRIs that have the NO_ADVERTISE community MUST NOT be propagated (§4.2.3) | {gap}, no test | the egress gate consults only the operator-configured export filter chain and never inspects community values (writeUpdateGated, internal/component/bgp/reactor/session_write.go:263-283), so an SR Policy NLRI carrying NO_ADVERTISE is propagated unless an operator filter happens to match it. The same omission is disclosed for RFC 1997 |
| [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2) By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any EBGP neighbor (§4.2.3) | {gap}, no test | propagation of SAFI 73 is family-generic with no eBGP default. The reactor's forwarding path keys on the negotiated families and the per-peer export filter, and grep for SAFISRPolicy over internal/component/bgp/reactor matches nothing, so a received SR Policy NLRI is forwarded to an eBGP neighbor that negotiated the family like any other route |
| [`RFC9830-5-1`](#rfc9830-5-1) A BGP speaker MUST perform syntactic validation of the SR Policy NLRI (per-NLRI length, total MP_REACH_NLRI/MP_UNREACH_NLRI length, and consistency of NLRI length with the AFI and endpoint) to determine if malformed (§5) | {gap}, no test | the syntactic validation is partial and its verdict is discarded. SplitSRPolicy checks the framing -- zero length, byte alignment, buffer overrun (internal/component/bgp/plugins/nlri/srpolicy/split.go:22-33) -- but never the consistency of the length with the AFI and endpoint, and every caller drops its error (internal/component/bgp/plugins/rib/rib_structured.go:229). The MP attribute's own NLRI check skips SAFI 73 outright (internal/component/bgp/message/rfc7606.go:701-704) |
| [`RFC9830-5-2`](#rfc9830-5-2) When the error allows skipping the malformed NLRI(s) and continuing, the router MUST handle such malformed NLRIs as treat-as-withdraw (§5) | {gap}, no test | no SR Policy NLRI is ever treated as withdrawn for being malformed. SAFI 73 is excluded from validateMPNLRISyntax (internal/component/bgp/message/rfc7606.go:701-704), which is the only path that turns an NLRI-level error into an RFC 7606 action, and the splitter's error is discarded (internal/component/bgp/plugins/rib/rib_structured.go:229) |
| [`RFC9830-5-4`](#rfc9830-5-4) The router MUST perform session reset when the session is only used for SR Policy or when AFI/SAFI disable is not possible (§5) | {gap}, no test | no SR Policy error path exists, so no session reset can be reached from one. The RFC7606ActionSessionReset verdict is produced only by the checks in internal/component/bgp/message/rfc7606.go, and SAFI 73 reaches none of them (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |
| [`RFC9830-5-5`](#rfc9830-5-5) The validation of the TLVs/sub-TLVs defined in Section 2.4 MUST be performed to determine if they are malformed or invalid (§5) | {gap}, no test | only one Section 2.4 sub-TLV is validated. TunnelTLV.Preference checks the mandated 6-octet value length before reading it (internal/core/bgp/attribute/tunnel_encap.go:151); every other sub-TLV is walked for framing only by TunnelTLV.SubTLVs (internal/core/bgp/attribute/tunnel_encap.go:104-131) and no length, flag or field of the Binding SID, SRv6 Binding SID, Priority, Segment List, Weight, Segment or name sub-TLVs is checked |
| [`RFC9830-5-6`](#rfc9830-5-6) The validation of the Tunnel Encapsulation Attribute and other TLVs/sub-TLVs (RFC 9012 Section 13) MUST be done as described in that document (§5) | {gap}, no test | the RFC 9012 Section 13 structural validation exists but is never driven at reception. ParseTunnelEncap is registered as the parser for attribute 23 (internal/core/bgp/attribute/wire.go:418) and rejects broken TLV framing (internal/core/bgp/attribute/tunnel_encap.go:41-54), but attributes are parsed lazily, only when something reads them (internal/core/bgp/attribute/wire.go:346), and nothing reads attribute 23 for a SAFI 73 route; there is no RFC 7606 validator that would force the parse (internal/component/bgp/message/rfc7606.go:415-429) |
| [`RFC9830-5-7`](#rfc9830-5-7) In case of any error detected at the attribute or its TLV/sub-TLV level, the treat-as-withdraw strategy MUST be applied (§5) | {gap}, no test | an error at the attribute or sub-TLV level produces no treat-as-withdraw. The RFC 7606 validator table has no entry for attribute 23 (internal/component/bgp/message/rfc7606.go:415-429), so the ParseTunnelEncap and SubTLVs errors (internal/core/bgp/attribute/tunnel_encap.go:43, :109) are only ever seen by a caller that chose to parse, never by the UPDATE validation path |
| [`RFC9830-5-8`](#rfc9830-5-8) An SR Policy update determined not valid per Section 4.2.1 MUST be handled by treat-as-withdraw (§5) | {gap}, no test | the Section 4.2.1 validity criteria are not evaluated at all (see RFC9830-4.2.1-1), so no update can be handled as treat-as-withdraw for failing them; SAFI 73 reaches neither the NLRI check nor an attribute validator (internal/component/bgp/message/rfc7606.go:701-704, :415-429) |

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

The NLRI Length value MUST be 96 when AFI = 1 and 192 when AFI = 2 (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC9830NLRIAddressFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L114) | unit/verify | unproven |

### [`RFC9830-2.1-3`](#rfc9830-2.1-3)

A BGP UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with the SR Policy SAFI MUST also carry the BGP mandatory attributes (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L184) | unit/verify | unproven |
| positive | [`TestRFC9830UpdateCarriesMandatoryAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L178) | unit/verify | unproven |

### [`RFC9830-2.2-1`](#rfc9830-2.2-1)

Use of any Tunnel Type other than SR Policy with the SR Policy SAFI MUST be considered malformed and handled by treat-as-withdraw (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.2-1, so no unit is bound to it.

### [`RFC9830-2.2-2`](#rfc9830-2.2-2)

A Tunnel Encapsulation Attribute MUST NOT contain more than one TLV of type "SR Policy" (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SinglePolicyTLVPerAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L194) | unit/verify | unproven |

### [`RFC9830-2.2-3`](#rfc9830-2.2-3)

Updates carrying more than one SR Policy TLV MUST be considered malformed and handled by treat-as-withdraw (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.2-3, so no unit is bound to it.

### [`RFC9830-2.3-1`](#rfc9830-2.3-1)

If the Tunnel Egress Endpoint and Color sub-TLVs are present, a BGP speaker MUST ignore them (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L293) | unit/verify | unproven |
| positive | [`TestRFC9830EgressEndpointAndColorSubTLVsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L282) | unit/verify | unproven |

### [`RFC9830-2.3-3`](#rfc9830-2.3-3)

Any other sub-TLVs without explicitly defined applicability to the SR Policy SAFI MUST be ignored by the BGP speaker (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L312) | unit/verify | unproven |
| positive | [`TestRFC9012MeaninglessSubTLVIgnoredNotRemoved`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L303) | unit/verify | unproven |

### [`RFC9830-2.4-1`](#rfc9830-2.4-1)

For single-instance TLVs/sub-TLVs, only the first instance is used and the other instances MUST be ignored (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L278) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4-2`](#rfc9830-2.4-2)

The other (duplicate) instances of a single-instance TLV/sub-TLV MUST NOT be considered malformed (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9012MalformedAttributeIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L155) | unit/verify | unproven |
| positive | [`TestRFC9012DuplicateSingleInstanceSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9012_test.go#L269) | unit/verify | unproven |

### [`RFC9830-2.4.1-2`](#rfc9830-2.4.1-2)

The Preference sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L263) | unit/verify | unproven |

### [`RFC9830-2.4.1-3`](#rfc9830-2.4.1-3)

The Preference sub-TLV Length value MUST be 6 (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L262) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L250) | unit/verify | unproven |

### [`RFC9830-2.4.1-4`](#rfc9830-2.4.1-4)

The Preference Flags field MUST be set to zero on transmission (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L254) | unit/verify | unproven |

### [`RFC9830-2.4.1-5`](#rfc9830-2.4.1-5)

The Preference Flags field MUST be ignored on receipt (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L254) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L244) | unit/verify | unproven |

### [`RFC9830-2.4.1-6`](#rfc9830-2.4.1-6)

The Preference RESERVED field MUST be set to zero on transmission (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L260) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L256) | unit/verify | unproven |

### [`RFC9830-2.4.1-7`](#rfc9830-2.4.1-7)

The Preference RESERVED field MUST be ignored on receipt (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L255) | unit/verify | unproven |
| positive | [`TestRFC9830PreferenceFlagsAndReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L249) | unit/verify | unproven |

### [`RFC9830-2.4.2-2`](#rfc9830-2.4.2-2)

The Binding SID sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L304) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L300) | unit/verify | unproven |

### [`RFC9830-2.4.2-4`](#rfc9830-2.4.2-4)

The Binding SID Length value MUST be 18 when an SRv6 BSID is present, 6 when an SR-MPLS BSID is present, or 2 when no BSID is present (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L282) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L280) | unit/verify | unproven |

### [`RFC9830-2.4.2-5`](#rfc9830-2.4.2-5)

The unassigned bits in the Binding SID Flags field MUST be set to zero upon transmission (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.2-5, so no unit is bound to it.

### [`RFC9830-2.4.2-6`](#rfc9830-2.4.2-6)

The unassigned bits in the Binding SID Flags field MUST be ignored upon receipt (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L112) | unit/verify | unproven |

### [`RFC9830-2.4.2-7`](#rfc9830-2.4.2-7)

The Binding SID RESERVED field MUST be set to zero on transmission (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L297) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L285) | unit/verify | unproven |

### [`RFC9830-2.4.2-8`](#rfc9830-2.4.2-8)

The Binding SID RESERVED field MUST be ignored on receipt (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L195) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L114) | unit/verify | unproven |

### [`RFC9830-2.4.2-9`](#rfc9830-2.4.2-9)

The Binding SID Label TC, S, and TTL bits MUST be set to zero (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L294) | unit/verify | unproven |
| positive | [`TestRFC9830BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L289) | unit/verify | unproven |

### [`RFC9830-2.4.2-10`](#rfc9830-2.4.2-10)

The Binding SID Label TC, S, and TTL bits MUST be ignored (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L196) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L116) | unit/verify | unproven |

### [`RFC9830-2.4.2-11`](#rfc9830-2.4.2-11)

The Binding SID Label field MUST NOT contain the reserved MPLS label values (0-15) (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.2-11, so no unit is bound to it.

### [`RFC9830-2.4.3-3`](#rfc9830-2.4.3-3)

The SRv6 Binding SID Length value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present, else MUST be 18 (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L315) | unit/verify | unproven |

### [`RFC9830-2.4.3-4`](#rfc9830-2.4.3-4)

The unassigned bits in the SRv6 Binding SID Flags field MUST be set to zero upon transmission (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L327) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L322) | unit/verify | unproven |

### [`RFC9830-2.4.3-5`](#rfc9830-2.4.3-5)

The unassigned bits in the SRv6 Binding SID Flags field MUST be ignored upon receipt (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L197) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L118) | unit/verify | unproven |

### [`RFC9830-2.4.3-6`](#rfc9830-2.4.3-6)

The SRv6 Binding SID RESERVED field MUST be set to zero on transmission (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L328) | unit/verify | unproven |
| positive | [`TestRFC9830SRv6BindingSIDSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L324) | unit/verify | unproven |

### [`RFC9830-2.4.3-7`](#rfc9830-2.4.3-7)

The SRv6 Binding SID RESERVED field MUST be ignored on receipt (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

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

The Segment List RESERVED field MUST be set to zero on transmission (§2.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L368) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L364) | unit/verify | unproven |

### [`RFC9830-2.4.4-5`](#rfc9830-2.4.4-5)

The Segment List RESERVED field MUST be ignored on receipt (§2.4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L199) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L122) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-2`](#rfc9830-2.4.4.1-2)

The Weight sub-TLV MUST NOT appear more than once inside the Segment List sub-TLV (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L392) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L373) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-3`](#rfc9830-2.4.4.1-3)

The Weight sub-TLV Length value MUST be 6 (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L389) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L377) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-4`](#rfc9830-2.4.4.1-4)

The Weight Flags field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L385) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L381) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-5`](#rfc9830-2.4.4.1-5)

The Weight Flags field MUST be ignored on receipt (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L200) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L126) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-6`](#rfc9830-2.4.4.1-6)

The Weight RESERVED field MUST be set to zero on transmission (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L386) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentListSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L383) | unit/verify | unproven |

### [`RFC9830-2.4.4.1-7`](#rfc9830-2.4.4.1-7)

The Weight RESERVED field MUST be ignored on receipt (§2.4.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L201) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L130) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-1`](#rfc9830-2.4.4.2.1-1)

The Type A Segment sub-TLV Length value MUST be 6 (§2.4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L422) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L408) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-2`](#rfc9830-2.4.4.2.1-2)

The Type A Segment RESERVED field MUST be set to zero on transmission (§2.4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L419) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L411) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-3`](#rfc9830-2.4.4.2.1-3)

The Type A Segment RESERVED field MUST be ignored on receipt (§2.4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L134) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-4`](#rfc9830-2.4.4.2.1-4)

The Type A Segment S bit MUST be zero upon transmission (§2.4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L417) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L413) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.1-5`](#rfc9830-2.4.4.2.1-5)

The Type A Segment S bit MUST be ignored upon reception (§2.4.4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L138) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-1`](#rfc9830-2.4.4.2.2-1)

The Type B Segment sub-TLV Length value MUST be 26 when the SRv6 Endpoint Behavior and SID Structure is present, else MUST be 18 (§2.4.4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L445) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L443) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-2`](#rfc9830-2.4.4.2.2-2)

The Type B Segment RESERVED field MUST be set to zero on transmission (§2.4.4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L451) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L448) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-3`](#rfc9830-2.4.4.2.2-3)

The Type B Segment RESERVED field MUST be ignored on receipt (§2.4.4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L204) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L150) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.2-4`](#rfc9830-2.4.4.2.2-4)

The Type B SRv6 Endpoint Behavior and SID Structure MUST NOT be included when the SRv6 SID has not been included (§2.4.4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L471) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L468) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-1`](#rfc9830-2.4.4.2.3-1)

The unassigned bits in the Segment Flags field MUST be set to zero upon transmission (§2.4.4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L458) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeASubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L425) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L455) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-2`](#rfc9830-2.4.4.2.3-2)

The unassigned bits in the Segment Flags field MUST be ignored upon receipt (§2.4.4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L205) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L142) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.3-3`](#rfc9830-2.4.4.2.3-3)

If the B-Flag appears with Segment Type A, it MUST be ignored (§2.4.4.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L206) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L146) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-2`](#rfc9830-2.4.4.2.4-2)

The SRv6 Endpoint Behavior and SID Structure Reserved field MUST be set to zero on transmission (§2.4.4.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L464) | unit/verify | unproven |
| positive | [`TestRFC9830SegmentTypeBSubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L462) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-3`](#rfc9830-2.4.4.2.4-3)

The SRv6 Endpoint Behavior and SID Structure Reserved field MUST be ignored on receipt (§2.4.4.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L154) | unit/verify | unproven |

### [`RFC9830-2.4.4.2.4-4`](#rfc9830-2.4.4.2.4-4)

The total of the locator block, locator node, function, and argument lengths MUST be less than or equal to 128 (§2.4.4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.4.2.4-4, so no unit is bound to it.

### [`RFC9830-2.4.5-2`](#rfc9830-2.4.5-2)

The ENLP sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-2, so no unit is bound to it.

### [`RFC9830-2.4.5-3`](#rfc9830-2.4.5-3)

The ENLP sub-TLV Length value MUST be 3 (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-3, so no unit is bound to it.

### [`RFC9830-2.4.5-4`](#rfc9830-2.4.5-4)

The ENLP Flags field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-4, so no unit is bound to it.

### [`RFC9830-2.4.5-5`](#rfc9830-2.4.5-5)

The ENLP Flags field MUST be ignored on receipt (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-5, so no unit is bound to it.

### [`RFC9830-2.4.5-6`](#rfc9830-2.4.5-6)

The ENLP RESERVED field MUST be set to zero on transmission (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-6, so no unit is bound to it.

### [`RFC9830-2.4.5-7`](#rfc9830-2.4.5-7)

The ENLP RESERVED field MUST be ignored on receipt (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-7, so no unit is bound to it.

### [`RFC9830-2.4.5-8`](#rfc9830-2.4.5-8)

Implementations MUST ignore the ENLP sub-TLV with unrecognized values (other than 1 through 4) (§2.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-2.4.5-8, so no unit is bound to it.

### [`RFC9830-2.4.6-3`](#rfc9830-2.4.6-3)

The Priority sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L351) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L348) | unit/verify | unproven |

### [`RFC9830-2.4.6-4`](#rfc9830-2.4.6-4)

The Priority sub-TLV Length value MUST be 2 (§2.4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PriorityLengthIsValueLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L525) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L339) | unit/verify | unproven |

### [`RFC9830-2.4.6-5`](#rfc9830-2.4.6-5)

The Priority RESERVED field MUST be set to zero on transmission (§2.4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L345) | unit/verify | unproven |
| positive | [`TestRFC9830PrioritySubTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L343) | unit/verify | unproven |

### [`RFC9830-2.4.6-6`](#rfc9830-2.4.6-6)

The Priority RESERVED field MUST be ignored on receipt (§2.4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L208) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L158) | unit/verify | unproven |

### [`RFC9830-2.4.7-5`](#rfc9830-2.4.7-5)

The SR Policy Candidate Path Name sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L513) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L506) | unit/verify | unproven |

### [`RFC9830-2.4.7-6`](#rfc9830-2.4.7-6)

The SR Policy Candidate Path Name RESERVED field MUST be set to zero on transmission (§2.4.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L502) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L495) | unit/verify | unproven |

### [`RFC9830-2.4.7-7`](#rfc9830-2.4.7-7)

The SR Policy Candidate Path Name RESERVED field MUST be ignored on receipt (§2.4.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L160) | unit/verify | unproven |

### [`RFC9830-2.4.8-5`](#rfc9830-2.4.8-5)

The SR Policy Name sub-TLV MUST NOT appear more than once in the SR Policy encoding (§2.4.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L514) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L507) | unit/verify | unproven |

### [`RFC9830-2.4.8-6`](#rfc9830-2.4.8-6)

The SR Policy Name RESERVED field MUST be set to zero on transmission (§2.4.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L503) | unit/verify | unproven |
| positive | [`TestRFC9830NameSubTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L496) | unit/verify | unproven |

### [`RFC9830-2.4.8-7`](#rfc9830-2.4.8-7)

The SR Policy Name RESERVED field MUST be ignored on receipt (§2.4.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC9830ReceivedFieldsAreIgnoredNotRead`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L162) | unit/verify | unproven |

### [`RFC9830-3-3`](#rfc9830-3-3)

Upon reception, an implementation MUST treat Color-Only Type 3 (bits 11) like Type 0 (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-3-3, so no unit is bound to it.

### [`RFC9830-4.1-2`](#rfc9830-4.1-2)

If no route target is attached, the NO_ADVERTISE community MUST be attached to the SR Policy update (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.1-2, so no unit is bound to it.

### [`RFC9830-4.2.1-1`](#rfc9830-4.2.1-1)

A BGP speaker MUST first perform validation based on the §4.2.1 rules in addition to the validation in §5 (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-1, so no unit is bound to it.

### [`RFC9830-4.2.1-2`](#rfc9830-4.2.1-2)

The SR Policy NLRI MUST include a Distinguisher, Color, and Endpoint field (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L151) | unit/verify | unproven |
| positive | [`TestRFC9830NLRICarriesAllThreeFields`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L143) | unit/verify | unproven |

### [`RFC9830-4.2.1-3`](#rfc9830-4.2.1-3)

The length of the NLRI MUST be either 12 or 24 octets depending on the Endpoint address family (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-3, so no unit is bound to it.

### [`RFC9830-4.2.1-4`](#rfc9830-4.2.1-4)

The SR Policy update MUST have either the NO_ADVERTISE community, at least one IPv4-address-format Route Target extended community, or both (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-4, so no unit is bound to it.

### [`RFC9830-4.2.1-5`](#rfc9830-4.2.1-5)

An SR Policy update with no Route Target extended communities and no NO_ADVERTISE community MUST be considered malformed (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-5, so no unit is bound to it.

### [`RFC9830-4.2.1-6`](#rfc9830-4.2.1-6)

The Tunnel Encapsulation Attribute MUST be attached to the BGP UPDATE message (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L215) | unit/verify | unproven |

### [`RFC9830-4.2.1-7`](#rfc9830-4.2.1-7)

The Tunnel Encapsulation Attribute MUST have a Tunnel Type TLV set to SR Policy (code point 15) (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L234) | unit/verify | unproven |
| positive | [`TestRFC9830TunnelTypeIsSRPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9830_test.go#L226) | unit/verify | unproven |

### [`RFC9830-4.2.1-8`](#rfc9830-4.2.1-8)

A router receiving an update not valid according to these criteria MUST treat the update as malformed (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-8, so no unit is bound to it.

### [`RFC9830-4.2.1-9`](#rfc9830-4.2.1-9)

An invalid SR Policy CP MUST NOT be passed to the SRPM (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.1-9, so no unit is bound to it.

### [`RFC9830-4.2.2-1`](#rfc9830-4.2.2-1)

If route targets are present, at least one MUST match the BGP Identifier of the receiver for the update to be usable (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-1, so no unit is bound to it.

### [`RFC9830-4.2.2-2`](#rfc9830-4.2.2-2)

The Route Target extended community MUST be of the same format (4-octet, unsigned, non-zero) as the BGP Identifier (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-2, so no unit is bound to it.

### [`RFC9830-4.2.2-5`](#rfc9830-4.2.2-5)

When an update results in the SR Policy NLRI becoming unusable, BGP MUST delete its corresponding SR Policy CP from the SRPM (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.2-5, so no unit is bound to it.

### [`RFC9830-4.2.3-1`](#rfc9830-4.2.3-1)

SR Policy NLRIs that have the NO_ADVERTISE community MUST NOT be propagated (§4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-4.2.3-1, so no unit is bound to it.

### [`RFC9830-4.2.3-2`](#rfc9830-4.2.3-2)

By default, a BGP node receiving an SR Policy NLRI MUST NOT propagate it to any EBGP neighbor (§4.2.3)

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

A BGP speaker MUST perform syntactic validation of the SR Policy NLRI (per-NLRI length, total MP_REACH_NLRI/MP_UNREACH_NLRI length, and consistency of NLRI length with the AFI and endpoint) to determine if malformed (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-1, so no unit is bound to it.

### [`RFC9830-5-2`](#rfc9830-5-2)

When the error allows skipping the malformed NLRI(s) and continuing, the router MUST handle such malformed NLRIs as treat-as-withdraw (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-2, so no unit is bound to it.

### [`RFC9830-5-4`](#rfc9830-5-4)

The router MUST perform session reset when the session is only used for SR Policy or when AFI/SAFI disable is not possible (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-4, so no unit is bound to it.

### [`RFC9830-5-5`](#rfc9830-5-5)

The validation of the TLVs/sub-TLVs defined in Section 2.4 MUST be performed to determine if they are malformed or invalid (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-5, so no unit is bound to it.

### [`RFC9830-5-6`](#rfc9830-5-6)

The validation of the Tunnel Encapsulation Attribute and other TLVs/sub-TLVs (RFC 9012 Section 13) MUST be done as described in that document (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-6, so no unit is bound to it.

### [`RFC9830-5-7`](#rfc9830-5-7)

In case of any error detected at the attribute or its TLV/sub-TLV level, the treat-as-withdraw strategy MUST be applied (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-7, so no unit is bound to it.

### [`RFC9830-5-8`](#rfc9830-5-8)

An SR Policy update determined not valid per Section 4.2.1 MUST be handled by treat-as-withdraw (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9830-5-8, so no unit is bound to it.

### [`RFC9830-5-9`](#rfc9830-5-9)

A BGP implementation MUST NOT perform semantic verification of the individual TLV/sub-TLV fields, nor consider the SR Policy update invalid or not usable based on such validation (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L337) | unit/verify | unproven |
| positive | [`TestRFC9830NoSemanticVerification`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc9830_test.go#L325) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9830, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9830, so its obligations are stated where they were written.
