# RFC 5392 - OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering

Experimental. Every requirement this repository extracted from RFC 5392, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.0% | 3 of 12 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 41.7% | 5 of 12 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 4 of 12 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 12 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 30 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 11 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5392.md` |
| Requirement shard | `rfc/requirements/rfc5392.md` |
| RFC text | `rfc/full/rfc5392.txt` |

## Enrolment

Enrolled: OSPF inter-AS TE (RFC 5392): OSPFv2 Opaque-type-6; 3 MET (Remote-AS required, Link-ID prohibited, re-advert rate-limit) + 5 single-polarity positive + 4 gap (OSPFv3 Inter-AS-TE-v3 function code 13 unimplemented)

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- OSPFv2 Inter-AS-TE-v2 (Opaque type 6): Remote-AS (21), IPv4/IPv6 Remote-ASBR-ID (22/24) sub-TLVs
- Link-ID prohibition and Remote-AS requirement enforced on originate and receive
- MinLSInterval-paced proxy origination with no adjacency or Hellos.


**What the ledger says remains**

Four MUST gaps: the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is unimplemented, so the U-bit=1 rule ([`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1)), the v3 Neighbor-ID prohibition ([`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2)), and the v3 IPv6/IPv4 Remote-ASBR-ID inclusion rules ([`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1), [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2)) have no v3 carrier to bind.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5392-3.2.1-4`](#rfc5392-3.2.1-4), [`RFC5392-3.2.1-1`](#rfc5392-3.2.1-1), [`RFC5392-4-3`](#rfc5392-4-3)

**Annotated instead of tested (9):** [`RFC5392-3.3.1-1`](#rfc5392-3.3.1-1), [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1), [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2), [`RFC5392-3.3.2-1`](#rfc5392-3.3.2-1), [`RFC5392-3.3.2-2`](#rfc5392-3.3.2-2), [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1), [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2), [`RFC5392-4-1`](#rfc5392-4-1), [`RFC5392-4-2`](#rfc5392-4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5392-3.2.1-4` | The Remote-AS-Number sub-TLV (type 21) is included in the Link TLV of both the Inter-AS-TE-v2 and Inter-AS-TE-v3 LSA; it is REQUIRED in any Link TLV advertising an inter-AS TE link (§3.2.1, §3.3.1) -- Ze (v2): `remote-as` mandatory in YANG + validateConfig; emitted as sub-TLV 21 (spec-ospf-ext-2) | REQUIRED | 3.2.1 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L250). **negative:** `unit/verify` [`TestTEReceiveType6MissingRemoteASSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_test.go#L73) |
| `RFC5392-3.3.1-1` | When only two octets are used for the AS number, the left (high-order) two octets of the Remote AS Number field MUST be set to zero (§3.3.1) -- Ze encodes the 4-octet field big-endian from a uint32, so a 2-byte ASN is zero-extended | MUST | 3.3.1 | **positive:** `unit/verify` [`TestInterAsTERemoteAsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/te_interas_test.go#L29). **negative:** no negative test. **{single-polarity}:** ze stores remote-as as a uint32 and encodes it big-endian into the fixed 4-octet field, so a 2-byte ASN is zero-extended by construction and no code path can set the high octets non-zero (internal/plugins/ospf/packet/te_interas.go:36, te_lsa.go:133) |
| `RFC5392-3.1.2-1` | The Inter-AS-TE-v3 U-bit is always set to 1 so an OSPFv3 router floods the LSA at its defined flooding scope even if it does not recognize the LS type (§3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates inter-AS TE only as the OSPFv2 Opaque-type-6 LSA; the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is not implemented, so there is no LS Type or U-bit to set (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.2.1-1` | The Link ID sub-TLV MUST NOT be used in the Link TLV of an Inter-AS-TE-v2 LSA (§3.2.1) -- Ze never emits sub-TLV 2 for an inter-AS link, and a received type-6 Link TLV carrying it is skipped (validateReceivedTELink) | MUST NOT | 3.2.1 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L245). **negative:** `unit/verify` [`TestTEReceiveMalformedNoEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_test.go#L52) |
| `RFC5392-3.2.1-2` | The Neighbor ID sub-TLV MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so there is no v3 inter-AS Link TLV in which a Neighbor ID sub-TLV could be emitted or prohibited (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.3.2-1` | In OSPFv2 advertisements, the IPv4 Remote ASBR ID sub-TLV (type 22) MUST be included if the neighboring ASBR has an IPv4 address (§3.3.2) -- Ze: `remote-asbr-ipv4` leaf emitted as sub-TLV 22; validateConfig requires at least one remote-asbr (spec-ospf-ext-2) | MUST | 3.3.2 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L252). **negative:** no negative test. **{single-polarity}:** ze emits sub-TLV 22 whenever the operator configures remote-asbr-ipv4; the remote ASBR's addresses are proxied from config, so ze cannot independently detect an IPv4 address and there is no adversarial negative (internal/plugins/ospf/packet/te_interas.go:38-39) |
| `RFC5392-3.3.2-2` | In OSPFv2, if the neighboring ASBR has no IPv4 address (not even an IPv4 TE Router ID), the IPv6 Remote ASBR ID sub-TLV MUST be included instead (§3.3.2) | MUST | 3.3.2 | **positive:** `unit/verify` [`TestInterAsTEIPv6AsbrIdType24`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/te_interas_test.go#L91). **negative:** no negative test. **{single-polarity}:** validateConfig requires at least one Remote ASBR ID, so a v4-less inter-AS link carries the IPv6 Remote ASBR ID sub-TLV 24; the selection is operator config and the only enforced rejection is neither-present (internal/plugins/ospf/te_config.go:163, packet/te_interas.go:41-44) |
| `RFC5392-3.3.3-1` | In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV (type 24) MUST be included if the neighboring ASBR has an IPv6 address (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv3 inter-AS TE (function code 13) is not implemented, so ze originates no OSPFv3 advertisement in which to require the IPv6 Remote ASBR ID (the type-24 codec exists but is only ever emitted into a v2 LSA) (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.3.3-2` | In OSPFv3, if the neighboring ASBR has no IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so the v3 IPv4-fallback rule has no origination path to bind (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-4-1` | Hellos MUST NOT be exchanged over the inter-AS link (§4) -- Ze proxies the inter-AS link from config only; the `inter-as` block forms no adjacency and sends no Hello on that link | MUST NOT | 4 | **positive:** `unit/verify` [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L278). **negative:** no negative test. **{single-polarity}:** ze's inter-AS TE advertisement is a config-only proxy that forms no adjacency and requires no Full neighbor, so the feature originates no Hellos; ze provides no guard forcing the interface passive, so there is no enforced rejection to exercise as a negative (internal/plugins/ospf/te_originate.go:167-202, te_config.go:53) |
| `RFC5392-4-2` | An OSPF adjacency MUST NOT be formed on the inter-AS link (§4) -- the inter-AS advertisement is config-driven (a passive/loopback proxy link); no FSM runs on it | MUST NOT | 4 | **positive:** `unit/verify` [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L281). **negative:** no negative test. **{single-polarity}:** inter-AS TE origination is decoupled from the adjacency FSM and never consults neighbor state, so the feature forms no adjacency; there is no guard rejecting an inter-as block on a non-passive interface, so no testable negative exists (internal/plugins/ospf/te_originate.go:167-202, te_config.go:45-53) |
| `RFC5392-4-3` | When re-advertising on TE parameter change, the ASBR MUST take precautions against excessive re-advertisements as described in [RFC3630] (§4) | MUST | 4 | **positive:** `unit/verify` [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_originate_test.go#L143). **negative:** `unit/verify` [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_originate_test.go#L153) |
| `RFC5392-3.1.1-1` | The inter-AS TE link advertisement SHOULD be carried in a Type 10 Opaque LSA when flooding scope is limited to the ASBR's IGP area (§3.1.1) -- Ze: `inter-as scope area` (the default) originates a Type 10 opaque LSA | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.1-2` | Configuration control of the Type 10 vs Type 11 (Inter-AS-TE-v2) choice SHOULD be provided in ASBR implementations that advertise inter-AS TE links (§3.1.1) -- Ze: the `inter-as scope { area \| as }` leaf selects Type 10 vs Type 11 per link | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-2` | For the Inter-AS-TE-v3 LSA, the S2/S1 bits SHOULD be set to 01 to limit flooding scope to the ASBR's IGP area (§3.1.2) | SHOULD | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-3` | Configuration control of the 01 vs 10 (Inter-AS-TE-v3) scope choice SHOULD be provided in ASBR implementations that advertise inter-AS TE links (§3.1.2) | SHOULD | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.2.1-3` | At least one of the IPv4-Remote-ASBR-ID and IPv6-Remote-ASBR-ID sub-TLV SHOULD be included in the Link TLV of both LSAs (§3.2.1) -- Ze: validateConfig requires at least one of `remote-asbr-ipv4` / `remote-asbr-ipv6` | SHOULD | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-4` | When TE is enabled on an inter-AS link and the link is up, the ASBR SHOULD advertise this link using normal OSPF-TE procedures (§4) -- Ze originates the Opaque-type-6 LSA via the standard opaque origination pass | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-5` | When the link is down or TE is disabled, the ASBR SHOULD withdraw the advertisement (§4) -- Ze's pull-model origination emits a Withdraw for a removed inter-AS instance (MaxAge-flush) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-6` | On TE parameter change, the ASBR SHOULD re-advertise the link (§4) -- a changed body re-originates on the next self-LSA pass under the carrier's MinLSInterval rate-limit | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-7` | Routers/PCEs SHOULD NOT use inter-AS TE links to compute paths that exit an AS to a remote ASBR then immediately re-enter the AS through another TE link (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-8` | Such exit-and-re-enter paths SHOULD NOT be allowed except as a result of specific policy configurations at the computing router or PCE (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-5-1` | If a different remote AS number is received in a BGP OPEN than locally configured into OSPF-TE, local policy SHOULD be applied to alert the operator or suppress the OSPF advertisement (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-5-2` | If BGP is used to exchange TE information (§4.1), the inter-AS BGP session SHOULD be secured per [RFC4271] for authentication and integrity (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.2-3` | Use of the TE Router ID from the Router Address TLV [RFC3630] is RECOMMENDED for the IPv4 Remote ASBR ID value (§3.3.2) | RECOMMENDED | 3.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.3-3` | Use of the IPv6 TE Router ID from the IPv6 Router Address TLV [RFC5329] is RECOMMENDED for the IPv6 Remote ASBR ID value (§3.3.3) | RECOMMENDED | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-2.1-1` | TE aggregation is not supported or recommended (§2.1, §3) | NOT RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.1-3` | The inter-AS TE link advertisement MAY be carried in a Type 11 Opaque LSA when the information is intended to reach all routers (ABRs, ASBRs, PCEs) in the AS (Inter-AS-TE-v2) (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-4` | For the Inter-AS-TE-v3 LSA, the S2/S1 bits MAY be set to 10 when the information should reach all routers (ABRs, ASBRs, PCEs) in the AS (§3.1.2) | MAY | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.2-4` | An IPv4 Remote ASBR ID sub-TLV and an IPv6 Remote ASBR ID sub-TLV MAY both be present in a Link TLV in OSPFv2 or OSPFv3 (§3.3.2, §3.3.3) | MAY | 3.3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1) The Inter-AS-TE-v3 U-bit is always set to 1 so an OSPFv3 router floods the LSA at its defined flooding scope even if it does not recognize the LS type (§3.1.2) | {gap}, no test | ze originates inter-AS TE only as the OSPFv2 Opaque-type-6 LSA; the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is not implemented, so there is no LS Type or U-bit to set (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2) The Neighbor ID sub-TLV MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1) | {gap}, no test | ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so there is no v3 inter-AS Link TLV in which a Neighbor ID sub-TLV could be emitted or prohibited (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1) In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV (type 24) MUST be included if the neighboring ASBR has an IPv6 address (§3.3.3) | {gap}, no test | OSPFv3 inter-AS TE (function code 13) is not implemented, so ze originates no OSPFv3 advertisement in which to require the IPv6 Remote ASBR ID (the type-24 codec exists but is only ever emitted into a v2 LSA) (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2) In OSPFv3, if the neighboring ASBR has no IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead (§3.3.3) | {gap}, no test | ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so the v3 IPv4-fallback rule has no origination path to bind (internal/plugins/ospf/te.go:88-91) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5392-3.2.1-4`](#rfc5392-3.2.1-4)

The Remote-AS-Number sub-TLV (type 21) is included in the Link TLV of both the Inter-AS-TE-v2 and Inter-AS-TE-v3 LSA; it is REQUIRED in any Link TLV advertising an inter-AS TE link (§3.2.1, §3.3.1) -- Ze (v2): `remote-as` mandatory in YANG + validateConfig; emitted as sub-TLV 21 (spec-ospf-ext-2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTEReceiveType6MissingRemoteASSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_test.go#L73) | unit/verify | unproven |
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L250) | unit/verify | unproven |

### [`RFC5392-3.3.1-1`](#rfc5392-3.3.1-1)

When only two octets are used for the AS number, the left (high-order) two octets of the Remote AS Number field MUST be set to zero (§3.3.1) -- Ze encodes the 4-octet field big-endian from a uint32, so a 2-byte ASN is zero-extended

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTERemoteAsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/te_interas_test.go#L29) | unit/verify | unproven |

### [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1)

The Inter-AS-TE-v3 U-bit is always set to 1 so an OSPFv3 router floods the LSA at its defined flooding scope even if it does not recognize the LS type (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.1.2-1, so no unit is bound to it.

### [`RFC5392-3.2.1-1`](#rfc5392-3.2.1-1)

The Link ID sub-TLV MUST NOT be used in the Link TLV of an Inter-AS-TE-v2 LSA (§3.2.1) -- Ze never emits sub-TLV 2 for an inter-AS link, and a received type-6 Link TLV carrying it is skipped (validateReceivedTELink)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTEReceiveMalformedNoEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_test.go#L52) | unit/verify | unproven |
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L245) | unit/verify | unproven |

### [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2)

The Neighbor ID sub-TLV MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.2.1-2, so no unit is bound to it.

### [`RFC5392-3.3.2-1`](#rfc5392-3.3.2-1)

In OSPFv2 advertisements, the IPv4 Remote ASBR ID sub-TLV (type 22) MUST be included if the neighboring ASBR has an IPv4 address (§3.3.2) -- Ze: `remote-asbr-ipv4` leaf emitted as sub-TLV 22; validateConfig requires at least one remote-asbr (spec-ospf-ext-2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L252) | unit/verify | unproven |

### [`RFC5392-3.3.2-2`](#rfc5392-3.3.2-2)

In OSPFv2, if the neighboring ASBR has no IPv4 address (not even an IPv4 TE Router ID), the IPv6 Remote ASBR ID sub-TLV MUST be included instead (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTEIPv6AsbrIdType24`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/te_interas_test.go#L91) | unit/verify | unproven |

### [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1)

In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV (type 24) MUST be included if the neighboring ASBR has an IPv6 address (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.3.3-1, so no unit is bound to it.

### [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2)

In OSPFv3, if the neighboring ASBR has no IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.3.3-2, so no unit is bound to it.

### [`RFC5392-4-1`](#rfc5392-4-1)

Hellos MUST NOT be exchanged over the inter-AS link (§4) -- Ze proxies the inter-AS link from config only; the `inter-as` block forms no adjacency and sends no Hello on that link

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L278) | unit/verify | unproven |

### [`RFC5392-4-2`](#rfc5392-4-2)

An OSPF adjacency MUST NOT be formed on the inter-AS link (§4) -- the inter-AS advertisement is config-driven (a passive/loopback proxy link); no FSM runs on it

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/te_originate_test.go#L281) | unit/verify | unproven |

### [`RFC5392-4-3`](#rfc5392-4-3)

When re-advertising on TE parameter change, the ASBR MUST take precautions against excessive re-advertisements as described in [RFC3630] (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_originate_test.go#L153) | unit/verify | unproven |
| positive | [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/opaque_originate_test.go#L143) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5392, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5392, so its obligations are stated where they were written.
