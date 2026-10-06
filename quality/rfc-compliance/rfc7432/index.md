# RFC 7432 - BGP MPLS-Based Ethernet VPN

Partial. Every requirement this repository extracted from RFC 7432, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9.8% | 10 of 102 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1.0% | 1 of 102 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 102 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 102 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 87.5% | 28 of 32 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 102 | of 119 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 76 | of 102 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 72.5% | 74 of 102 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 102 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 2.0% | 2 of 102 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 14.7% | 15 of 102 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 102 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 119 |
| Gated MUST-level | 102 |
| Not applicable, so out of scope | 74 |
| Declared gaps | 15 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 32 |
| Recorded audit verdicts | 10 |
| Discrimination records | 28 |
| Summary | `rfc/short/rfc7432.md` |
| Requirement shard | `rfc/requirements/rfc7432.md` |
| RFC text | `rfc/full/rfc7432.txt` |

## Enrolment

Enrolled: BGP MPLS-Based Ethernet VPN (EVPN)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- EVPN NLRI family (AFI 25 / SAFI 70), route-type codecs, ADD-PATH framing, MP_REACH construction and explicit configured control-plane origination through `evpn.parseConfigRoute`. Configured extended communities, including ESI Label communities, reach the route attributes
- `evpn.EncodeRoute` also carries them through `message.BuildEVPN`. The shared `message.ValidateEVPNOrigination` guard recognizes MAX-ET Type 1 routes as per-ES and requires a Type 1 RD, zero NLRI label, ESI Label community and route target. It also requires a Type 1 RD and ES-Import Route Target for locally originated Ethernet Segment routes. These source paths await current integration verification
- permissive receive decoding is not evidence of sender compliance, and route-support evidence does not establish every per-ES obligation.


**What the ledger says remains**

Separate implementation and proof gaps remain in the requirement checklist.

- **No full EVPN PE or multihoming claim:** MAC-VRF/EVI ownership, automatic segment discovery, DF election, MAC learning, EVPN forwarding, split horizon, aliasing and BUM replication remain outside the implemented control-plane boundary. The explicit per-ES producer has no independent intent selector forcing MAX-ET, no validation of ESI Label flags and embedded label against an independent redundancy-mode setting, and no cross-route equal-ESI label consistency check. ESI Label presence is guarded but its row-specific enforcement proof remains unverified; carrying configured bytes is not autonomous label allocation or redundancy operation. Historical checklist counts and source citations are not current execution evidence. The new Type 4 origination guard and its behavioral carrier require Main's verification. Route targets are never auto-derived from the Ethernet Tag ID (Section 7.10.1).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 10 | one part of the gated population |
| Annotated (including scoped evidence) | 92 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **102** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (10):** [`RFC7432-5-1`](#rfc7432-5-1), [`RFC7432-8.2.1-2`](#rfc7432-8.2.1-2), [`RFC7432-8.1.1-1`](#rfc7432-8.1.1-1), [`RFC7432-8.1.1-2`](#rfc7432-8.1.1-2), [`RFC7432-9.2.1-1`](#rfc7432-9.2.1-1), [`RFC7432-10-1`](#rfc7432-10-1), [`RFC7432-8.2.1-10`](#rfc7432-8.2.1-10), [`RFC7432-8.3-2`](#rfc7432-8.3-2), [`RFC7432-9.2.1-6`](#rfc7432-9.2.1-6), [`RFC7432-11.1-3`](#rfc7432-11.1-3)

**Annotated (including scoped evidence) (92):** [`RFC7432-7.9-1`](#rfc7432-7.9-1), [`RFC7432-7.9-2`](#rfc7432-7.9-2), [`RFC7432-7.9-3`](#rfc7432-7.9-3), [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1), [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3), [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4), [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5), [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6), [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7), [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8), [`RFC7432-7.6-1`](#rfc7432-7.6-1), [`RFC7432-7.6-2`](#rfc7432-7.6-2), [`RFC7432-11-1`](#rfc7432-11-1), [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2), [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3), [`RFC7432-9.1-1`](#rfc7432-9.1-1), [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4), [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1), [`RFC7432-10-2`](#rfc7432-10-2), [`RFC7432-10-3`](#rfc7432-10-3), [`RFC7432-10-4`](#rfc7432-10-4), [`RFC7432-11.1-1`](#rfc7432-11.1-1), [`RFC7432-11.1-2`](#rfc7432-11.1-2), [`RFC7432-11.2-1`](#rfc7432-11.2-1), [`RFC7432-11.2-2`](#rfc7432-11.2-2), [`RFC7432-11.2-3`](#rfc7432-11.2-3), [`RFC7432-6.1-1`](#rfc7432-6.1-1), [`RFC7432-6.1-2`](#rfc7432-6.1-2), [`RFC7432-6.2-1`](#rfc7432-6.2-1), [`RFC7432-6.3-1`](#rfc7432-6.3-1), [`RFC7432-6.3-2`](#rfc7432-6.3-2), [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1), [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1), [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1), [`RFC7432-15-1`](#rfc7432-15-1), [`RFC7432-15-2`](#rfc7432-15-2), [`RFC7432-15.1-1`](#rfc7432-15.1-1), [`RFC7432-15.1-2`](#rfc7432-15.1-2), [`RFC7432-15.2-1`](#rfc7432-15.2-1), [`RFC7432-10.1-1`](#rfc7432-10.1-1), [`RFC7432-17.3-1`](#rfc7432-17.3-1), [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1), [`RFC7432-8.4-1`](#rfc7432-8.4-1), [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2), [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3), [`RFC7432-5-3`](#rfc7432-5-3), [`RFC7432-5-4`](#rfc7432-5-4), [`RFC7432-5-5`](#rfc7432-5-5), [`RFC7432-5-6`](#rfc7432-5-6), [`RFC7432-5-7`](#rfc7432-5-7), [`RFC7432-5-8`](#rfc7432-5-8), [`RFC7432-5-9`](#rfc7432-5-9), [`RFC7432-5-10`](#rfc7432-5-10), [`RFC7432-5-11`](#rfc7432-5-11), [`RFC7432-5-12`](#rfc7432-5-12), [`RFC7432-5-13`](#rfc7432-5-13), [`RFC7432-12-1`](#rfc7432-12-1), [`RFC7432-12.1-1`](#rfc7432-12.1-1), [`RFC7432-12.1-2`](#rfc7432-12.1-2), [`RFC7432-12.2-1`](#rfc7432-12.2-1), [`RFC7432-13.1-1`](#rfc7432-13.1-1), [`RFC7432-13.1-2`](#rfc7432-13.1-2), [`RFC7432-13.1-3`](#rfc7432-13.1-3), [`RFC7432-13.1-4`](#rfc7432-13.1-4), [`RFC7432-13.1-5`](#rfc7432-13.1-5), [`RFC7432-14.1-1`](#rfc7432-14.1-1), [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2), [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3), [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1), [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2), [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1), [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2), [`RFC7432-10.1-4`](#rfc7432-10.1-4), [`RFC7432-6.2-2`](#rfc7432-6.2-2), [`RFC7432-6.3-3`](#rfc7432-6.3-3), [`RFC7432-6.3-4`](#rfc7432-6.3-4), [`RFC7432-7.10.1-1`](#rfc7432-7.10.1-1), [`RFC7432-7.10.1-2`](#rfc7432-7.10.1-2), [`RFC7432-8.2.1-9`](#rfc7432-8.2.1-9), [`RFC7432-8.2.1.1-1`](#rfc7432-8.2.1.1-1), [`RFC7432-8.3-1`](#rfc7432-8.3-1), [`RFC7432-8.3.1.1-6`](#rfc7432-8.3.1.1-6), [`RFC7432-8.3.1.1-7`](#rfc7432-8.3.1.1-7), [`RFC7432-8.3.1.2-2`](#rfc7432-8.3.1.2-2), [`RFC7432-8.5-2`](#rfc7432-8.5-2), [`RFC7432-10.1-5`](#rfc7432-10.1-5), [`RFC7432-11.2-5`](#rfc7432-11.2-5), [`RFC7432-11.2-6`](#rfc7432-11.2-6), [`RFC7432-11.2-7`](#rfc7432-11.2-7), [`RFC7432-13.1-6`](#rfc7432-13.1-6), [`RFC7432-13.1-7`](#rfc7432-13.1-7), [`RFC7432-17.3-3`](#rfc7432-17.3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7432-7.9-1` | The Route Distinguisher (RD) MUST be set to the RD of the MAC-VRF that is advertising the NLRI. (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RD of an EVPN NLRI is whatever the route command supplies -- l2vpnRouteToEVPNParams parses it with ParseRDString and hands it straight to the NLRI constructors (internal/component/bgp/plugins/nlri/evpn/encode.go:71-78) -- and ze holds no MAC-VRF whose RD the advertisement could be required to match |
| `RFC7432-7.9-2` | An RD MUST be assigned for a given MAC-VRF on a PE (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseL2VPNArgs refuses a route with an empty route-distinguisher (internal/component/bgp/plugins/nlri/evpn/encode.go:240-242), so every advertised EVPN NLRI does carry an RD, but the unit of assignment is the configured route and ze has no MAC-VRF construct nor any per-MAC-VRF RD allocation |
| `RFC7432-7.9-3` | RD MUST be unique across all MAC-VRFs on a PE (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** each route command's RD is parsed on its own (internal/component/bgp/plugins/nlri/evpn/encode.go:73-77) and no code compares it against the RDs already in use, so nothing enforces uniqueness across instances |
| `RFC7432-5-1` | The Ethernet Segment Identifier (ESI) MUST be set to the 10-octet value described in Section 5. (§8.1.1) | MUST | 8.1.1 | **positive:** `unit/verify` [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L403). **negative:** `unit/verify` [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L443) |
| `RFC7432-8.2.1-1` | The Ethernet Tag ID MUST be set to MAX-ET. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams passes the operator-supplied EthernetTag to NewEVPNType1. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination recognizes MAX-ET and validates the per-ES branch, but these producers have no independent per-ES intent selector that forces MAX-ET. A configured MAX-ET example establishes support, not independent enforcement of tag selection |
| `RFC7432-8.2.1-2` | The MPLS label in the NLRI MUST be set to 0. (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L35). **positive:** `unit/verify` [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L14). **negative:** `unit/verify` [`TestConfiguredEVPNRefusesInvalidPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L54). **negative:** `unit/verify` [`TestOriginEthernetADPerESRefusesLabelStack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L69) |
| `RFC7432-8.2.1-3` | The ESI Label extended community MUST be included in the route. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** row-specific enforcement proof remains unverified, not an absent producer. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute merges configured extended communities, calls ValidateEVPNOrigination and attaches the bytes to PluginRoute; internal/component/bgp/plugins/nlri/evpn/encode.go::EncodeRoute carries ExtCommunityBytes through l2vpnRouteToEVPNParams to BuildEVPN. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination rejects MAX-ET Type 1 origination without an ESI Label community (type 0x06/subtype 0x01). This implemented presence guard requires its own proof and verdict; the route-support verdict does not promote this row |
| `RFC7432-8.2.1-4` | If All-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 0 and the MPLS label in that Extended Community MUST be set to a valid MPLS label value. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** configured ESI Label bytes are carried by internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks community presence, not the Single-Active bit or embedded MPLS label against an independent All-Active mode setting. That conditional field validation is unimplemented. Autonomous multihoming remains outside the selected control-plane role; its absence does not establish conformance of configured signaling |
| `RFC7432-8.2.1-5` | The MPLS label in this Extended Community is referred to as the ESI label and MUST have the same value in each Ethernet A-D per ES route advertised for the ES. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute carries each route's supplied ESI Label bytes. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks community presence without comparing embedded labels across separately originated routes with the same ESI. The cross-route consistency invariant is unimplemented; one conforming per-ES example does not prove it |
| `RFC7432-8.2.1-6` | If the advertising PE is using P2MP MPLS LSPs for sending multicast, broadcast, or unknown unicast traffic, then this label MUST be an upstream assigned MPLS label. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the condition is a PE sending BUM traffic over P2MP MPLS LSPs. Ze's selected BGP control-plane role, documented in docs/architecture/wire/nlri-evpn.md, does not forward Ethernet/BUM traffic or manage its P2MP trees. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry explicitly supplied ESI Label bytes, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks their presence; these producers neither establish P2MP LSPs nor allocate upstream labels. Carrying the community does not fill the conditional forwarding role |
| `RFC7432-8.2.1-7` | If Single-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 1 (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry configured ESI Label bytes. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks presence, not the Single-Active bit against an independent Single-Active mode setting. That conditional flag validation is unimplemented. Ze does not autonomously select or operate a redundancy mode in its documented BGP control-plane role; this role boundary is not proof that explicitly configured signaling satisfies the condition |
| `RFC7432-8.2.1-8` | Each Ethernet A-D per ES route MUST carry one or more Route Target (RT) attributes. (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** row-specific enforcement proof remains unverified, not an absent producer or guard. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute merges and attaches configured extended communities; encode.go::l2vpnRouteToEVPNParams supplies ExtCommunityBytes to BuildEVPN. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination rejects MAX-ET Type 1 origination without a Route Target (type 0x00, 0x01 or 0x02/subtype 0x02). This presence guard requires its own proof and verdict; it does not auto-derive RTs or guarantee the entire ES-to-EVI RT set required by RFC7432-8.2.1.1-1 |
| `RFC7432-8.1.1-1` | The Route Distinguisher (RD) MUST be a Type 1 RD [RFC4364]. (§8.1.1). `message.ValidateEVPNOrigination` checks locally originated Type 4 NLRIs; current verification remains required. The receive codec remains permissive because this is an origination requirement. | MUST | 8.1.1 | **positive:** `unit/verify` [`TestRFC7432EthernetSegmentOriginationAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L54). **negative:** `unit/verify` [`TestRFC7432EthernetSegmentOriginationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L67) |
| `RFC7432-8.1.1-2` | The BGP advertisement that advertises the Ethernet Segment route MUST also carry an ES-Import Route Target, as defined in Section 7.6. (§8.1.1). `message.ValidateEVPNOrigination` requires extended community type 0x06/subtype 0x02 for local Type 4 origination; an ESI Label community is not a substitute. Current verification remains required. | MUST | 8.1.1 | **positive:** `unit/verify` [`TestRFC7432EthernetSegmentOriginationAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L55). **positive:** `unit/verify` [`TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_es_import_test.go#L21). **negative:** `unit/verify` [`TestRFC7432EthernetSegmentOriginationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L73). **negative:** `unit/verify` [`TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_es_import_test.go#L23) |
| `RFC7432-7.6-1` | The Ethernet Segment route filtering MUST be done such that the Ethernet Segment route is imported only by the PEs that are multihomed to the same Ethernet segment. (§8.1.1) | MUST | 8.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze's selected boundary is explicit control-plane origination, not the EVPN PE segment-discovery role. Configured ES-Import bytes can be advertised, but Ze has no Ethernet-segment membership or automatically constructed segment import rules; the origin guard does not establish this distribution behavior |
| `RFC7432-7.6-2` | A BGP speaker that implements RT Constraint [RFC4684] MUST apply the RT Constraint procedures to the ES-Import RT as well. (§7.6) | MUST | 7.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no RT Constraint procedures: the rtc plugin is an NLRI codec alone (internal/component/bgp/plugins/nlri/rtc/types.go:97 parses, :186 writes) and grep -rni 'rtc' over internal/component/bgp/reactor, internal/component/bgp/rib and internal/component/bgp/plugins/rib matches only two comments naming RTC as a non-CIDR family, so no route-target-based filtering exists to extend to an ES-Import RT |
| `RFC7432-11-1` | Each PE MUST advertise an "Inclusive Multicast Ethernet Tag route" to enable the above. (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVI or MAC-VRF: grep -rn '\\bEVI\\b' --include=*.go over internal/component/bgp matches nothing and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing, so no per-instance origination duty is expressed anywhere; the Inclusive Multicast codec advertises only what an operator asks for |
| `RFC7432-9.2.1-1` | The encoding of a MAC address MUST be the 6-octet MAC address specified by [802.1Q] and [802.1D-REV]. (§9.2.1) | MUST | 9.2.1 | **positive:** `unit/verify` [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L296). **negative:** `unit/verify` [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L318) |
| `RFC7432-9.2.1-2` | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the IPv4 or IPv6 address of the advertising PE. (§9.2.1, §11.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the earlier zero-address fallback claim is obsolete. internal/component/bgp/message/update_build_evpn.go::BuildEVPN rejects missing, unspecified and multicast next hops before buildMPReachEVPN writes the supplied address. Native configuration retains next-hop self and internal/component/bgp/reactor/peer_static_routes.go::toPluginParams resolves it from the connected local address. These guards do not establish that every explicitly supplied next hop belongs to the advertising PE. Whole-row enforcement remains unverified |
| `RFC7432-9.2.1-3` | MPLS Label1 MUST be downstream assigned (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Label1 is the number typed into the route command (internal/component/bgp/plugins/nlri/evpn/encode.go:101-107); ze allocates no local label space for EVPN, so nothing makes the advertised label downstream assigned |
| `RFC7432-9.1-1` | The PEs in a particular EVPN instance MUST support local data-plane learning using standard IEEE Ethernet learning procedures. (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-9.2.1-4` | The BGP advertisement for the MAC/IP Advertisement route MUST also carry one or more Route Target (RT) attributes. (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams and config.go::parseConfigRoute carry configured extended communities. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination has no Type 2 Route Target presence check, so explicit MAC/IP origination without a Route Target is not refused. The missing guard, not an inability to carry configured Route Targets, is the gap |
| `RFC7432-8.4.1-1` | The Ethernet A-D route MUST carry one or more Route Target (RT) attributes, per Section 7.10. (§8.4.1) | MUST | 8.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams and config.go::parseConfigRoute carry configured extended communities. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination requires a Route Target for Type 1 only when its Ethernet Tag is MAX-ET. Explicit per-EVI origination without a Route Target is not refused. The per-ES support proof does not establish this separate per-EVI obligation |
| `RFC7432-10-1` | For ARP and ND purposes, the IP Address Length field MUST be set to 32 for an IPv4 address or 128 for an IPv6 address. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L355). **positive:** `unit/verify` [`TestRFC7432MACIPSenderUsesBitLengths`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_ip_length_send_test.go#L13). **negative:** `unit/verify` [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L377) |
| `RFC7432-10-2` | If there are multiple IP addresses associated with a MAC address, then multiple MAC/IP Advertisement routes MUST be generated, one for each IP address. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no IP-to-MAC binding table to enumerate: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-10-3` | When the IP address is dissociated with the MAC address, then the MAC/IP Advertisement route with that particular IP address MUST be withdrawn. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no IP-to-MAC binding whose removal could drive a withdrawal: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-10-4` | If the receiving PE receives both the MAC-only route and the MAC/IP route, then when it receives a withdraw message for the MAC/IP route, it MUST delete the corresponding entry from the ARP table but not the MAC entry from the MAC-VRF table, unless it receives a withdraw message for the MAC-only route. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no ARP or ND cache to delete an entry from: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-11.1-1` | The Originating Router's IP Address field value MUST be set to an IP address of the PE that should be common for all the EVIs on the PE (e.g., this address may be the PE's loopback address). (§11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams uses the route's explicit IP as its originator, or its explicit next hop when IP is absent, and refuses an absent originating address. It does not establish PE ownership or a common address across EVIs. The configured IMET originator can differ from the session next hop |
| `RFC7432-11.1-2` | The assignment of RTs as described in Section 7.10 MUST be followed. (§11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** row-specific enforcement proof remains unverified. Section 7.10 permits configured RTs or automatic derivation. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry configured RTs, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination requires at least one on IMET. The earlier claim that configured communities could not reach this producer is obsolete. Automatic derivation is not selected, as documented in docs/architecture/wire/nlri-evpn.md |
| `RFC7432-11.2-1` | In order to identify the P-tunnel used for sending broadcast, unknown unicast, or multicast traffic, the Inclusive Multicast Ethernet Tag route MUST carry a Provider Multicast Service Interface (PMSI) Tunnel attribute as specified in [RFC6514]. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-11.2-2` | + The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-11.2-3` | The PMSI Tunnel attribute MUST carry a downstream assigned MPLS label. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-6.1-1` | a VID translation MUST be supported in the data path and MUST be performed on the disposition PE. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and no data path performs VID translation |
| `RFC7432-6.1-2` | The Ethernet Tag ID in all EVPN routes MUST be set to 0. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189) |
| `RFC7432-6.2-1` | This implies that MAC addresses MUST be unique across all VLANs for that EVI in order for this service to work. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no bundle of VLANs shares a MAC learning space |
| `RFC7432-6.3-1` | In the case where a single VLAN is represented by a single VID and thus no VID translation is required, an MPLS-encapsulated packet MUST carry that VID. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.3-2` | In the case where a single VLAN is represented by different VIDs on different CEs and thus VID translation is required, a normalized Ethernet Tag ID (VID) MUST be carried in the EVPN BGP routes. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no VID normalization happens before origination |
| `RFC7432-8.3.1.1-1` | - A non-DF ingress PE MUST include the ESI label distributed by the DF egress PE in the copy of a BUM packet sent to it. (§8.3.1.1) | MUST | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.3.1.2-1` | Penultimate hop popping MUST be disabled on the P2MP LSPs used in the MPLS transport infrastructure for EVPN. (§8.3.1.2) | MUST | 8.3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no P2MP LSP and controls no penultimate hop popping: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-8.3.1-1` | This label MUST be programmed in the platform label space by the advertising PE, and the forwarding entry for this label must result in NOT forwarding packets received with this label onto the Ethernet segment for which the label was distributed. (§8.3.1.1) | MUST | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-15-1` | In order to process mobility events correctly, an implementation MUST handle scenarios in which sequence number wraparound occurs. (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing |
| `RFC7432-15-2` | In order to allow all of the PEs in the EVPN instance to correctly determine the current location of the MAC address, all advertisements of it being reachable via the previous Ethernet segment MUST be withdrawn by the PEs, for the previous Ethernet segment, that had advertised it. (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no MAC move is observed and nothing is withdrawn |
| `RFC7432-15.1-1` | The PE MUST alert the operator and stop sending and processing any BGP MAC/IP Advertisement routes for that MAC address until a corrective action is taken by the operator. (§15.1) | MUST | 15.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no duplicate-MAC counter or operator alert exists |
| `RFC7432-15.1-2` | The values of M and N MUST be configurable to allow for flexibility in operator control. (§15.1) | MUST | 15.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the M and N duplicate-detection parameters have nothing to configure |
| `RFC7432-15.2-1` | If a PE receives such advertisements and later learns the same MAC address(es) via local learning, then the PE MUST alert the operator. (§15.2) | MUST | 15.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the sticky flag is never read and local learning never happens |
| `RFC7432-10.1-1` | This is accomplished by requiring the route to carry the Default Gateway extended community defined in Section 7.8 (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Default Gateway extended community: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing, and no default-gateway route origination exists |
| `RFC7432-17.3-1` | If the connectivity between the multihomed CE and one of the PEs to which it is attached fails, the PE MUST withdraw the set of Ethernet A-D per ES routes that had been previously advertised for that ES. (§17.3) | MUST | 17.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no PE-to-CE attachment circuit: grep -rni 'attachment circuit\|pe.to.ce' --include=*.go over internal/ matches nothing, so no link event drives an EVPN withdrawal |
| `RFC7432-14.1.1-1` | In this case, the set of Ethernet A-D per ES routes advertised by each PE MUST have the "Single-Active" bit in the flags of the ESI Label extended community set to 1. (§8.5) | MUST | 8.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.4-1` | Therefore, in order to handle corner cases and race conditions, the Ethernet A-D per EVI route MUST NOT be used for traffic forwarding by a remote PE until it also receives the associated set of Ethernet A-D per ES routes. (§8.4) | MUST NOT | 8.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no route ever becomes a forwarding entry to withhold |
| `RFC7432-8.3.1.1-2` | In both All-Active and Single-Active redundancy mode, an ingress PE MUST NOT include an ESI label in the copy of a BUM packet sent to an egress PE that is not attached to the ES through which the BUM packet entered the EVI. (§8.3.1.1) | MUST NOT | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.3.1.1-3` | If the next label is the ESI label assigned by PE2 for ES1, then PE2 MUST NOT forward the packet onto ES1. (§8.3.1.1, §8.3.1.2) | MUST NOT | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code |
| `RFC7432-5-2` | Ethernet segment SHOULD have a non-reserved ESI (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-7.9-4` | It is RECOMMENDED to use the Type 1 RD [RFC4364]. (§7.9) | SHOULD | 7.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.4-2` | A remote PE that receives a MAC/IP Advertisement route with a non-reserved ESI SHOULD consider the advertised MAC address to be reachable via all PEs that have advertised reachability to that MAC address's EVI/ES via the combination of an Ethernet A-D per EVI route for that EVI/ES (and Ethernet tag, if applicable) AND Ethernet A-D per ES routes for that ES with the "Single-Active" bit in the flags of the ESI Label extended community set to 0. (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10-5` | When a PE receives an ARP Request for an IP address from a CE, and if the PE has the MAC address binding for that IP address, the PE SHOULD perform ARP proxy by responding to the ARP Request. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.5-1` | In the case of Single-Active multihoming, when a service moves from one PE in the redundancy group to another PE as a result of re-carving, the PE, which ends up being the elected DF for the service, SHOULD trigger a MAC address flush notification towards the associated Ethernet segment. (§8.5) | SHOULD | 8.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1-2` | The ESI label SHOULD be distributed by all PEs when operating in Single-Active redundancy mode using a set of Ethernet A-D per ES routes. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1.1-4` | An ingress PE (DF or non-DF) SHOULD include the ESI label distributed by each non-DF egress PE in the copy of a BUM packet sent to it. (§8.3.1.1) | SHOULD | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.6-1` | some of the multihoming procedures described above SHOULD be supported even by single- homing PEs (§8.6) | SHOULD | 8.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10.1-2` | if there is a discrepancy, then the PE SHOULD notify the operator and log an error message. (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-9.1-2` | Alternatively, PEs MAY learn the MAC addresses of the CEs in the control plane or via management-plane integration between the PEs and the CEs. (§9.1) | MAY | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-11.2-4` | A PE that uses a P-multicast tree for the P-tunnel MAY aggregate two or more EVPN instances (EVIs) present on the PE onto the same tree. (§11.2) | MAY | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10.1-3` | Each PE that acts as a default gateway for a given EVPN instance MAY advertise in the EVPN control plane its default gateway MAC address using the MAC/IP Advertisement route, and each such PE indicates that such a route is associated with the default gateway. (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-5-3` | The CE LACP System MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-4` | The CE LACP port key MUST be encoded in the 2 octets next to the System MAC address. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-5` | The Root Bridge MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-6` | The CE Root Bridge Priority MUST be encoded in the 2 octets next to the Root Bridge MAC address. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-7` | The PE MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-8` | The Local Discriminator value MUST be encoded in the low-order 3 octets of the ESI Value. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-9` | The system router ID MUST be encoded in the high-order 4 octets of the ESI Value field. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-10` | The Local Discriminator value MUST be encoded in the 4 octets next to the IP address. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-11` | This is an AS number owned by the system and MUST be encoded in the high-order 4 octets of the ESI Value field. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-12` | The Local Discriminator value MUST be encoded in the 4 octets next to the AS number. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-13` | If the CE(s) constituting an Ethernet segment is (are) managed by the network operator, then ESI uniqueness should be guaranteed; however, if the CE(s) is (are) not managed, then the operator MUST configure a network-wide unique ESI for that Ethernet segment. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no multihomed Ethernet segment and no managed CE: ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code; the ESI is operator-provided text parsed by ParseESIString (internal/component/bgp/plugins/nlri/evpn/types.go:119) |
| `RFC7432-12-1` | PEx MUST NOT send the frame to other PEs, since PEy would have already done so. (§12) | MUST NOT | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-12.1-1` | The PE that receives a packet with this particular MPLS label MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received label is classified as BUM |
| `RFC7432-12.1-2` | Further, if the MAC address is a unicast MAC address, the PE MUST treat the packet as an unknown unicast packet. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received frame is classified as unknown unicast |
| `RFC7432-12.2-1` | The PE that receives a packet on the P2MP LSP specified in the PMSI Tunnel attribute MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.2) | MUST | 12.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names, and ze receives no P2MP LSP traffic |
| `RFC7432-13.1-1` | The PE MUST flood the packet to other PEs. The PE MUST first encapsulate the packet in the ESI MPLS label as described in Section 8.3. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-2` | If ingress replication is used, the packet MUST be replicated to each remote PE, with the VPN label being an MPLS label determined as follows: This is the MPLS label advertised by the remote PE in a PMSI Tunnel attribute in the Inclusive Multicast Ethernet Tag route for a <MAC-VRF> or <MAC-VRF, Ethernet tag> combination. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-3` | If P2MP LSPs are being used, the packet MUST be sent on the P2MP LSP of which the PE is the root, for the Ethernet tag in the EVPN instance. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-4` | If the same P2MP LSP is used for all Ethernet tags, then all the PEs in the EVPN instance MUST be the leaves of the P2MP LSP. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-5` | If the MAC address is unknown, then, if the administrative policy on the PE does not allow flooding of unknown unicast traffic: - the PE MUST drop the packet. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no packet is dropped for an unknown MAC |
| `RFC7432-14.1-1` | Whenever a remote PE imports a MAC/IP Advertisement route for a given <ESI, Ethernet tag> in a MAC-VRF, it MUST examine all imported Ethernet A-D routes for that ESI in order to determine the load- balancing characteristics of the Ethernet segment. (§14.1) | MUST | 14.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so importing a MAC/IP route triggers no correlation with Ethernet A-D routes |
| `RFC7432-14.1.1-2` | For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from at least one PE, where the "Single-Active" flag in the ESI Label extended community is set, then the remote PE MUST deduce that the ES is operating in Single-Active redundancy mode. (§14.1.1) | MUST | 14.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| `RFC7432-14.1.1-3` | If there is more than one backup PE for a given ES, the remote PE MUST use the primary PE's withdrawal of its set of Ethernet A-D per ES routes as a trigger to start flooding traffic for the associated MAC addresses (as long as flooding of unknown unicast packets is administratively allowed), as it is not possible to select a single backup PE. (§14.1.1) | MUST | 14.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no primary or backup PE state and no flooding decision exists |
| `RFC7432-14.1.2-1` | For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from one or more PEs and none of them have the "Single-Active" flag in the ESI Label extended community set, then the remote PE MUST deduce that the ES is operating in All-Active redundancy mode. (§14.1.2) | MUST | 14.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| `RFC7432-14.1.2-2` | The remote PE MUST use received MAC/IP Advertisement routes and Ethernet A-D per EVI/per ES routes to construct the set of next hops for the advertised MAC address. (§14.1.2) | MUST | 14.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no ECMP set is built from EVPN routes |
| `RFC7432-9.2.2-1` | If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to the reserved ESI value of 0 or MAX-ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be based on the MAC/IP Advertisement route alone. (§9.2.2) | MUST | 9.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-9.2.2-2` | If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to a non-reserved ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be when both the MAC/IP Advertisement route AND the associated set of Ethernet A-D per ES routes have been received. (§9.2.2) | MUST | 9.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-10.1-4` | Unless it is known a priori (by means outside of this document) that all PEs of a given EVPN instance act as a default gateway for that EVPN instance, the MPLS label MUST be set to a valid downstream assigned label. (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Default Gateway extended community and allocates no label space for one: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing |
| `RFC7432-6.2-2` | The MPLS-encapsulated frames MUST remain tagged with the originating VID. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.3-3` | Furthermore, the advertising PE advertises the MPLS Label1 in the MAC/IP Advertisement route representing both the Ethernet Tag ID and the EVI, so that upon receiving an MPLS-encapsulated packet, it can identify the corresponding bridge table from the MPLS EVPN label and perform Ethernet Tag ID translation ONLY at the disposition PE -- i.e., the Ethernet frames transported over the MPLS/IP network MUST remain tagged with the originating VID, and VID translation is performed on the disposition PE. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.1-3` | the Ethernet frames transported over an MPLS/IP network SHOULD remain tagged with the originating VID (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-17.3-2` | When a PE receives a withdrawal of a particular Ethernet A-D route from an advertising PE, it SHOULD consider all the MAC/IP Advertisement routes that are learned from the same ESI as in the Ethernet A-D route from the advertising PE as having been withdrawn. (§17.3) | SHOULD | 17.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1.1-5` | if PE2 receives a BUM packet for VLAN1 from CE1, then it SHOULD encapsulate the packet with an ESI label received from PE1 when sending it to PE1 in order to avoid any transient loops during a failure scenario that would impact ES1 (§8.3.1.1) | SHOULD | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-18-1` | If a network uses entropy labels [RFC6790], then the control word SHOULD NOT be used when sending EVPN-encapsulated packets over an MP2P LSP. - When sending EVPN-encapsulated packets over a P2MP LSP or P2P LSP, then the control word SHOULD NOT be used. (§18) | SHOULD NOT | 18 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-18-2` | If a network uses deep packet inspection for its ECMP, then the "Preferred PW MPLS Control Word" [RFC4385] SHOULD be used with the value 0 (e.g., a 4-octet field with a value of zero) when sending EVPN-encapsulated packets over an MP2P LSP. (§18) | SHOULD | 18 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-6.3-4` | The Ethernet Tag ID in all EVPN routes MUST be set to that VID. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type and binds no VLAN to an EVPN instance: grep -rniE 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute takes the Ethernet Tag ID verbatim from the configured route, so ze holds no VID to copy into it |
| `RFC7432-7.10.1-1` | The Global Administrator field of the RT MUST be set to the Autonomous System (AS) number with which the PE is associated (§7.10.1) | MUST | 7.10.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "it is highly desirable to auto-derive the RT from the Ethernet Tag ID (VLAN ID) for that EVPN instance"; ze auto-derives no route target: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute attaches only the extended communities the operator configures |
| `RFC7432-7.10.1-2` | The 12-bit VLAN ID MUST be encoded in the lowest 12 bits of the Local Administrator field, with the remaining bits set to zero (§7.10.1) | MUST | 7.10.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "it is highly desirable to auto-derive the RT from the Ethernet Tag ID (VLAN ID) for that EVPN instance"; ze auto-derives no route target, so it encodes no VLAN ID into one: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute attaches only the extended communities the operator configures |
| `RFC7432-8.2.1-9` | Support of this route is REQUIRED. (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L34). **positive:** `unit/verify` [`TestEVPNIngressPreservesPerESZeroLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7432_session_validation_evpn_test.go#L12). **positive:** `unit/verify` [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L13). **negative:** no negative test. **{single-polarity}:** owner ruling 2026-10-02 continuation ba93202e, MSS retirement and EVPN polarity ruling. Invalid label encodings test their own wire obligations, not absence of route support. |
| `RFC7432-8.2.1-10` | The Route Distinguisher (RD) MUST be a Type 1 RD [RFC4364]. (§8.2.1) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L36). **positive:** `unit/verify` [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L15). **negative:** `unit/verify` [`TestConfiguredEVPNRefusesInvalidPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L55). **negative:** `unit/verify` [`TestOriginEthernetADPerESRefusesASTypeRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L34) |
| `RFC7432-8.2.1.1-1` | The set of Ethernet A-D routes per ES MUST carry the entire set of RTs for all the EVPN instances to which the Ethernet segment belongs (§8.2.1.1) | MUST | 8.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze holds no Ethernet segment and no EVPN instance membership, so it has no set of EVIs whose RTs an A-D per ES route could carry: grep -rniE 'designated.forwarder\|df.election' --include=*.go over internal/ and pkg/ matches nothing, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination can only require that a locally originated A-D per ES route carries at least one route target, which it does |
| `RFC7432-8.3-1` | In this case, the DF PE to which the CE is multihomed MUST drop the packet and not forward back to the CE. (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-8.3-2` | This label is referred to as the ESI label and MUST be distributed by all PEs when operating in All-Active redundancy mode using a set of Ethernet A-D per ES routes, per Section 8.2.1 above. (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC7432PerESRouteDistributesESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L85). **negative:** `unit/verify` [`TestRFC7432PerESRouteDistributesESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L91) |
| `RFC7432-8.3.1.1-6` | It MUST then push onto the MPLS label stack the MPLS label distributed by PE2 in the Inclusive Multicast Ethernet Tag route for VLAN1. (§8.3.1.1) | MUST | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-8.3.1.1-7` | If the next label is an ESI label that has not been assigned by PE2, then PE2 MUST drop the packet. (§8.3.1.1) | MUST | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-8.3.1.2-2` | If the next label is the ESI label assigned by PE1 to ES1 and PE3 is not connected to ES1, then PE3 MUST pop the label and flood the packet over all local ESIs in that EVPN instance. (§8.3.1.2) | MUST | 8.3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-8.5-2` | In the case of VLAN-(aware) bundle service, then the numerically lowest VLAN value in that bundle on that ES MUST be used in the modulo function. (§8.5) | MUST | 8.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze elects no designated forwarder: grep -rniE 'designated.forwarder\|df.election' --include=*.go over internal/ and pkg/ matches nothing, and ze holds no Ethernet segment or VLAN bundle whose lowest VLAN could enter a modulo function |
| `RFC7432-9.2.1-6` | The encoding of an IP address MUST be either 4 octets for IPv4 or 16 octets for IPv6 (§9.2.1) | MUST | 9.2.1 | **positive:** `unit/verify` [`TestRFC7432Type2IPAddressOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_type2_ip_test.go#L28). **negative:** `unit/verify` [`TestRFC7432Type2IPAddressOctetsShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_type2_ip_test.go#L92) |
| `RFC7432-10.1-5` | Each PE that acts as a default gateway for a given EVPN instance that receives this route and imports it as per procedures specified in this document MUST create MAC forwarding state that enables it to apply IP forwarding to the packets destined to the MAC address carried in the route. (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze acts as no EVPN default gateway and creates no MAC forwarding state: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-11.1-3` | The BGP advertisement for the Inclusive Multicast Ethernet Tag route MUST also carry one or more Route Target (RT) attributes. (§11.1) | MUST | 11.1 | **positive:** `unit/verify` [`TestConfiguredEVPNIMETOriginatorIsNotNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L66). **positive:** `unit/verify` [`TestOriginInclusiveMulticastIncludesRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L47). **negative:** `unit/verify` [`TestConfiguredEVPNIMETRefusesMissingRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L83). **negative:** `unit/verify` [`TestOriginInclusiveMulticastRefusesMissingRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L59) |
| `RFC7432-11.2-5` | + If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for EVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no P-multicast tree and constructs no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go lists PMSI among the attribute codes with no parser, and internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds none for an Inclusive Multicast Ethernet Tag route |
| `RFC7432-11.2-6` | In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute MUST carry an MPLS upstream assigned label, which the PE has bound uniquely to the EVI associated with this update (as determined by its RTs). (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze aggregates no EVPN instance onto a P-multicast tree and assigns no upstream label: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds no PMSI Tunnel attribute and ze holds no EVI |
| `RFC7432-11.2-7` | If the PE has already advertised Inclusive Multicast Ethernet Tag routes for two or more EVIs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze aggregates no EVPN instance onto a P-multicast tree, so it never re-advertises Inclusive Multicast Ethernet Tag routes for one: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds no PMSI Tunnel attribute |
| `RFC7432-13.1-6` | If a distinct P2MP LSP is used for a given Ethernet tag in the EVPN instance, then only the PEs in the Ethernet tag MUST be the leaves of the P2MP LSP. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-13.1-7` | The packet MUST be encapsulated in the P2MP LSP label stack. (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| `RFC7432-17.3-3` | When the MAC entry on the PE ages out, the PE MUST withdraw the MAC address from BGP (§17.3) | MUST | 17.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze learns no MAC address and holds no MAC table that could age: MAC/IP Advertisement routes reach ze only from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute), and ze forwards no EVPN traffic |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7432-7.9-1`](#rfc7432-7.9-1) The Route Distinguisher (RD) MUST be set to the RD of the MAC-VRF that is advertising the NLRI. (§7.9) | {gap}, no test | the RD of an EVPN NLRI is whatever the route command supplies -- l2vpnRouteToEVPNParams parses it with ParseRDString and hands it straight to the NLRI constructors (internal/component/bgp/plugins/nlri/evpn/encode.go:71-78) -- and ze holds no MAC-VRF whose RD the advertisement could be required to match |
| [`RFC7432-7.9-2`](#rfc7432-7.9-2) An RD MUST be assigned for a given MAC-VRF on a PE (§7.9) | {gap}, no test | parseL2VPNArgs refuses a route with an empty route-distinguisher (internal/component/bgp/plugins/nlri/evpn/encode.go:240-242), so every advertised EVPN NLRI does carry an RD, but the unit of assignment is the configured route and ze has no MAC-VRF construct nor any per-MAC-VRF RD allocation |
| [`RFC7432-7.9-3`](#rfc7432-7.9-3) RD MUST be unique across all MAC-VRFs on a PE (§7.9) | {gap}, no test | each route command's RD is parsed on its own (internal/component/bgp/plugins/nlri/evpn/encode.go:73-77) and no code compares it against the RDs already in use, so nothing enforces uniqueness across instances |
| [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1) The Ethernet Tag ID MUST be set to MAX-ET. (§8.2.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams passes the operator-supplied EthernetTag to NewEVPNType1. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination recognizes MAX-ET and validates the per-ES branch, but these producers have no independent per-ES intent selector that forces MAX-ET. A configured MAX-ET example establishes support, not independent enforcement of tag selection |
| [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3) The ESI Label extended community MUST be included in the route. (§8.2.1) | {gap}, no test | row-specific enforcement proof remains unverified, not an absent producer. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute merges configured extended communities, calls ValidateEVPNOrigination and attaches the bytes to PluginRoute; internal/component/bgp/plugins/nlri/evpn/encode.go::EncodeRoute carries ExtCommunityBytes through l2vpnRouteToEVPNParams to BuildEVPN. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination rejects MAX-ET Type 1 origination without an ESI Label community (type 0x06/subtype 0x01). This implemented presence guard requires its own proof and verdict; the route-support verdict does not promote this row |
| [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4) If All-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 0 and the MPLS label in that Extended Community MUST be set to a valid MPLS label value. (§8.2.1) | {gap}, no test | configured ESI Label bytes are carried by internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks community presence, not the Single-Active bit or embedded MPLS label against an independent All-Active mode setting. That conditional field validation is unimplemented. Autonomous multihoming remains outside the selected control-plane role; its absence does not establish conformance of configured signaling |
| [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5) The MPLS label in this Extended Community is referred to as the ESI label and MUST have the same value in each Ethernet A-D per ES route advertised for the ES. (§8.2.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute carries each route's supplied ESI Label bytes. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks community presence without comparing embedded labels across separately originated routes with the same ESI. The cross-route consistency invariant is unimplemented; one conforming per-ES example does not prove it |
| [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6) If the advertising PE is using P2MP MPLS LSPs for sending multicast, broadcast, or unknown unicast traffic, then this label MUST be an upstream assigned MPLS label. (§8.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the condition is a PE sending BUM traffic over P2MP MPLS LSPs. Ze's selected BGP control-plane role, documented in docs/architecture/wire/nlri-evpn.md, does not forward Ethernet/BUM traffic or manage its P2MP trees. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry explicitly supplied ESI Label bytes, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks their presence; these producers neither establish P2MP LSPs nor allocate upstream labels. Carrying the community does not fill the conditional forwarding role |
| [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7) If Single-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 1 (§8.2.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry configured ESI Label bytes. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination checks presence, not the Single-Active bit against an independent Single-Active mode setting. That conditional flag validation is unimplemented. Ze does not autonomously select or operate a redundancy mode in its documented BGP control-plane role; this role boundary is not proof that explicitly configured signaling satisfies the condition |
| [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8) Each Ethernet A-D per ES route MUST carry one or more Route Target (RT) attributes. (§8.2.1) | {gap}, no test | row-specific enforcement proof remains unverified, not an absent producer or guard. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute merges and attaches configured extended communities; encode.go::l2vpnRouteToEVPNParams supplies ExtCommunityBytes to BuildEVPN. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination rejects MAX-ET Type 1 origination without a Route Target (type 0x00, 0x01 or 0x02/subtype 0x02). This presence guard requires its own proof and verdict; it does not auto-derive RTs or guarantee the entire ES-to-EVI RT set required by RFC7432-8.2.1.1-1 |
| [`RFC7432-7.6-1`](#rfc7432-7.6-1) The Ethernet Segment route filtering MUST be done such that the Ethernet Segment route is imported only by the PEs that are multihomed to the same Ethernet segment. (§8.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: Ze's selected boundary is explicit control-plane origination, not the EVPN PE segment-discovery role. Configured ES-Import bytes can be advertised, but Ze has no Ethernet-segment membership or automatically constructed segment import rules; the origin guard does not establish this distribution behavior |
| [`RFC7432-7.6-2`](#rfc7432-7.6-2) A BGP speaker that implements RT Constraint [RFC4684] MUST apply the RT Constraint procedures to the ES-Import RT as well. (§7.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no RT Constraint procedures: the rtc plugin is an NLRI codec alone (internal/component/bgp/plugins/nlri/rtc/types.go:97 parses, :186 writes) and grep -rni 'rtc' over internal/component/bgp/reactor, internal/component/bgp/rib and internal/component/bgp/plugins/rib matches only two comments naming RTC as a non-CIDR family, so no route-target-based filtering exists to extend to an ES-Import RT |
| [`RFC7432-11-1`](#rfc7432-11-1) Each PE MUST advertise an "Inclusive Multicast Ethernet Tag route" to enable the above. (§11) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVI or MAC-VRF: grep -rn '\\bEVI\\b' --include=*.go over internal/component/bgp matches nothing and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing, so no per-instance origination duty is expressed anywhere; the Inclusive Multicast codec advertises only what an operator asks for |
| [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2) The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the IPv4 or IPv6 address of the advertising PE. (§9.2.1, §11.1) | {gap}, no test | the earlier zero-address fallback claim is obsolete. internal/component/bgp/message/update_build_evpn.go::BuildEVPN rejects missing, unspecified and multicast next hops before buildMPReachEVPN writes the supplied address. Native configuration retains next-hop self and internal/component/bgp/reactor/peer_static_routes.go::toPluginParams resolves it from the connected local address. These guards do not establish that every explicitly supplied next hop belongs to the advertising PE. Whole-row enforcement remains unverified |
| [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3) MPLS Label1 MUST be downstream assigned (§9.2.1) | {gap}, no test | Label1 is the number typed into the route command (internal/component/bgp/plugins/nlri/evpn/encode.go:101-107); ze allocates no local label space for EVPN, so nothing makes the advertised label downstream assigned |
| [`RFC7432-9.1-1`](#rfc7432-9.1-1) The PEs in a particular EVPN instance MUST support local data-plane learning using standard IEEE Ethernet learning procedures. (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4) The BGP advertisement for the MAC/IP Advertisement route MUST also carry one or more Route Target (RT) attributes. (§9.2.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams and config.go::parseConfigRoute carry configured extended communities. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination has no Type 2 Route Target presence check, so explicit MAC/IP origination without a Route Target is not refused. The missing guard, not an inability to carry configured Route Targets, is the gap |
| [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1) The Ethernet A-D route MUST carry one or more Route Target (RT) attributes, per Section 7.10. (§8.4.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams and config.go::parseConfigRoute carry configured extended communities. internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination requires a Route Target for Type 1 only when its Ethernet Tag is MAX-ET. Explicit per-EVI origination without a Route Target is not refused. The per-ES support proof does not establish this separate per-EVI obligation |
| [`RFC7432-10-2`](#rfc7432-10-2) If there are multiple IP addresses associated with a MAC address, then multiple MAC/IP Advertisement routes MUST be generated, one for each IP address. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no IP-to-MAC binding table to enumerate: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-10-3`](#rfc7432-10-3) When the IP address is dissociated with the MAC address, then the MAC/IP Advertisement route with that particular IP address MUST be withdrawn. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no IP-to-MAC binding whose removal could drive a withdrawal: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-10-4`](#rfc7432-10-4) If the receiving PE receives both the MAC-only route and the MAC/IP route, then when it receives a withdraw message for the MAC/IP route, it MUST delete the corresponding entry from the ARP table but not the MAC entry from the MAC-VRF table, unless it receives a withdraw message for the MAC-only route. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no ARP or ND cache to delete an entry from: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-11.1-1`](#rfc7432-11.1-1) The Originating Router's IP Address field value MUST be set to an IP address of the PE that should be common for all the EVIs on the PE (e.g., this address may be the PE's loopback address). (§11.1) | {gap}, no test | internal/component/bgp/plugins/nlri/evpn/encode.go::l2vpnRouteToEVPNParams uses the route's explicit IP as its originator, or its explicit next hop when IP is absent, and refuses an absent originating address. It does not establish PE ownership or a common address across EVIs. The configured IMET originator can differ from the session next hop |
| [`RFC7432-11.1-2`](#rfc7432-11.1-2) The assignment of RTs as described in Section 7.10 MUST be followed. (§11.1) | {gap}, no test | row-specific enforcement proof remains unverified. Section 7.10 permits configured RTs or automatic derivation. internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute and encode.go::l2vpnRouteToEVPNParams carry configured RTs, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination requires at least one on IMET. The earlier claim that configured communities could not reach this producer is obsolete. Automatic derivation is not selected, as documented in docs/architecture/wire/nlri-evpn.md |
| [`RFC7432-11.2-1`](#rfc7432-11.2-1) In order to identify the P-tunnel used for sending broadcast, unknown unicast, or multicast traffic, the Inclusive Multicast Ethernet Tag route MUST carry a Provider Multicast Service Interface (PMSI) Tunnel attribute as specified in [RFC6514]. (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-11.2-2`](#rfc7432-11.2-2) + The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-11.2-3`](#rfc7432-11.2-3) The PMSI Tunnel attribute MUST carry a downstream assigned MPLS label. (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-6.1-1`](#rfc7432-6.1-1) a VID translation MUST be supported in the data path and MUST be performed on the disposition PE. (§6.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and no data path performs VID translation |
| [`RFC7432-6.1-2`](#rfc7432-6.1-2) The Ethernet Tag ID in all EVPN routes MUST be set to 0. (§6.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189) |
| [`RFC7432-6.2-1`](#rfc7432-6.2-1) This implies that MAC addresses MUST be unique across all VLANs for that EVI in order for this service to work. (§6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no bundle of VLANs shares a MAC learning space |
| [`RFC7432-6.3-1`](#rfc7432-6.3-1) In the case where a single VLAN is represented by a single VID and thus no VID translation is required, an MPLS-encapsulated packet MUST carry that VID. (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.3-2`](#rfc7432-6.3-2) In the case where a single VLAN is represented by different VIDs on different CEs and thus VID translation is required, a normalized Ethernet Tag ID (VID) MUST be carried in the EVPN BGP routes. (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no VID normalization happens before origination |
| [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1) - A non-DF ingress PE MUST include the ESI label distributed by the DF egress PE in the copy of a BUM packet sent to it. (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1) Penultimate hop popping MUST be disabled on the P2MP LSPs used in the MPLS transport infrastructure for EVPN. (§8.3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no P2MP LSP and controls no penultimate hop popping: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1) This label MUST be programmed in the platform label space by the advertising PE, and the forwarding entry for this label must result in NOT forwarding packets received with this label onto the Ethernet segment for which the label was distributed. (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-15-1`](#rfc7432-15-1) In order to process mobility events correctly, an implementation MUST handle scenarios in which sequence number wraparound occurs. (§15) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing |
| [`RFC7432-15-2`](#rfc7432-15-2) In order to allow all of the PEs in the EVPN instance to correctly determine the current location of the MAC address, all advertisements of it being reachable via the previous Ethernet segment MUST be withdrawn by the PEs, for the previous Ethernet segment, that had advertised it. (§15) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no MAC move is observed and nothing is withdrawn |
| [`RFC7432-15.1-1`](#rfc7432-15.1-1) The PE MUST alert the operator and stop sending and processing any BGP MAC/IP Advertisement routes for that MAC address until a corrective action is taken by the operator. (§15.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no duplicate-MAC counter or operator alert exists |
| [`RFC7432-15.1-2`](#rfc7432-15.1-2) The values of M and N MUST be configurable to allow for flexibility in operator control. (§15.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the M and N duplicate-detection parameters have nothing to configure |
| [`RFC7432-15.2-1`](#rfc7432-15.2-1) If a PE receives such advertisements and later learns the same MAC address(es) via local learning, then the PE MUST alert the operator. (§15.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the sticky flag is never read and local learning never happens |
| [`RFC7432-10.1-1`](#rfc7432-10.1-1) This is accomplished by requiring the route to carry the Default Gateway extended community defined in Section 7.8 (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Default Gateway extended community: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing, and no default-gateway route origination exists |
| [`RFC7432-17.3-1`](#rfc7432-17.3-1) If the connectivity between the multihomed CE and one of the PEs to which it is attached fails, the PE MUST withdraw the set of Ethernet A-D per ES routes that had been previously advertised for that ES. (§17.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no PE-to-CE attachment circuit: grep -rni 'attachment circuit\|pe.to.ce' --include=*.go over internal/ matches nothing, so no link event drives an EVPN withdrawal |
| [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1) In this case, the set of Ethernet A-D per ES routes advertised by each PE MUST have the "Single-Active" bit in the flags of the ESI Label extended community set to 1. (§8.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.4-1`](#rfc7432-8.4-1) Therefore, in order to handle corner cases and race conditions, the Ethernet A-D per EVI route MUST NOT be used for traffic forwarding by a remote PE until it also receives the associated set of Ethernet A-D per ES routes. (§8.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no route ever becomes a forwarding entry to withhold |
| [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2) In both All-Active and Single-Active redundancy mode, an ingress PE MUST NOT include an ESI label in the copy of a BUM packet sent to an egress PE that is not attached to the ES through which the BUM packet entered the EVI. (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3) If the next label is the ESI label assigned by PE2 for ES1, then PE2 MUST NOT forward the packet onto ES1. (§8.3.1.1, §8.3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code |
| [`RFC7432-5-3`](#rfc7432-5-3) The CE LACP System MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-4`](#rfc7432-5-4) The CE LACP port key MUST be encoded in the 2 octets next to the System MAC address. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-5`](#rfc7432-5-5) The Root Bridge MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-6`](#rfc7432-5-6) The CE Root Bridge Priority MUST be encoded in the 2 octets next to the Root Bridge MAC address. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-7`](#rfc7432-5-7) The PE MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-8`](#rfc7432-5-8) The Local Discriminator value MUST be encoded in the low-order 3 octets of the ESI Value. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-9`](#rfc7432-5-9) The system router ID MUST be encoded in the high-order 4 octets of the ESI Value field. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-10`](#rfc7432-5-10) The Local Discriminator value MUST be encoded in the 4 octets next to the IP address. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-11`](#rfc7432-5-11) This is an AS number owned by the system and MUST be encoded in the high-order 4 octets of the ESI Value field. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-12`](#rfc7432-5-12) The Local Discriminator value MUST be encoded in the 4 octets next to the AS number. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-13`](#rfc7432-5-13) If the CE(s) constituting an Ethernet segment is (are) managed by the network operator, then ESI uniqueness should be guaranteed; however, if the CE(s) is (are) not managed, then the operator MUST configure a network-wide unique ESI for that Ethernet segment. (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no multihomed Ethernet segment and no managed CE: ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code; the ESI is operator-provided text parsed by ParseESIString (internal/component/bgp/plugins/nlri/evpn/types.go:119) |
| [`RFC7432-12-1`](#rfc7432-12-1) PEx MUST NOT send the frame to other PEs, since PEy would have already done so. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-12.1-1`](#rfc7432-12.1-1) The PE that receives a packet with this particular MPLS label MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received label is classified as BUM |
| [`RFC7432-12.1-2`](#rfc7432-12.1-2) Further, if the MAC address is a unicast MAC address, the PE MUST treat the packet as an unknown unicast packet. (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received frame is classified as unknown unicast |
| [`RFC7432-12.2-1`](#rfc7432-12.2-1) The PE that receives a packet on the P2MP LSP specified in the PMSI Tunnel attribute MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names, and ze receives no P2MP LSP traffic |
| [`RFC7432-13.1-1`](#rfc7432-13.1-1) The PE MUST flood the packet to other PEs. The PE MUST first encapsulate the packet in the ESI MPLS label as described in Section 8.3. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-2`](#rfc7432-13.1-2) If ingress replication is used, the packet MUST be replicated to each remote PE, with the VPN label being an MPLS label determined as follows: This is the MPLS label advertised by the remote PE in a PMSI Tunnel attribute in the Inclusive Multicast Ethernet Tag route for a <MAC-VRF> or <MAC-VRF, Ethernet tag> combination. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-3`](#rfc7432-13.1-3) If P2MP LSPs are being used, the packet MUST be sent on the P2MP LSP of which the PE is the root, for the Ethernet tag in the EVPN instance. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-4`](#rfc7432-13.1-4) If the same P2MP LSP is used for all Ethernet tags, then all the PEs in the EVPN instance MUST be the leaves of the P2MP LSP. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-5`](#rfc7432-13.1-5) If the MAC address is unknown, then, if the administrative policy on the PE does not allow flooding of unknown unicast traffic: - the PE MUST drop the packet. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no packet is dropped for an unknown MAC |
| [`RFC7432-14.1-1`](#rfc7432-14.1-1) Whenever a remote PE imports a MAC/IP Advertisement route for a given <ESI, Ethernet tag> in a MAC-VRF, it MUST examine all imported Ethernet A-D routes for that ESI in order to determine the load- balancing characteristics of the Ethernet segment. (§14.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so importing a MAC/IP route triggers no correlation with Ethernet A-D routes |
| [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2) For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from at least one PE, where the "Single-Active" flag in the ESI Label extended community is set, then the remote PE MUST deduce that the ES is operating in Single-Active redundancy mode. (§14.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3) If there is more than one backup PE for a given ES, the remote PE MUST use the primary PE's withdrawal of its set of Ethernet A-D per ES routes as a trigger to start flooding traffic for the associated MAC addresses (as long as flooding of unknown unicast packets is administratively allowed), as it is not possible to select a single backup PE. (§14.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no primary or backup PE state and no flooding decision exists |
| [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1) For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from one or more PEs and none of them have the "Single-Active" flag in the ESI Label extended community set, then the remote PE MUST deduce that the ES is operating in All-Active redundancy mode. (§14.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2) The remote PE MUST use received MAC/IP Advertisement routes and Ethernet A-D per EVI/per ES routes to construct the set of next hops for the advertised MAC address. (§14.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no ECMP set is built from EVPN routes |
| [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1) If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to the reserved ESI value of 0 or MAX-ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be based on the MAC/IP Advertisement route alone. (§9.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2) If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to a non-reserved ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be when both the MAC/IP Advertisement route AND the associated set of Ethernet A-D per ES routes have been received. (§9.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-10.1-4`](#rfc7432-10.1-4) Unless it is known a priori (by means outside of this document) that all PEs of a given EVPN instance act as a default gateway for that EVPN instance, the MPLS label MUST be set to a valid downstream assigned label. (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Default Gateway extended community and allocates no label space for one: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing |
| [`RFC7432-6.2-2`](#rfc7432-6.2-2) The MPLS-encapsulated frames MUST remain tagged with the originating VID. (§6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.3-3`](#rfc7432-6.3-3) Furthermore, the advertising PE advertises the MPLS Label1 in the MAC/IP Advertisement route representing both the Ethernet Tag ID and the EVI, so that upon receiving an MPLS-encapsulated packet, it can identify the corresponding bridge table from the MPLS EVPN label and perform Ethernet Tag ID translation ONLY at the disposition PE -- i.e., the Ethernet frames transported over the MPLS/IP network MUST remain tagged with the originating VID, and VID translation is performed on the disposition PE. (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.3-4`](#rfc7432-6.3-4) The Ethernet Tag ID in all EVPN routes MUST be set to that VID. (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type and binds no VLAN to an EVPN instance: grep -rniE 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute takes the Ethernet Tag ID verbatim from the configured route, so ze holds no VID to copy into it |
| [`RFC7432-7.10.1-1`](#rfc7432-7.10.1-1) The Global Administrator field of the RT MUST be set to the Autonomous System (AS) number with which the PE is associated (§7.10.1) | no test | no test carries this requirement id; annotated {feature-declined}: "it is highly desirable to auto-derive the RT from the Ethernet Tag ID (VLAN ID) for that EVPN instance"; ze auto-derives no route target: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute attaches only the extended communities the operator configures |
| [`RFC7432-7.10.1-2`](#rfc7432-7.10.1-2) The 12-bit VLAN ID MUST be encoded in the lowest 12 bits of the Local Administrator field, with the remaining bits set to zero (§7.10.1) | no test | no test carries this requirement id; annotated {feature-declined}: "it is highly desirable to auto-derive the RT from the Ethernet Tag ID (VLAN ID) for that EVPN instance"; ze auto-derives no route target, so it encodes no VLAN ID into one: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute attaches only the extended communities the operator configures |
| [`RFC7432-8.2.1.1-1`](#rfc7432-8.2.1.1-1) The set of Ethernet A-D routes per ES MUST carry the entire set of RTs for all the EVPN instances to which the Ethernet segment belongs (§8.2.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze holds no Ethernet segment and no EVPN instance membership, so it has no set of EVIs whose RTs an A-D per ES route could carry: grep -rniE 'designated.forwarder\|df.election' --include=*.go over internal/ and pkg/ matches nothing, and internal/component/bgp/message/update_build_evpn.go::ValidateEVPNOrigination can only require that a locally originated A-D per ES route carries at least one route target, which it does |
| [`RFC7432-8.3-1`](#rfc7432-8.3-1) In this case, the DF PE to which the CE is multihomed MUST drop the packet and not forward back to the CE. (§8.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-8.3.1.1-6`](#rfc7432-8.3.1.1-6) It MUST then push onto the MPLS label stack the MPLS label distributed by PE2 in the Inclusive Multicast Ethernet Tag route for VLAN1. (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-8.3.1.1-7`](#rfc7432-8.3.1.1-7) If the next label is an ESI label that has not been assigned by PE2, then PE2 MUST drop the packet. (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-8.3.1.2-2`](#rfc7432-8.3.1.2-2) If the next label is the ESI label assigned by PE1 to ES1 and PE3 is not connected to ES1, then PE3 MUST pop the label and flood the packet over all local ESIs in that EVPN instance. (§8.3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-8.5-2`](#rfc7432-8.5-2) In the case of VLAN-(aware) bundle service, then the numerically lowest VLAN value in that bundle on that ES MUST be used in the modulo function. (§8.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze elects no designated forwarder: grep -rniE 'designated.forwarder\|df.election' --include=*.go over internal/ and pkg/ matches nothing, and ze holds no Ethernet segment or VLAN bundle whose lowest VLAN could enter a modulo function |
| [`RFC7432-10.1-5`](#rfc7432-10.1-5) Each PE that acts as a default gateway for a given EVPN instance that receives this route and imports it as per procedures specified in this document MUST create MAC forwarding state that enables it to apply IP forwarding to the packets destined to the MAC address carried in the route. (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze acts as no EVPN default gateway and creates no MAC forwarding state: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-11.2-5`](#rfc7432-11.2-5) + If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for EVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no P-multicast tree and constructs no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go lists PMSI among the attribute codes with no parser, and internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds none for an Inclusive Multicast Ethernet Tag route |
| [`RFC7432-11.2-6`](#rfc7432-11.2-6) In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute MUST carry an MPLS upstream assigned label, which the PE has bound uniquely to the EVI associated with this update (as determined by its RTs). (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze aggregates no EVPN instance onto a P-multicast tree and assigns no upstream label: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds no PMSI Tunnel attribute and ze holds no EVI |
| [`RFC7432-11.2-7`](#rfc7432-11.2-7) If the PE has already advertised Inclusive Multicast Ethernet Tag routes for two or more EVIs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze aggregates no EVPN instance onto a P-multicast tree, so it never re-advertises Inclusive Multicast Ethernet Tag routes for one: internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute builds no PMSI Tunnel attribute |
| [`RFC7432-13.1-6`](#rfc7432-13.1-6) If a distinct P2MP LSP is used for a given Ethernet tag in the EVPN instance, then only the PEs in the Ethernet tag MUST be the leaves of the P2MP LSP. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-13.1-7`](#rfc7432-13.1-7) The packet MUST be encapsulated in the P2MP LSP label stack. (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no EVPN traffic: it has no bridge domain, no BUM replication and no P2MP LSP, grep -rli evpn over internal/plugins/fib/ and internal/component/iface/ matches nothing, and EVPN routes reach ze only as BGP UPDATEs it encodes, decodes and originates from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute) |
| [`RFC7432-17.3-3`](#rfc7432-17.3-3) When the MAC entry on the PE ages out, the PE MUST withdraw the MAC address from BGP (§17.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze learns no MAC address and holds no MAC table that could age: MAC/IP Advertisement routes reach ze only from configuration (internal/component/bgp/plugins/nlri/evpn/config.go::parseConfigRoute), and ze forwards no EVPN traffic |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7432-7.9-1`](#rfc7432-7.9-1)

The Route Distinguisher (RD) MUST be set to the RD of the MAC-VRF that is advertising the NLRI. (§7.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.9-1, so no unit is bound to it.

### [`RFC7432-7.9-2`](#rfc7432-7.9-2)

An RD MUST be assigned for a given MAC-VRF on a PE (§7.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.9-2, so no unit is bound to it.

### [`RFC7432-7.9-3`](#rfc7432-7.9-3)

RD MUST be unique across all MAC-VRFs on a PE (§7.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.9-3, so no unit is bound to it.

### [`RFC7432-5-1`](#rfc7432-5-1)

The Ethernet Segment Identifier (ESI) MUST be set to the 10-octet value described in Section 5. (§8.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. row now quotes the §8.1.1 sentence that the ESI MUST be the 10-octet value of §5 (the extraction maps 8.1.1:2 here). types_test.go::TestEVPNESIIsTenOctets decodes and re-encodes a 10-octet ESI on the Ethernet Segment and Ethernet A-D routes and asserts the following field starts after octet ten; the negatives cut the ESI to nine octets and assert ErrEVPNTruncated. ESI is a [10]byte, so no origination path can produce another width

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L443) | unit/verify | unproven |
| positive | [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L403) | unit/verify | unproven |

### [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1)

The Ethernet Tag ID MUST be set to MAX-ET. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-1, so no unit is bound to it.

### [`RFC7432-8.2.1-2`](#rfc7432-8.2.1-2)

The MPLS label in the NLRI MUST be set to 0. (§8.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7432 Section 8.2.1: The MPLS label in the NLRI MUST be set to 0. Independently read all four current config/command carriers. TestConfiguredEVPNPerESRoute compares the complete Type1 MAX-ET NLRI including three zero label octets; TestOriginEthernetADPerESUsesTypeOneRD checks the zero label in the NLRI carried by MP_REACH. TestConfiguredEVPNRefusesInvalidPerESRoute and TestOriginEthernetADPerESRefusesLabelStack reject a nonzero label with a Type1 RD, RT and ESI Label community already supplied. ValidateEVPNOrigination rejects any nonzero label octet for MAX-ET Type1 and is reached by both parseConfigRoute and EncodeRoute through BuildEVPN. The second-label subcase and malformed-width ingress cases remain framing calibration, not support-negative evidence. Four discrimination tuples exist in the canonical rfc/discrimination/rfc7432.json; this read-only judgment makes no new execution or native freshness claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfiguredEVPNRefusesInvalidPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L54) | unit/verify | revert, verified |
| negative | [`TestOriginEthernetADPerESRefusesLabelStack`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L14) | unit/verify | revert, verified |

### [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3)

The ESI Label extended community MUST be included in the route. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-3, so no unit is bound to it.

### [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4)

If All-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 0 and the MPLS label in that Extended Community MUST be set to a valid MPLS label value. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-4, so no unit is bound to it.

### [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5)

The MPLS label in this Extended Community is referred to as the ESI label and MUST have the same value in each Ethernet A-D per ES route advertised for the ES. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-5, so no unit is bound to it.

### [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6)

If the advertising PE is using P2MP MPLS LSPs for sending multicast, broadcast, or unknown unicast traffic, then this label MUST be an upstream assigned MPLS label. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-6, so no unit is bound to it.

### [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7)

If Single-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 1 (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-7, so no unit is bound to it.

### [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8)

Each Ethernet A-D per ES route MUST carry one or more Route Target (RT) attributes. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-8, so no unit is bound to it.

### [`RFC7432-8.1.1-1`](#rfc7432-8.1.1-1)

The Route Distinguisher (RD) MUST be a Type 1 RD [RFC4364]. (§8.1.1). `message.ValidateEVPNOrigination` checks locally originated Type 4 NLRIs; current verification remains required. The receive codec remains permissive because this is an origination requirement.

Audit verdict: enforced (the tests do what the requirement demands), fresh. update_build_evpn_rfc7432_test.go: TestRFC7432EthernetSegmentOriginationAccepted builds an ES route with a Type 1 RD; TestRFC7432EthernetSegmentOriginationRefused asserts ErrEVPNOrigination and no UPDATE for Type 0 and Type 2 RDs, isolated with a valid ES-Import RT

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7432EthernetSegmentOriginationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC7432EthernetSegmentOriginationAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L54) | unit/verify | revert, verified |

### [`RFC7432-8.1.1-2`](#rfc7432-8.1.1-2)

The BGP advertisement that advertises the Ethernet Segment route MUST also carry an ES-Import Route Target, as defined in Section 7.6. (§8.1.1). `message.ValidateEVPNOrigination` requires extended community type 0x06/subtype 0x02 for local Type 4 origination; an ESI Label community is not a substitute. Current verification remains required.

Audit verdict: enforced (the tests do what the requirement demands), fresh. ES-Import presence proven by the HEAD units; the ESI-Label-substitute clause now proven: TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel builds a Type 4 route with ES-Import (extended communities exactly it) and refuses ESI Label 0x06/0x01 (same type octet, asserted) alone or with an RT via ErrEVPNOrigination, no UPDATE.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_es_import_test.go#L23) | unit/verify | revert, verified |
| negative | [`TestRFC7432EthernetSegmentOriginationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC7432EthernetSegmentRouteNeedsESImportNotESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_es_import_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC7432EthernetSegmentOriginationAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L55) | unit/verify | revert, verified |

### [`RFC7432-7.6-1`](#rfc7432-7.6-1)

The Ethernet Segment route filtering MUST be done such that the Ethernet Segment route is imported only by the PEs that are multihomed to the same Ethernet segment. (§8.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.6-1, so no unit is bound to it.

### [`RFC7432-7.6-2`](#rfc7432-7.6-2)

A BGP speaker that implements RT Constraint [RFC4684] MUST apply the RT Constraint procedures to the ES-Import RT as well. (§7.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.6-2, so no unit is bound to it.

### [`RFC7432-11-1`](#rfc7432-11-1)

Each PE MUST advertise an "Inclusive Multicast Ethernet Tag route" to enable the above. (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11-1, so no unit is bound to it.

### [`RFC7432-9.2.1-1`](#rfc7432-9.2.1-1)

The encoding of a MAC address MUST be the 6-octet MAC address specified by [802.1Q] and [802.1D-REV]. (§9.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7432 §9.2.1: "The MAC Address Length field is in bits, and it is set to 48. MAC address length values other than 48 bits are outside the scope of this document. The encoding of a MAC address MUST be the 6-octet MAC address specified by [802.1Q] and [802.1D-REV]." Read its sole tagged carrier internal/component/bgp/plugins/nlri/evpn/types_test.go::TestEVPNMACAddressLength and repaired shared helper evpnT2Body. The positive now supplies a complete Type-2 MAC-only route including mandatory Label1 00 01 01, requires decoded MAC 00:11:22:33:44:55, then independently pins the output length octet to 48 and the exact six MAC octets. The negative cases change only MAC length to 0/32/47/49/64/255 and require ErrEVPNInvalidAddress, preserving a valid zero-length optional IP field and label. types.go::parseEVPNType2 checks 48 before copying six bytes, and EVPNType2.WriteTo always emits 48 and the six-byte MAC. Thus the assertions implement the RFC's six-octet format, fail on altered width/content, isolate the MAC rule after the label repair, and cover the whole selected encoding obligation. One positive and one negative tag refer to distinct subcases, not the same assertion twice. Rejecting out-of-scope widths is a local validation boundary, not an invented RFC requirement for future MAC formats. Prior enforced judgment remains enforced after independent semantic rereading; no runtime execution or new discrimination record is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L318) | unit/verify | unproven |
| positive | [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L296) | unit/verify | unproven |

### [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2)

The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the IPv4 or IPv6 address of the advertising PE. (§9.2.1, §11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-2, so no unit is bound to it.

### [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3)

MPLS Label1 MUST be downstream assigned (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-3, so no unit is bound to it.

### [`RFC7432-9.1-1`](#rfc7432-9.1-1)

The PEs in a particular EVPN instance MUST support local data-plane learning using standard IEEE Ethernet learning procedures. (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.1-1, so no unit is bound to it.

### [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4)

The BGP advertisement for the MAC/IP Advertisement route MUST also carry one or more Route Target (RT) attributes. (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-4, so no unit is bound to it.

### [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1)

The Ethernet A-D route MUST carry one or more Route Target (RT) attributes, per Section 7.10. (§8.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.4.1-1, so no unit is bound to it.

### [`RFC7432-10-1`](#rfc7432-10-1)

For ARP and ND purposes, the IP Address Length field MUST be set to 32 for an IPv4 address or 128 for an IPv6 address. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7432 §10: "For ARP and ND purposes, the IP Address Length field MUST be set to 32 for an IPv4 address or 128 for an IPv6 address." Independently reread both current tagged carriers after the parent repaired their shared fixture. internal/component/bgp/plugins/nlri/evpn/rfc7432_ip_length_send_test.go::TestRFC7432MACIPSenderUsesBitLengths pins the sender's raw octet 31 to 32/128, exact IPv4/IPv6 address octets, complete length and mandatory Label1=100 bytes. types_test.go::TestEVPNIPAddressLength separately accepts literal IPv4/IPv6 layouts with the exact decoded address and refuses non-0/32/128 lengths with ErrEVPNInvalidAddress. Its evpnT2Body now includes valid mandatory Label1 bytes 00 01 01 after the IP field; the former missing-label confound is gone. The absent-IP control is permitted by §9.2.1, not an exception to §10's present-IP obligation. Producers types.go::NewEVPNType2/EVPNType2.WriteTo choose bit lengths independently of parseEVPNType2's literal 0/32/128 branches. Exact raw sender expectations distinguish bits from octets, while parser negatives vary the tested length and retain valid surrounding fields. Current tags are two positive and one negative, backed by distinct conforming and invalid-length cases. The entire selected length obligation is now enforced at source level; no new runtime success or observed producer-break red is asserted. This fixture repair does not authorize EVPN forwarding or label-allocation features.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L377) | unit/verify | revert, verified |
| positive | [`TestRFC7432MACIPSenderUsesBitLengths`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_ip_length_send_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L355) | unit/verify | revert, verified |

### [`RFC7432-10-2`](#rfc7432-10-2)

If there are multiple IP addresses associated with a MAC address, then multiple MAC/IP Advertisement routes MUST be generated, one for each IP address. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-2, so no unit is bound to it.

### [`RFC7432-10-3`](#rfc7432-10-3)

When the IP address is dissociated with the MAC address, then the MAC/IP Advertisement route with that particular IP address MUST be withdrawn. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-3, so no unit is bound to it.

### [`RFC7432-10-4`](#rfc7432-10-4)

If the receiving PE receives both the MAC-only route and the MAC/IP route, then when it receives a withdraw message for the MAC/IP route, it MUST delete the corresponding entry from the ARP table but not the MAC entry from the MAC-VRF table, unless it receives a withdraw message for the MAC-only route. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-4, so no unit is bound to it.

### [`RFC7432-11.1-1`](#rfc7432-11.1-1)

The Originating Router's IP Address field value MUST be set to an IP address of the PE that should be common for all the EVIs on the PE (e.g., this address may be the PE's loopback address). (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.1-1, so no unit is bound to it.

### [`RFC7432-11.1-2`](#rfc7432-11.1-2)

The assignment of RTs as described in Section 7.10 MUST be followed. (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.1-2, so no unit is bound to it.

### [`RFC7432-11.2-1`](#rfc7432-11.2-1)

In order to identify the P-tunnel used for sending broadcast, unknown unicast, or multicast traffic, the Inclusive Multicast Ethernet Tag route MUST carry a Provider Multicast Service Interface (PMSI) Tunnel attribute as specified in [RFC6514]. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-1, so no unit is bound to it.

### [`RFC7432-11.2-2`](#rfc7432-11.2-2)

+ The Leaf Information Required flag of the PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-2, so no unit is bound to it.

### [`RFC7432-11.2-3`](#rfc7432-11.2-3)

The PMSI Tunnel attribute MUST carry a downstream assigned MPLS label. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-3, so no unit is bound to it.

### [`RFC7432-6.1-1`](#rfc7432-6.1-1)

a VID translation MUST be supported in the data path and MUST be performed on the disposition PE. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.1-1, so no unit is bound to it.

### [`RFC7432-6.1-2`](#rfc7432-6.1-2)

The Ethernet Tag ID in all EVPN routes MUST be set to 0. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.1-2, so no unit is bound to it.

### [`RFC7432-6.2-1`](#rfc7432-6.2-1)

This implies that MAC addresses MUST be unique across all VLANs for that EVI in order for this service to work. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.2-1, so no unit is bound to it.

### [`RFC7432-6.3-1`](#rfc7432-6.3-1)

In the case where a single VLAN is represented by a single VID and thus no VID translation is required, an MPLS-encapsulated packet MUST carry that VID. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-1, so no unit is bound to it.

### [`RFC7432-6.3-2`](#rfc7432-6.3-2)

In the case where a single VLAN is represented by different VIDs on different CEs and thus VID translation is required, a normalized Ethernet Tag ID (VID) MUST be carried in the EVPN BGP routes. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-2, so no unit is bound to it.

### [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1)

- A non-DF ingress PE MUST include the ESI label distributed by the DF egress PE in the copy of a BUM packet sent to it. (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-1, so no unit is bound to it.

### [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1)

Penultimate hop popping MUST be disabled on the P2MP LSPs used in the MPLS transport infrastructure for EVPN. (§8.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.2-1, so no unit is bound to it.

### [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1)

This label MUST be programmed in the platform label space by the advertising PE, and the forwarding entry for this label must result in NOT forwarding packets received with this label onto the Ethernet segment for which the label was distributed. (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1-1, so no unit is bound to it.

### [`RFC7432-15-1`](#rfc7432-15-1)

In order to process mobility events correctly, an implementation MUST handle scenarios in which sequence number wraparound occurs. (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15-1, so no unit is bound to it.

### [`RFC7432-15-2`](#rfc7432-15-2)

In order to allow all of the PEs in the EVPN instance to correctly determine the current location of the MAC address, all advertisements of it being reachable via the previous Ethernet segment MUST be withdrawn by the PEs, for the previous Ethernet segment, that had advertised it. (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15-2, so no unit is bound to it.

### [`RFC7432-15.1-1`](#rfc7432-15.1-1)

The PE MUST alert the operator and stop sending and processing any BGP MAC/IP Advertisement routes for that MAC address until a corrective action is taken by the operator. (§15.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.1-1, so no unit is bound to it.

### [`RFC7432-15.1-2`](#rfc7432-15.1-2)

The values of M and N MUST be configurable to allow for flexibility in operator control. (§15.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.1-2, so no unit is bound to it.

### [`RFC7432-15.2-1`](#rfc7432-15.2-1)

If a PE receives such advertisements and later learns the same MAC address(es) via local learning, then the PE MUST alert the operator. (§15.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.2-1, so no unit is bound to it.

### [`RFC7432-10.1-1`](#rfc7432-10.1-1)

This is accomplished by requiring the route to carry the Default Gateway extended community defined in Section 7.8 (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10.1-1, so no unit is bound to it.

### [`RFC7432-17.3-1`](#rfc7432-17.3-1)

If the connectivity between the multihomed CE and one of the PEs to which it is attached fails, the PE MUST withdraw the set of Ethernet A-D per ES routes that had been previously advertised for that ES. (§17.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-17.3-1, so no unit is bound to it.

### [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1)

In this case, the set of Ethernet A-D per ES routes advertised by each PE MUST have the "Single-Active" bit in the flags of the ESI Label extended community set to 1. (§8.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-1, so no unit is bound to it.

### [`RFC7432-8.4-1`](#rfc7432-8.4-1)

Therefore, in order to handle corner cases and race conditions, the Ethernet A-D per EVI route MUST NOT be used for traffic forwarding by a remote PE until it also receives the associated set of Ethernet A-D per ES routes. (§8.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.4-1, so no unit is bound to it.

### [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2)

In both All-Active and Single-Active redundancy mode, an ingress PE MUST NOT include an ESI label in the copy of a BUM packet sent to an egress PE that is not attached to the ES through which the BUM packet entered the EVI. (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-2, so no unit is bound to it.

### [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3)

If the next label is the ESI label assigned by PE2 for ES1, then PE2 MUST NOT forward the packet onto ES1. (§8.3.1.1, §8.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-3, so no unit is bound to it.

### [`RFC7432-5-3`](#rfc7432-5-3)

The CE LACP System MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-3, so no unit is bound to it.

### [`RFC7432-5-4`](#rfc7432-5-4)

The CE LACP port key MUST be encoded in the 2 octets next to the System MAC address. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-4, so no unit is bound to it.

### [`RFC7432-5-5`](#rfc7432-5-5)

The Root Bridge MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-5, so no unit is bound to it.

### [`RFC7432-5-6`](#rfc7432-5-6)

The CE Root Bridge Priority MUST be encoded in the 2 octets next to the Root Bridge MAC address. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-6, so no unit is bound to it.

### [`RFC7432-5-7`](#rfc7432-5-7)

The PE MAC address MUST be encoded in the high-order 6 octets of the ESI Value field. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-7, so no unit is bound to it.

### [`RFC7432-5-8`](#rfc7432-5-8)

The Local Discriminator value MUST be encoded in the low-order 3 octets of the ESI Value. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-8, so no unit is bound to it.

### [`RFC7432-5-9`](#rfc7432-5-9)

The system router ID MUST be encoded in the high-order 4 octets of the ESI Value field. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-9, so no unit is bound to it.

### [`RFC7432-5-10`](#rfc7432-5-10)

The Local Discriminator value MUST be encoded in the 4 octets next to the IP address. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-10, so no unit is bound to it.

### [`RFC7432-5-11`](#rfc7432-5-11)

This is an AS number owned by the system and MUST be encoded in the high-order 4 octets of the ESI Value field. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-11, so no unit is bound to it.

### [`RFC7432-5-12`](#rfc7432-5-12)

The Local Discriminator value MUST be encoded in the 4 octets next to the AS number. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-12, so no unit is bound to it.

### [`RFC7432-5-13`](#rfc7432-5-13)

If the CE(s) constituting an Ethernet segment is (are) managed by the network operator, then ESI uniqueness should be guaranteed; however, if the CE(s) is (are) not managed, then the operator MUST configure a network-wide unique ESI for that Ethernet segment. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-13, so no unit is bound to it.

### [`RFC7432-12-1`](#rfc7432-12-1)

PEx MUST NOT send the frame to other PEs, since PEy would have already done so. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12-1, so no unit is bound to it.

### [`RFC7432-12.1-1`](#rfc7432-12.1-1)

The PE that receives a packet with this particular MPLS label MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.1-1, so no unit is bound to it.

### [`RFC7432-12.1-2`](#rfc7432-12.1-2)

Further, if the MAC address is a unicast MAC address, the PE MUST treat the packet as an unknown unicast packet. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.1-2, so no unit is bound to it.

### [`RFC7432-12.2-1`](#rfc7432-12.2-1)

The PE that receives a packet on the P2MP LSP specified in the PMSI Tunnel attribute MUST treat the packet as a broadcast, multicast, or unknown unicast packet. (§12.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.2-1, so no unit is bound to it.

### [`RFC7432-13.1-1`](#rfc7432-13.1-1)

The PE MUST flood the packet to other PEs. The PE MUST first encapsulate the packet in the ESI MPLS label as described in Section 8.3. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-1, so no unit is bound to it.

### [`RFC7432-13.1-2`](#rfc7432-13.1-2)

If ingress replication is used, the packet MUST be replicated to each remote PE, with the VPN label being an MPLS label determined as follows: This is the MPLS label advertised by the remote PE in a PMSI Tunnel attribute in the Inclusive Multicast Ethernet Tag route for a <MAC-VRF> or <MAC-VRF, Ethernet tag> combination. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-2, so no unit is bound to it.

### [`RFC7432-13.1-3`](#rfc7432-13.1-3)

If P2MP LSPs are being used, the packet MUST be sent on the P2MP LSP of which the PE is the root, for the Ethernet tag in the EVPN instance. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-3, so no unit is bound to it.

### [`RFC7432-13.1-4`](#rfc7432-13.1-4)

If the same P2MP LSP is used for all Ethernet tags, then all the PEs in the EVPN instance MUST be the leaves of the P2MP LSP. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-4, so no unit is bound to it.

### [`RFC7432-13.1-5`](#rfc7432-13.1-5)

If the MAC address is unknown, then, if the administrative policy on the PE does not allow flooding of unknown unicast traffic: - the PE MUST drop the packet. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-5, so no unit is bound to it.

### [`RFC7432-14.1-1`](#rfc7432-14.1-1)

Whenever a remote PE imports a MAC/IP Advertisement route for a given <ESI, Ethernet tag> in a MAC-VRF, it MUST examine all imported Ethernet A-D routes for that ESI in order to determine the load- balancing characteristics of the Ethernet segment. (§14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1-1, so no unit is bound to it.

### [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2)

For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from at least one PE, where the "Single-Active" flag in the ESI Label extended community is set, then the remote PE MUST deduce that the ES is operating in Single-Active redundancy mode. (§14.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-2, so no unit is bound to it.

### [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3)

If there is more than one backup PE for a given ES, the remote PE MUST use the primary PE's withdrawal of its set of Ethernet A-D per ES routes as a trigger to start flooding traffic for the associated MAC addresses (as long as flooding of unknown unicast packets is administratively allowed), as it is not possible to select a single backup PE. (§14.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-3, so no unit is bound to it.

### [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1)

For a given ES, if the remote PE has imported the set of Ethernet A-D per ES routes from one or more PEs and none of them have the "Single-Active" flag in the ESI Label extended community set, then the remote PE MUST deduce that the ES is operating in All-Active redundancy mode. (§14.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.2-1, so no unit is bound to it.

### [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2)

The remote PE MUST use received MAC/IP Advertisement routes and Ethernet A-D per EVI/per ES routes to construct the set of next hops for the advertised MAC address. (§14.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.2-2, so no unit is bound to it.

### [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1)

If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to the reserved ESI value of 0 or MAX-ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be based on the MAC/IP Advertisement route alone. (§9.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.2-1, so no unit is bound to it.

### [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2)

If the Ethernet Segment Identifier field in a received MAC/IP Advertisement route is set to a non-reserved ESI, then if the receiving PE decides to install forwarding state for the associated MAC address, it MUST be when both the MAC/IP Advertisement route AND the associated set of Ethernet A-D per ES routes have been received. (§9.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.2-2, so no unit is bound to it.

### [`RFC7432-10.1-4`](#rfc7432-10.1-4)

Unless it is known a priori (by means outside of this document) that all PEs of a given EVPN instance act as a default gateway for that EVPN instance, the MPLS label MUST be set to a valid downstream assigned label. (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10.1-4, so no unit is bound to it.

### [`RFC7432-6.2-2`](#rfc7432-6.2-2)

The MPLS-encapsulated frames MUST remain tagged with the originating VID. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.2-2, so no unit is bound to it.

### [`RFC7432-6.3-3`](#rfc7432-6.3-3)

Furthermore, the advertising PE advertises the MPLS Label1 in the MAC/IP Advertisement route representing both the Ethernet Tag ID and the EVI, so that upon receiving an MPLS-encapsulated packet, it can identify the corresponding bridge table from the MPLS EVPN label and perform Ethernet Tag ID translation ONLY at the disposition PE -- i.e., the Ethernet frames transported over the MPLS/IP network MUST remain tagged with the originating VID, and VID translation is performed on the disposition PE. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-3, so no unit is bound to it.

### [`RFC7432-6.3-4`](#rfc7432-6.3-4)

The Ethernet Tag ID in all EVPN routes MUST be set to that VID. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-4, so no unit is bound to it.

### [`RFC7432-7.10.1-1`](#rfc7432-7.10.1-1)

The Global Administrator field of the RT MUST be set to the Autonomous System (AS) number with which the PE is associated (§7.10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.10.1-1, so no unit is bound to it.

### [`RFC7432-7.10.1-2`](#rfc7432-7.10.1-2)

The 12-bit VLAN ID MUST be encoded in the lowest 12 bits of the Local Administrator field, with the remaining bits set to zero (§7.10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.10.1-2, so no unit is bound to it.

### [`RFC7432-8.2.1-9`](#rfc7432-8.2.1-9)

Support of this route is REQUIRED. (§8.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7432 §8.2.1: "This section describes the procedures used to construct the Ethernet A-D per ES route, which is used for fast convergence (as discussed above) and for advertising the ESI label used for split-horizon filtering (as discussed in Section 8.3). Support of this route is REQUIRED." Within the established explicit BGP control-plane role, current positive carriers demonstrate actual route support rather than only malformed-label rejection: internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go::TestConfiguredEVPNPerESRoute checks the entire MAX-ET Type-1 NLRI plus configured RT/ESI Label bytes; internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go::TestOriginEthernetADPerESUsesTypeOneRD checks the complete NLRI in an actual UPDATE MP_REACH and decodes it; internal/component/bgp/reactor/rfc7432_session_validation_evpn_test.go::TestEVPNIngressPreservesPerESZeroLabel requires exact unchanged ingress NLRI and action None. Producers evpn/config.go::parseConfigRoute, encode.go::EncodeRoute/l2vpnRouteToEVPNParams, message/update_build_evpn.go::BuildEVPN/ValidateEVPNOrigination and session_validation.go::enforceRFC7606 support those paths. Removing the route or changing its layout fails these assertions. Current single-polarity annotation is semantically justified: malformed label/RD cases are neighboring obligations, not a negative of route support. It does not waive any runtime proof, invent an owner exception, or claim PE fast-convergence, split-horizon, automatic ESI/label allocation or undisclosed whole EVPN support.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestEVPNIngressPreservesPerESZeroLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7432_session_validation_evpn_test.go#L12) | unit/verify | revert, verified |

### [`RFC7432-8.2.1-10`](#rfc7432-8.2.1-10)

The Route Distinguisher (RD) MUST be a Type 1 RD [RFC4364]. (§8.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7432 §8.2.1: "The Route Distinguisher (RD) MUST be a Type 1 RD [RFC4364]. The value field comprises an IP address of the PE (typically, the loopback address) followed by a number unique to the PE." The selected row is the Type-1 format obligation. Read all four carriers: internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go::TestConfiguredEVPNPerESRoute pins complete Type-1 RD bytes; TestConfiguredEVPNRefusesInvalidPerESRoute includes an otherwise guarded per-ES route with AS-specific RD; internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go::TestOriginEthernetADPerESUsesTypeOneRD pins RD=1 in the encoded and MP_REACH-carried NLRI; TestOriginEthernetADPerESRefusesASTypeRD demands refusal/nil output for per-ES and permits the same AS-specific format for per-EVI. RT and ESI Label are present, so missing-community rejection cannot mask the RD error. evpn/config.go::parseConfigRoute and encode.go::EncodeRoute both reach message/update_build_evpn.go::ValidateEVPNOrigination, which checks RD type before label/community checks for MAX-ET. Removing the type check would violate the negative pair. This proves required RD type within configured control-plane origination, not automatic PE-owned RD allocation/uniqueness or a new multihoming role; no runtime claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfiguredEVPNRefusesInvalidPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestOriginEthernetADPerESRefusesASTypeRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestConfiguredEVPNPerESRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestOriginEthernetADPerESUsesTypeOneRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L15) | unit/verify | revert, verified |

### [`RFC7432-8.2.1.1-1`](#rfc7432-8.2.1.1-1)

The set of Ethernet A-D routes per ES MUST carry the entire set of RTs for all the EVPN instances to which the Ethernet segment belongs (§8.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1.1-1, so no unit is bound to it.

### [`RFC7432-8.3-1`](#rfc7432-8.3-1)

In this case, the DF PE to which the CE is multihomed MUST drop the packet and not forward back to the CE. (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3-1, so no unit is bound to it.

### [`RFC7432-8.3-2`](#rfc7432-8.3-2)

This label is referred to as the ESI label and MUST be distributed by all PEs when operating in All-Active redundancy mode using a set of Ethernet A-D per ES routes, per Section 8.2.1 above. (§8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. update_build_evpn_rfc7432_test.go::TestRFC7432PerESRouteDistributesESILabel: a per-ES A-D route with the ESI Label community (flags 0, All-Active) and an RT builds an UPDATE carrying both; without the ESI Label community it is refused with ErrEVPNOrigination while the RT is present, so the RT rule cannot trip first

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7432PerESRouteDistributesESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC7432PerESRouteDistributesESILabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7432_update_build_evpn_test.go#L85) | unit/verify | revert, verified |

### [`RFC7432-8.3.1.1-6`](#rfc7432-8.3.1.1-6)

It MUST then push onto the MPLS label stack the MPLS label distributed by PE2 in the Inclusive Multicast Ethernet Tag route for VLAN1. (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-6, so no unit is bound to it.

### [`RFC7432-8.3.1.1-7`](#rfc7432-8.3.1.1-7)

If the next label is an ESI label that has not been assigned by PE2, then PE2 MUST drop the packet. (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-7, so no unit is bound to it.

### [`RFC7432-8.3.1.2-2`](#rfc7432-8.3.1.2-2)

If the next label is the ESI label assigned by PE1 to ES1 and PE3 is not connected to ES1, then PE3 MUST pop the label and flood the packet over all local ESIs in that EVPN instance. (§8.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.2-2, so no unit is bound to it.

### [`RFC7432-8.5-2`](#rfc7432-8.5-2)

In the case of VLAN-(aware) bundle service, then the numerically lowest VLAN value in that bundle on that ES MUST be used in the modulo function. (§8.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.5-2, so no unit is bound to it.

### [`RFC7432-9.2.1-6`](#rfc7432-9.2.1-6)

The encoding of an IP address MUST be either 4 octets for IPv4 or 16 octets for IPv6 (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7432Type2IPAddressOctetsShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_type2_ip_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC7432Type2IPAddressOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_type2_ip_test.go#L28) | unit/verify | revert, verified |

### [`RFC7432-10.1-5`](#rfc7432-10.1-5)

Each PE that acts as a default gateway for a given EVPN instance that receives this route and imports it as per procedures specified in this document MUST create MAC forwarding state that enables it to apply IP forwarding to the packets destined to the MAC address carried in the route. (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10.1-5, so no unit is bound to it.

### [`RFC7432-11.1-3`](#rfc7432-11.1-3)

The BGP advertisement for the Inclusive Multicast Ethernet Tag route MUST also carry one or more Route Target (RT) attributes. (§11.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. origination_test.go and bgp_routes_evpn_test.go: IMET with target:65000:100 emits the exact RT bytes; IMET with no extended community, or only a Route Origin (subtype 0x03), is refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestConfiguredEVPNIMETRefusesMissingRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L83) | unit/verify | revert, verified |
| negative | [`TestOriginInclusiveMulticastRefusesMissingRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestConfiguredEVPNIMETOriginatorIsNotNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7432_bgp_routes_evpn_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestOriginInclusiveMulticastIncludesRouteTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc7432_origination_test.go#L47) | unit/verify | revert, verified |

### [`RFC7432-11.2-5`](#rfc7432-11.2-5)

+ If the PE that originates the advertisement uses a P-multicast tree for the P-tunnel for EVPN, the PMSI Tunnel attribute MUST contain the identity of the tree (note that the PE could create the identity of the tree prior to the actual instantiation of the tree). (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-5, so no unit is bound to it.

### [`RFC7432-11.2-6`](#rfc7432-11.2-6)

In this case, in addition to carrying the identity of the tree, the PMSI Tunnel attribute MUST carry an MPLS upstream assigned label, which the PE has bound uniquely to the EVI associated with this update (as determined by its RTs). (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-6, so no unit is bound to it.

### [`RFC7432-11.2-7`](#rfc7432-11.2-7)

If the PE has already advertised Inclusive Multicast Ethernet Tag routes for two or more EVIs that it now desires to aggregate, then the PE MUST re-advertise those routes. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-7, so no unit is bound to it.

### [`RFC7432-13.1-6`](#rfc7432-13.1-6)

If a distinct P2MP LSP is used for a given Ethernet tag in the EVPN instance, then only the PEs in the Ethernet tag MUST be the leaves of the P2MP LSP. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-6, so no unit is bound to it.

### [`RFC7432-13.1-7`](#rfc7432-13.1-7)

The packet MUST be encapsulated in the P2MP LSP label stack. (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-7, so no unit is bound to it.

### [`RFC7432-17.3-3`](#rfc7432-17.3-3)

When the MAC entry on the PE ages out, the PE MUST withdraw the MAC address from BGP (§17.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-17.3-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7432.txt |
| Source fingerprint | e0c94051f8085c2a |
| Record | rfc/extraction/rfc7432.json |
| Mapped sentences | 101 |
| Declined as scope | 24 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 11 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 2 | walked | not stated |
| `6.2` | not stated | 3 | walked | not stated |
| `6.2.1` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 5 | walked | not stated |
| `6.3.1` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 1 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `7.8` | not stated | 0 | walked | not stated |
| `7.9` | not stated | 3 | walked | not stated |
| `7.10` | not stated | 0 | walked | not stated |
| `7.10.1` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 4 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 11 | walked | not stated |
| `8.2.1.1` | not stated | 2 | walked | not stated |
| `8.3` | not stated | 3 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `8.3.1.1` | not stated | 7 | walked | not stated |
| `8.3.1.2` | not stated | 6 | walked | not stated |
| `8.4` | not stated | 1 | walked | not stated |
| `8.4.1` | not stated | 4 | walked | not stated |
| `8.5` | not stated | 2 | walked | not stated |
| `8.6` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.2.1` | not stated | 6 | walked | not stated |
| `9.2.2` | not stated | 2 | walked | not stated |
| `10` | not stated | 4 | walked | not stated |
| `10.1` | not stated | 3 | walked | not stated |
| `11` | not stated | 1 | walked | not stated |
| `11.1` | not stated | 5 | walked | not stated |
| `11.2` | not stated | 8 | walked | not stated |
| `12` | not stated | 1 | walked | not stated |
| `12.1` | not stated | 2 | walked | not stated |
| `12.2` | not stated | 2 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `13.1` | not stated | 8 | walked | not stated |
| `13.2` | not stated | 0 | walked | not stated |
| `13.2.1` | not stated | 0 | walked | not stated |
| `13.2.2` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 1 | walked | not stated |
| `14.1.1` | not stated | 2 | walked | not stated |
| `14.1.2` | not stated | 3 | walked | not stated |
| `14.2` | not stated | 0 | walked | not stated |
| `14.2.1` | not stated | 0 | walked | not stated |
| `14.2.2` | not stated | 0 | walked | not stated |
| `15` | not stated | 2 | walked | not stated |
| `15.1` | not stated | 2 | walked | not stated |
| `15.2` | not stated | 1 | walked | not stated |
| `16` | not stated | 0 | walked | not stated |
| `16.1` | not stated | 0 | walked | not stated |
| `16.2` | not stated | 0 | walked | not stated |
| `16.2.1` | not stated | 0 | walked | not stated |
| `17` | not stated | 0 | walked | not stated |
| `17.1` | not stated | 0 | walked | not stated |
| `17.2` | not stated | 0 | walked | not stated |
| `17.3` | not stated | 4 | walked | not stated |
| `18` | not stated | 0 | walked | not stated |
| `19` | not stated | 0 | walked | not stated |
| `20` | not stated | 0 | walked | not stated |
| `21` | not stated | 0 | walked | not stated |
| `21.1` | not stated | 0 | walked | not stated |
| `21.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the zero Ethernet Tag ID obligation for VLAN bundle service that Section 6.1 states for VLAN-based service: "The Ethernet Tag ID in all EVPN routes MUST be set to 0." | The Ethernet Tag ID in all EVPN routes MUST be set to 0. |
| `6.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the normalized Ethernet Tag ID obligation the same section already states: "a normalized Ethernet Tag ID (VID) MUST be carried in the EVPN BGP routes." | The Ethernet Tag ID in all EVPN routes MUST be set to the normalized Ethernet Tag ID assigned by the EVPN provider. |
| `8.2.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the 10-octet Ethernet Segment Identifier that Section 5 defines and RFC7432-5-1 carries. | The Ethernet Segment Identifier MUST be a 10-octet entity as described in Section 5 ("Ethernet Segment"). |
| `8.2.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the downstream assigned ESI label that the All-Active sentence in the same section states, which RFC7432-8.2.1-4 carries. | This label MUST be a downstream assigned MPLS label if the advertising PE is using ingress replication for receiving multicast, broadcast, or unknown unicast traffic from other PEs. |
| `8.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The surrounding Section 8.3 paragraph concerns the Ethernet A-D per ES route and repeats its ESI Label requirement from Section 8.2.1. The sentence cross-references Section 8.1.1, whose Ethernet Segment route actually requires ES-Import, not ESI Label. | As described in Section 8.1.1, the route MUST carry an ESI Label extended community with a valid ESI label. |
| `8.3.1.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates, in the worked example, the obligation on the ingress PE to push the ESI label the egress PE distributed, which RFC7432-8.3.1.1-1 carries. | So, when PE1 sends a BUM packet that it receives from CE1, it MUST first push onto the MPLS label stack the ESI label that PE2 has distributed for ES1. |
| `8.3.1.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the ESI label programming obligation for the P2MP case that RFC7432-8.3.1-1 carries. | This label MUST be programmed by the other PEs that are connected to the ESI advertised in the route, in the context label space for the advertising PE. |
| `8.3.1.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the ESI label programming obligation for PEs that import the route without being connected to the ESI, which RFC7432-8.3.1-1 carries. | This label MUST also be programmed by the other PEs that import the route but are not connected to the ESI advertised in the route, in the context label space for the advertising PE. |
| `8.3.1.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates, for the P2MP case, the obligation to push the ESI label first, which RFC7432-8.3.1.1-1 carries. | When PE1 sends a BUM packet that it receives from CE1, it MUST first push onto the MPLS label stack the ESI label that it has assigned for the ESI on which the packet was received. |
| `8.3.1.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the split-horizon prohibition for the P2MP case that RFC7432-8.3.1.1-3 carries. | If the next label is the ESI label assigned by PE1 to ES1, then PE2 MUST NOT forward the packet onto ES1. |
| `8.4.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "The Route Distinguisher (RD) MUST be set per Section 7.9" defers to Section 7.9, whose obligation RFC7432-7.9-1 carries. | The Route Distinguisher (RD) MUST be set per Section 7.9. |
| `8.4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the 10-octet Ethernet Segment Identifier that Section 5 defines and RFC7432-5-1 carries. | The Ethernet Segment Identifier MUST be a 10-octet entity as described in Section 5 ("Ethernet Segment"). |
| `8.4.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the MP_REACH_NLRI next-hop obligation that RFC7432-9.2.1-2 carries. | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the IPv4 or IPv6 address of the advertising PE. |
| `9.2.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "The RD MUST be set per Section 7.9" defers to Section 7.9, whose obligation RFC7432-7.9-1 carries. | The RD MUST be set per Section 7.9. |
| `10.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the valid downstream assigned label the default gateway route must carry, which RFC7432-10.1-4 carries. | Furthermore, even if all PEs of a given EVPN instance do act as a default gateway for that EVPN instance, but only some, but not all, of these PEs have sufficient (routing) information to provide inter-subnet routing for all the inter-subnet traffic originated within the subnet associated with the EVPN instance, then when such a PE advertises in the EVPN control plane its default gateway MAC address using the MAC/IP Advertisement route and indicates that such a route is associated with the default gateway, the route MUST carry a valid downstream assigned label. |
| `11.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "The RD MUST be set per Section 7.9" defers to Section 7.9, whose obligation RFC7432-7.9-1 carries. | The RD MUST be set per Section 7.9. |
| `11.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the MP_REACH_NLRI next-hop obligation that RFC7432-9.2.1-2 carries. | The Next Hop field of the MP_REACH_NLRI attribute of the route MUST be set to the IPv4 or IPv6 address of the advertising PE. |
| `11.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the re-advertisement obligation in the preceding sentence, which RFC7432-11.2-7 carries in full. | The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute. |
| `11.2:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates, for the ingress replication case, the obligation to include the PMSI Tunnel attribute that RFC7432-11.2-1 carries. | + If the PE that originates the advertisement uses ingress replication for the P-tunnel for EVPN, the route MUST include the PMSI Tunnel attribute with the Tunnel Type set to Ingress Replication and the Tunnel Identifier set to a routable address of the PE. |
| `12.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates, for the P2MP case, the unknown unicast treatment that RFC7432-12.1-2 carries. | Further, if the MAC address is a unicast MAC address, the PE MUST treat the packet as an unknown unicast packet. |
| `13.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the ESI label encapsulation the same requirement carries: RFC7432-13.1-1 covers flooding to other PEs and the ESI label encapsulation together. | The PE MUST first encapsulate the packet in the ESI MPLS label as described in Section 8.3. |
| `14.1.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the label stack choice inside the ECMP next-hop construction that RFC7432-14.1.2-2 carries. | - If the next hop is constructed as a result of a MAC route, then this label stack MUST be used. |
| `17.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the withdrawal of Ethernet A-D per EVI routes on decommissioning that RFC7432-17.3-1 carries. | When an Ethernet tag is decommissioned on an Ethernet segment, then the PE MUST withdraw the Ethernet A-D per EVI route(s) announced for the <ESI, Ethernet tags> that are impacted by the decommissioning. |
| `17.3:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the withdrawal of the impacted MAC/IP Advertisement routes that RFC7432-17.3-1 carries. | In addition, the PE MUST also withdraw the MAC/IP Advertisement routes that are impacted by the decommissioning. |

## Superseded

No document obsoletes RFC 7432, so its obligations are stated where they were written.
