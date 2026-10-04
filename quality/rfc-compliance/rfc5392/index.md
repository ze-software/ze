# RFC 5392 - OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering

Experimental. Every requirement this repository extracted from RFC 5392, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 38.5% | 5 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 23.1% | 3 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 35.7% | 5 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 31 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 38.5% | 5 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 9 | of 13 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 31 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 9 |
| Discrimination records | 5 |
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

Five MUST gaps: the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is unimplemented, so the U-bit=1 rule ([`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1)), the v3 Neighbor-ID prohibition ([`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2)), the v3 Remote-AS-Number inclusion rule ([`RFC5392-3.2.1-5`](#rfc5392-3.2.1-5)), and the v3 IPv6/IPv4 Remote-ASBR-ID inclusion rules ([`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1), [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2)) have no v3 carrier to bind.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5392-3.2.1-4`](#rfc5392-3.2.1-4), [`RFC5392-3.2.1-1`](#rfc5392-3.2.1-1), [`RFC5392-4-1`](#rfc5392-4-1), [`RFC5392-4-2`](#rfc5392-4-2), [`RFC5392-4-3`](#rfc5392-4-3)

**Annotated (including scoped evidence) (8):** [`RFC5392-3.3.1-1`](#rfc5392-3.3.1-1), [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1), [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2), [`RFC5392-3.2.1-5`](#rfc5392-3.2.1-5), [`RFC5392-3.3.2-1`](#rfc5392-3.3.2-1), [`RFC5392-3.3.2-2`](#rfc5392-3.3.2-2), [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1), [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5392-3.2.1-4` | The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA (§3.2.1) -- Ze: `remote-as` mandatory in YANG + validateConfig; emitted as sub-TLV 21 in every Inter-AS-TE-v2 Link TLV, and a received type-6 Link TLV without it is skipped; the Inter-AS-TE-v3 clause is RFC5392-3.2.1-5; spec-ospf-ext-2 | MUST | 3.2.1 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L250). **negative:** `unit/verify` [`TestTEReceiveType6MissingRemoteASSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_test.go#L73) |
| `RFC5392-3.3.1-1` | When only two octets are used for the AS number, as in current deployments, the left (high- order) two octets MUST be set to zero. (§3.3.1) -- Ze encodes the 4-octet field big-endian from a uint32, so a 2-byte ASN is zero-extended | MUST | 3.3.1 | **positive:** `unit/verify` [`TestInterAsTERemoteAsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5392_te_interas_test.go#L29). **negative:** no negative test. **{single-polarity}:** ze stores remote-as as a uint32 and encodes it big-endian into the fixed 4-octet field, so a 2-byte ASN is zero-extended by construction and no code path can set the high octets non-zero (internal/plugins/ospf/packet/te_interas.go:36, te_lsa.go:133) |
| `RFC5392-3.1.2-1` | The U-bit is always set to 1 to indicate that an OSPFv3 router MUST flood the LSA at its defined flooding scope even if it does not recognize the LS type. (§3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze originates inter-AS TE only as the OSPFv2 Opaque-type-6 LSA; the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is not implemented, so there is no LS Type or U-bit to set (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.2.1-1` | The Link ID sub-TLV [OSPF-TE] MUST NOT be used in the Link TLV of an Inter-AS-TE-v2 LSA (§3.2.1) -- Ze never emits sub-TLV 2 for an inter-AS link, and a received type-6 Link TLV carrying it is skipped by validateReceivedTELink | MUST NOT | 3.2.1 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L245). **negative:** `unit/verify` [`TestTEReceiveMalformedNoEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_test.go#L52) |
| `RFC5392-3.2.1-2` | the Neighbor ID sub-TLV [OSPF-V3-TE] MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so there is no v3 inter-AS Link TLV in which a Neighbor ID sub-TLV could be emitted or prohibited (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.2.1-5` | The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA. (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Inter-AS-TE-v3 clause only (the Inter-AS-TE-v2 clause is RFC5392-3.2.1-4, met): ze implements no OSPFv3 Inter-AS-TE-v3 LSA (function code 13), so there is no v3 inter-AS Link TLV in which to include the Remote-AS-Number sub-TLV (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.3.2-1` | In OSPFv2 advertisements, the IPv4 Remote ASBR ID sub-TLV MUST be included if the neighboring ASBR has an IPv4 address. (§3.3.2) -- Ze: `remote-asbr-ipv4` leaf emitted as sub-TLV 22; validateConfig requires at least one remote-asbr; spec-ospf-ext-2 | MUST | 3.3.2 | **positive:** `unit/verify` [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L252). **negative:** no negative test. **{single-polarity}:** ze emits sub-TLV 22 whenever the operator configures remote-asbr-ipv4; the remote ASBR's addresses are proxied from config, so ze cannot independently detect an IPv4 address and there is no adversarial negative (internal/plugins/ospf/packet/te_interas.go:38-39) |
| `RFC5392-3.3.2-2` | If the neighboring ASBR does not have an IPv4 address (not even an IPv4 TE Router ID), the IPv6 Remote ASBR ID sub-TLV MUST be included instead. (§3.3.2) | MUST | 3.3.2 | **positive:** `unit/verify` [`TestInterAsTEIPv6AsbrIdType24`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5392_te_interas_test.go#L91). **positive:** `unit/verify` [`TestRFC5392OriginatedV6OnlyLinkCarriesIPv6RemoteASBRID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_v6_remote_asbr_origination_test.go#L15). **negative:** no negative test. **{single-polarity}:** validateConfig requires at least one Remote ASBR ID, so a v4-less inter-AS link carries the IPv6 Remote ASBR ID sub-TLV 24; the selection is operator config and the only enforced rejection is neither-present (internal/plugins/ospf/te_config.go:163, packet/te_interas.go:41-44) |
| `RFC5392-3.3.3-1` | In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV MUST be included if the neighboring ASBR has an IPv6 address. (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** OSPFv3 inter-AS TE (function code 13) is not implemented, so ze originates no OSPFv3 advertisement in which to require the IPv6 Remote ASBR ID (the type-24 codec exists but is only ever emitted into a v2 LSA) (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-3.3.3-2` | If the neighboring ASBR does not have an IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead. (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so the v3 IPv4-fallback rule has no origination path to bind (internal/plugins/ospf/te.go:88-91) |
| `RFC5392-4-1` | Hellos MUST NOT be exchanged over the inter-AS link (§4) -- an interface carrying an `inter-as` block is forced passive at config parse by parseInterface in internal/plugins/ospf/config.go, so no Hello is sent on it | MUST NOT | 4 | **positive:** `unit/verify` [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L278). **negative:** `unit/verify` [`TestRFC5392InterASInterfaceIsPassive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_interas_passive_test.go#L33) |
| `RFC5392-4-2` | Hellos MUST NOT be exchanged over the inter-AS link, and consequently, an OSPF adjacency MUST NOT be formed. (§4) -- the forced-passive interface is not an active interface, so no neighbor FSM runs and no adjacency forms on it | MUST NOT | 4 | **positive:** `unit/verify` [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L281). **negative:** `unit/verify` [`TestRFC5392InterASInterfaceIsPassive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_interas_passive_test.go#L36) |
| `RFC5392-4-3` | the ASBR MUST take precautions against excessive re- advertisements as described in [OSPF-TE]. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5392_opaque_originate_test.go#L143). **negative:** `unit/verify` [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5392_opaque_originate_test.go#L153) |
| `RFC5392-3.1.1-1` | The inter-AS TE link advertisement SHOULD be carried in a Type 10 Opaque LSA [RFC5250] if the flooding scope is to be limited to within the single IGP area to which the ASBR belongs (§3.1.1) -- Ze: `inter-as scope area`, the default, originates a Type 10 opaque LSA | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.1-2` | The choice between the use of a Type 10 (area-scoped) or Type 11 (AS-scoped) Opaque LSA is an AS-wide policy choice, and configuration control of it SHOULD be provided in ASBR implementations that support the advertisement of inter-AS TE links. (§3.1.1) -- Ze: the `inter-as scope { area \| as }` leaf selects Type 10 vs Type 11 per link | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-2` | For the Inter-AS-TE-v3-LSA, the S2 and S1 bits SHOULD be set to 01 to indicate that the flooding scope is to be limited to within the single IGP area to which the ASBR belongs (§3.1.2) | SHOULD | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-3` | The choice between the use of 01 or 10 is a network-wide policy choice, and configuration control SHOULD be provided in ASBR implementations that support the advertisement of inter-AS TE links. (§3.1.2) | SHOULD | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.2.1-3` | At least one of the IPv4-Remote-ASBR-ID sub-TLV and the IPv6-Remote-ASBR-ID sub-TLV SHOULD be included in the Link TLV of the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA. (§3.2.1) -- Ze: validateConfig requires at least one of `remote-asbr-ipv4` / `remote-asbr-ipv6` | SHOULD | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-4` | When TE is enabled on an inter-AS link and the link is up, the ASBR SHOULD advertise this link using the normal procedures for OSPF-TE [OSPF-TE]. (§4) -- Ze originates the Opaque-type-6 LSA via the standard opaque origination pass | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-5` | When either the link is down or TE is disabled on the link, the ASBR SHOULD withdraw the advertisement. (§4) -- Ze's pull-model origination emits a Withdraw for a removed inter-AS instance by MaxAge-flush | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-6` | When there are changes to the TE parameters for the link (for example, when the available bandwidth changes), the ASBR SHOULD re-advertise the link (§4) -- a changed body re-originates on the next self-LSA pass under the carrier's MinLSInterval rate-limit | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-7` | Routers or PCEs that are capable of processing advertisements of inter-AS TE links SHOULD NOT use such links to compute paths that exit an AS to a remote ASBR and then immediately re-enter the AS through another TE link. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-4-8` | Such paths would constitute extremely rare occurrences and SHOULD NOT be allowed except as the result of specific policy configurations at the router or PCE computing the path. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-5-1` | For example, if a different remote AS number is received in a BGP OPEN [BGP] from that locally configured into OSPF-TE, as we describe here, then local policy SHOULD be applied to determine whether to alert the operator to a potential misconfiguration or to suppress the OSPF advertisement of the inter-AS TE link. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-5-2` | if BGP is used to exchange TE information as described in Section 4.1, the inter-AS BGP session SHOULD be secured using mechanisms as described in [BGP] to provide authentication and integrity checks. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.2-3` | Use of the TE Router Address TE Router ID as specified in the Router Address TLV [OSPF-TE] is RECOMMENDED. (§3.3.2) | RECOMMENDED | 3.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.3-3` | Use of the TE Router IPv6 Address IPv6 TE Router ID as specified in the IPv6 Router Address, which is specified in the IPv6 Router Address TLV [OSPF-V3-TE], is RECOMMENDED. (§3.3.3) | RECOMMENDED | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-2.1-1` | TE aggregation is not supported or recommended (§2.1, §3) | NOT RECOMMENDED | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.1-3` | The inter-AS TE link advertisement SHOULD be carried in a Type 10 Opaque LSA [RFC5250] if the flooding scope is to be limited to within the single IGP area to which the ASBR belongs, or MAY be carried in a Type 11 Opaque LSA [RFC5250] if the information is intended to reach all routers (including area border routers, ASBRs, and PCEs) in the AS. (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.1.2-4` | For the Inter-AS-TE-v3-LSA, the S2 and S1 bits SHOULD be set to 01 to indicate that the flooding scope is to be limited to within the single IGP area to which the ASBR belongs, but MAY be set to 10 if the information should reach all routers (including area border routers, ASBRs, and PCEs) in the AS. (§3.1.2) | MAY | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5392-3.3.2-4` | An IPv4 Remote ASBR ID sub-TLV and IPv6 Remote ASBR ID sub-TLV MAY both be present in a Link TLV in OSPFv2 or OSPFv3. (§3.3.2) | MAY | 3.3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1) The U-bit is always set to 1 to indicate that an OSPFv3 router MUST flood the LSA at its defined flooding scope even if it does not recognize the LS type. (§3.1.2) | {gap}, no test | ze originates inter-AS TE only as the OSPFv2 Opaque-type-6 LSA; the OSPFv3 Inter-AS-TE-v3 LSA (function code 13) is not implemented, so there is no LS Type or U-bit to set (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2) the Neighbor ID sub-TLV [OSPF-V3-TE] MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1) | {gap}, no test | ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so there is no v3 inter-AS Link TLV in which a Neighbor ID sub-TLV could be emitted or prohibited (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.2.1-5`](#rfc5392-3.2.1-5) The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA. (§3.2.1) | {gap}, no test | the Inter-AS-TE-v3 clause only (the Inter-AS-TE-v2 clause is RFC5392-3.2.1-4, met): ze implements no OSPFv3 Inter-AS-TE-v3 LSA (function code 13), so there is no v3 inter-AS Link TLV in which to include the Remote-AS-Number sub-TLV (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1) In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV MUST be included if the neighboring ASBR has an IPv6 address. (§3.3.3) | {gap}, no test | OSPFv3 inter-AS TE (function code 13) is not implemented, so ze originates no OSPFv3 advertisement in which to require the IPv6 Remote ASBR ID (the type-24 codec exists but is only ever emitted into a v2 LSA) (internal/plugins/ospf/te.go:88-91) |
| [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2) If the neighboring ASBR does not have an IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead. (§3.3.3) | {gap}, no test | ze implements no OSPFv3 Inter-AS-TE-v3 LSA, so the v3 IPv4-fallback rule has no origination path to bind (internal/plugins/ospf/te.go:88-91) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5392-3.2.1-4`](#rfc5392-3.2.1-4)

The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA (§3.2.1) -- Ze: `remote-as` mandatory in YANG + validateConfig; emitted as sub-TLV 21 in every Inter-AS-TE-v2 Link TLV, and a received type-6 Link TLV without it is skipped; the Inter-AS-TE-v3 clause is RFC5392-3.2.1-5; spec-ospf-ext-2

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row now quotes the verbatim span ending at 'the Inter-AS-TE-v2 LSA' (R3 split; the v3 clause is RFC5392-3.2.1-5, verified against rfc/full/rfc5392.txt lines 528-529). The obligation binds the originator: + TestInterAsTEOriginateScopePolicy fails unless every originated Inter-AS-TE-v2 Link TLV carries sub-TLV 21 with the configured AS 65001 (record on buildInterASTELink observed red); - per owner ruling 2, the handling of a peer's Link TLV lacking it: TestTEReceiveType6MissingRemoteASSkipped encodes an otherwise well-formed type-6 Link TLV (Link Type, IPv4 Remote ASBR ID) without sub-TLV 21 and asserts no TED link is stored (validateReceivedTELink's !HasRemoteAS branch; record observed red). Buffer isolated: Link Type present and no Link ID, so only the missing sub-TLV 21 can trip.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTEReceiveType6MissingRemoteASSkipped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L250) | unit/verify | revert, verified |

### [`RFC5392-3.3.1-1`](#rfc5392-3.3.1-1)

When only two octets are used for the AS number, as in current deployments, the left (high- order) two octets MUST be set to zero. (§3.3.1) -- Ze encodes the 4-octet field big-endian from a uint32, so a 2-byte ASN is zero-extended

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a 2-octet AS number encoded with non-zero high-order octets in the 4-octet Remote AS Number field. Red on it: TestInterAsTERemoteAsTLV (packet/te_interas_test.go) encodes RemoteAS 65001 and asserts `body[8] != 0x00 || body[9] != 0x00` fatal on the wire bytes. Positive only, covered by the row's {single-polarity: positive} marker (uint32 big-endian encode cannot set the high octets for a 2-byte ASN).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTERemoteAsTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5392_te_interas_test.go#L29) | unit/verify | unproven |

### [`RFC5392-3.1.2-1`](#rfc5392-3.1.2-1)

The U-bit is always set to 1 to indicate that an OSPFv3 router MUST flood the LSA at its defined flooding scope even if it does not recognize the LS type. (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.1.2-1, so no unit is bound to it.

### [`RFC5392-3.2.1-1`](#rfc5392-3.2.1-1)

The Link ID sub-TLV [OSPF-TE] MUST NOT be used in the Link TLV of an Inter-AS-TE-v2 LSA (§3.2.1) -- Ze never emits sub-TLV 2 for an inter-AS link, and a received type-6 Link TLV carrying it is skipped by validateReceivedTELink

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Type-6 Inter-AS-TE-v2 Link TLV that carries the Link ID sub-TLV (type 2). Red on it: TestInterAsTEOriginateScopePolicy asserts `if l.HasLinkID { t.Fatalf }` on the Link TLV teOriginateType6 emits, for both area and AS scope. Negative polarity: TestTEReceiveMalformedNoEntry feeds a type-6 Link TLV that is otherwise valid (Link Type, Remote AS, IPv4 Remote ASBR ID) plus a Link ID and asserts no TED entry (validateReceivedTELink rejects l.HasLinkID for InterAsTEOpaqueType).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTEReceiveMalformedNoEntry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_test.go#L52) | unit/verify | unproven |
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L245) | unit/verify | unproven |

### [`RFC5392-3.2.1-2`](#rfc5392-3.2.1-2)

the Neighbor ID sub-TLV [OSPF-V3-TE] MUST NOT be used in the Link TLV of an Inter-AS-TE-v3 LSA (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.2.1-2, so no unit is bound to it.

### [`RFC5392-3.2.1-5`](#rfc5392-3.2.1-5)

The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA. (§3.2.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. The Inter-AS-TE-v3 clause of the §3.2.1 sentence: ze registers inter-AS TE only as the OSPFv2 Opaque type-6 consumer (registerTEConsumer, te.go) and implements no OSPFv3 Inter-AS-TE-v3 LSA (function code 13), so there is no v3 inter-AS Link TLV in which to include sub-TLV 21. Absent feature, recorded as a {gap}; the v2 clause is RFC5392-3.2.1-4. Disclosed on the Support remaining text (Five MUST gaps).

No test carries RFC5392-3.2.1-5, so no unit is bound to it.

### [`RFC5392-3.3.2-1`](#rfc5392-3.3.2-1)

In OSPFv2 advertisements, the IPv4 Remote ASBR ID sub-TLV MUST be included if the neighboring ASBR has an IPv4 address. (§3.3.2) -- Ze: `remote-asbr-ipv4` leaf emitted as sub-TLV 22; validateConfig requires at least one remote-asbr; spec-ospf-ext-2

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an OSPFv2 inter-AS advertisement that omits the IPv4 Remote ASBR ID sub-TLV (22) when the neighboring ASBR has an IPv4 address. Ze learns the remote ASBR's address only from config (remote-asbr-ipv4), so the observable case is a configured IPv4 remote ASBR ID. Red on it: TestInterAsTEOriginateScopePolicy configures remote-asbr-ipv4 203.0.113.9 and asserts `!l.HasRemoteASBRv4` fatal on the originated type-6 Link TLV. Positive only, covered by the row's {single-polarity: positive} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTEOriginateScopePolicy`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L252) | unit/verify | unproven |

### [`RFC5392-3.3.2-2`](#rfc5392-3.3.2-2)

If the neighboring ASBR does not have an IPv4 address (not even an IPv4 TE Router ID), the IPv6 Remote ASBR ID sub-TLV MUST be included instead. (§3.3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Origination now driven: v6-only inter-as config (remote-as 65001, remote-asbr-ipv6 2001:db8::9) through teOriginateType6/buildInterASTELink; originated body holds the literal sub-TLV 00 18 00 10 + address, decode shows v6 ID present, no v4 ID, Remote AS 65001. Judge overlay dropping HasRemoteASBRv6 in buildInterASTELink turns it red. {single-polarity: positive} on the row; record observed red on buildInterASTELink.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInterAsTEIPv6AsbrIdType24`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5392_te_interas_test.go#L91) | unit/verify | unproven |
| positive | [`TestRFC5392OriginatedV6OnlyLinkCarriesIPv6RemoteASBRID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_v6_remote_asbr_origination_test.go#L15) | unit/verify | revert, verified |

### [`RFC5392-3.3.3-1`](#rfc5392-3.3.3-1)

In OSPFv3 advertisements, the IPv6 Remote ASBR ID sub-TLV MUST be included if the neighboring ASBR has an IPv6 address. (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.3.3-1, so no unit is bound to it.

### [`RFC5392-3.3.3-2`](#rfc5392-3.3.3-2)

If the neighboring ASBR does not have an IPv6 address, the IPv4 Remote ASBR ID sub-TLV MUST be included instead. (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5392-3.3.3-2, so no unit is bound to it.

### [`RFC5392-4-1`](#rfc5392-4-1)

Hellos MUST NOT be exchanged over the inter-AS link (§4) -- an interface carrying an `inter-as` block is forced passive at config parse by parseInterface in internal/plugins/ospf/config.go, so no Hello is sent on it

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by QF-2 after the row's trailing implementation note lost its parenthesis, so the quote ends at the §4 sub-span 'Hellos MUST NOT be exchanged over the inter-AS link'. Forbidden: an inter-AS interface that runs Hellos. TestRFC5392InterASInterfaceIsPassive asserts `!ic.Passive || active` fails the test when parseInterface leaves an inter-as interface active, with an intra-AS TE control. Enforced stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5392InterASInterfaceIsPassive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_interas_passive_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L278) | unit/verify | unproven |

### [`RFC5392-4-2`](#rfc5392-4-2)

Hellos MUST NOT be exchanged over the inter-AS link, and consequently, an OSPF adjacency MUST NOT be formed. (§4) -- the forced-passive interface is not an active interface, so no neighbor FSM runs and no adjacency forms on it

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-27 by DF-OSPF-C (author of the fix; independent /ze-rfc-audit owed). Same producer as RFC5392-4-1: the forced-passive inter-AS interface is not in activeInterfaces, so no neighbor FSM runs and no adjacency forms. The negative TestRFC5392InterASInterfaceIsPassive now carries the rule; the positive TestInterASTEOriginatesWithoutNeighbor only shows origination needs no neighbor. REV-OSPF independent re-audit 2026-09-27: kept enforced on the same producer and assertion as RFC5392-4-1: no socket is opened on a passive interface, so no neighbor FSM and no adjacency.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5392InterASInterfaceIsPassive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_interas_passive_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestInterASTEOriginatesWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5392_te_originate_test.go#L281) | unit/verify | unproven |

### [`RFC5392-4-3`](#rfc5392-4-3)

the ASBR MUST take precautions against excessive re- advertisements as described in [OSPF-TE]. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: re-advertising a changed inter-AS TE LSA without any precaution against excessive re-advertisement. Red on it: TestInterASTEReAdvertiseRateLimited (internal/plugins/ospf/lsdb/opaque_originate_test.go) originates a Type-6 LSA, changes its body within MinLSInterval and asserts `ok || h2.Sequence != h1.Sequence` fatal plus no extra LS Update send. Negative polarity: after MinLSInterval the changed body is re-advertised with the next sequence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5392_opaque_originate_test.go#L153) | unit/verify | unproven |
| positive | [`TestInterASTEReAdvertiseRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc5392_opaque_originate_test.go#L143) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5392.txt |
| Source fingerprint | 0833ca775f7d650c |
| Record | rfc/extraction/rfc5392.json |
| Mapped sentences | 10 |
| Declined as scope | 10 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `2.3` | not stated | 4 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 2 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.3.1` | not stated | 2 | walked | not stated |
| `3.3.2` | not stated | 2 | walked | not stated |
| `3.3.3` | not stated | 2 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 0 | walked | not stated |
| `6.1.2` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2.2 walks a worked example of a path computation across AS1/AS2/AS3; the lowercase 'must' describes what router R5 in the example has to work out, not an obligation on an implementation. | The next hop in the ERO shows AS3, and R5 must determine a path segment across AS2 to reach AS3. |
| `2.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive sentence about GMPLS networks ('further information may also be required'), lowercase and referring the reader to [GMPLS-TE]. | In GMPLS networks, further information may also be required to select the correct TE links as defined in [GMPLS-TE]. |
| `2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Definition of what a BRPC tree of paths consists of; a description of the PCE algorithm, not an obligation. | Each tree consists of the set of paths from all Boundary Nodes located in domain(i) to the destination where each path satisfies the set of required constraints for the TE LSP (bandwidth, affinities, etc.). |
| `2.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Motivational prose stating what a PCE needs to know to correlate trees; lowercase 'must know', describing the information requirement this document exists to satisfy. | In order that the tree of paths provided by one PCE to its neighbor can be correlated, the identities of the ASBRs for each path need to be referenced, so the PCE must know the identities of the ASBRs in the remote AS reached by any inter-AS TE link, and, in order that it provides only suitable paths in the tree, the PCE must know the TE properties of the inter-AS TE links. |
| `2.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Same BRPC motivation paragraph: lowercase 'must determine' / 'must know' describing the PCE's input needs, not a normative rule. | But, to provide suitable path segments, PCE3 must determine which entry boundary nodes provide connectivity to its upstream neighbor AS (identified by its AS number), and must know the TE properties of the inter-AS TE links. |
| `2.3:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Concluding sentence of the BRPC motivation ('the same information listed in Section 2.2 is required'); it states no obligation of its own. | Thus, to support Backward Recursive Path Computation the same information listed in Section 2.2 is required. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Rationale for carrying all TE information in this mechanism rather than depending on other protocols; 'needed'/'required' are descriptive. | While some of the TE information of an inter-AS TE link may be available within the AS from other protocols, in order to avoid any dependency on where such protocols are processed, this mechanism carries all the information needed for the required TE operations. |
| `3.3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.3.1 restates as REQUIRED the Remote-AS-Number sub-TLV inclusion that Section 3.2.1 states as MUST; RFC5392-3.2.1-4 cites both sections. | The Remote AS Number sub-TLV is REQUIRED in a Link TLV that advertises an inter-AS TE link. |
| `4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explains why two LSAs per TE link exist in normal OSPF-TE ('This enables CSPF to do a two-way check'); it is the rationale for the proxy advertisement, not a rule. | This enables Constrained Shortest Path First (CSPF) to do a two-way check on the link when performing path computation and eliminate it from consideration unless both directions of the link satisfy the required constraints. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 is an observation that BGP can supply some of the configuration ('it is possible, and may be operationally advantageous'); no keyword and no obligation. | We note further that it is possible, and may be operationally advantageous, to obtain some of the required configuration information from BGP. |

## Superseded

No document obsoletes RFC 5392, so its obligations are stated where they were written.
