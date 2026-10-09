# RFC 4659 - BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN

Partial. Every requirement this repository extracted from RFC 4659, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11.1% | 2 of 18 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 2 of 18 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 18 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 62.5% | 10 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 18 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 18 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 50.0% | 9 of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 18 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 18 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 27.8% | 5 of 18 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 6 | of 18 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 18 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 24 |
| Gated MUST-level | 18 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 6 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc4659.md` |
| Requirement shard | `rfc/requirements/rfc4659.md` |
| RFC text | `rfc/full/rfc4659.txt` |

## Enrolment

Enrolled: BGP-MPLS IPv6 VPN / VPNv6 (RFC 4659): labeled VPNv6 NLRI, AFI2/SAFI128 capability negotiation, and configured zero-RD global-IPv6 next-hop emission. The widened originator-address obligation has independent recipient-wire proof. IPv4-mapped-IPv6 and global-plus-link-local next-hop gaps remain explicit; data-plane PE tunneling and the integrated multi-AS ASBR service remain outside the implemented baseline. Generic VPNv6 origination and cross-peer forwarding do not establish the option-(b) service.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

VPNv6 NLRI (RD + MPLS label + IPv6 prefix) encode/decode, AFI=2/SAFI=128 capability negotiation, and the zero-RD + global-IPv6 24-octet next-hop. Native configured self and explicit IPv6 next-hop selection is checked on actual recipient Sessions under both grouping modes, against independently speaker-owned interface addresses. This is control-plane origination proof, not tunnel creation.

**What the ledger says remains**

No IPv4-mapped-IPv6 next-hop for IPv4 transport ([`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4)); no 48-octet global+link-local next-hop or common-subnet iff decision ([`RFC4659-8-3`](#rfc4659-8-3), [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)). Data-plane PE tunneling (Section 4) and integrated multi-AS ASBR options (Section 8 a/b) are not performed. [`RFC4659-8-5`](#rfc4659-8-5) remains an explicit option-(b) implementation gap: generic VPNv6 IPv6 next-hop encoding and cross-peer forwarding are implemented, but the integrated inter-ASBR label-binding/forwarding path is not. The conditional SHALL is retained without whole-requirement credit; no ASBR feature is commissioned. The widened [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1) has been independently rejudged against configured recipient-wire proof; the link-local-only unspecified-global exception remains.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4659-3.2-1`](#rfc4659-3.2-1), [`RFC4659-3.4-1`](#rfc4659-3.4-1)

**Annotated (including scoped evidence) (16):** [`RFC4659-3.2-2`](#rfc4659-3.2-2), [`RFC4659-4-1`](#rfc4659-4-1), [`RFC4659-4-2`](#rfc4659-4-2), [`RFC4659-4-3`](#rfc4659-4-3), [`RFC4659-4-4`](#rfc4659-4-4), [`RFC4659-4-5`](#rfc4659-4-5), [`RFC4659-4-6`](#rfc4659-4-6), [`RFC4659-4-7`](#rfc4659-4-7), [`RFC4659-8-1`](#rfc4659-8-1), [`RFC4659-8-2`](#rfc4659-8-2), [`RFC4659-8-3`](#rfc4659-8-3), [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1), [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4), [`RFC4659-8-5`](#rfc4659-8-5), [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4659-3.2-1` | When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75). **positive:** `unit/verify` [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23). **positive:** `unit/verify` [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47). **negative:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99). **negative:** `unit/verify` [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50). **negative:** `unit/verify` [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) |
| `RFC4659-3.2-2` | The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L597). **negative:** no negative test. **{single-polarity}:** the obligation is to SET AFI=2/SAFI=128 when advertising a VPNv6 route, so the only conforming assertion is that the emitted fields equal 2/128 and a MUST-NOT-set-other-values companion is degenerate (internal/component/bgp/message/update_build_vpn.go:221, internal/component/bgp/plugins/nlri/vpn/types.go:37) |
| `RFC4659-3.4-1` | In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L172). **positive:** `unit/verify` [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68). **positive:** `unit/verify` [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L127). **negative:** `unit/verify` [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L217). **negative:** `unit/verify` [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L87). **negative:** `unit/verify` [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L141) |
| `RFC4659-4-1` | The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an ingress-PE data-plane forwarding behavior; ze is a BGP control-plane speaker with no VPNv6 VRF-to-backbone tunneling path |
| `RFC4659-4-2` | When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane transport-selection decision for a forwarding PE; ze performs no VPNv6 data-plane forwarding, and no IPv4-mapped detection exists in the BGP path |
| `RFC4659-4-3` | When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane transport-selection decision for a forwarding PE, a role ze does not perform |
| `RFC4659-4-4` | When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane encapsulation behavior; ze installs no VPNv6 IPv4-tunnel forwarding entries |
| `RFC4659-4-5` | When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane encapsulation behavior; ze installs no VPNv6 IPv6-tunnel forwarding entries |
| `RFC4659-4-6` | When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an MPLS data-plane label-imposition behavior for a forwarding PE; ze has no VPNv6 VRF-to-LSP forwarding path |
| `RFC4659-4-7` | To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** binds the ingress-PE VPN-data-over-LDP-LSP forwarding role; ze has LDP label distribution and an MPLS FIB but performs no VPNv6 customer-data forwarding |
| `RFC4659-8-1` | The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the inter-provider option-A back-to-back-VRF ASBR role; ze has no per-VPN VRF inter-AS exchange, and the referenced RFC 2545 IPv6 next-hop wire behavior is enrolled under its own RFC |
| `RFC4659-8-2` | The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the inter-provider option-B ASBR label-swap/redistribution role; ze implements the VPNv6 NLRI and next-hop encodings but performs no inter-AS VPN ASBR redistribution |
| `RFC4659-8-3` | An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1) | MUST | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| `RFC4659-3.2.1.1-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L48). **positive:** `unit/verify` [`TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_link_local_peering_test.go#L40). **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L646). **negative:** no negative test. **{single-polarity}:** the obligation is sender emission under the operator-selected IPv6 transport policy; native configured origination now checks the zero-RD field and independently speaker-owned global IPv6 address on recipient Session output for self and explicit selection under both grouping modes. The separate link-local-peering exception remains governed by the surrounding Section 3.2.1.1 text; this is not a receiver-rejection obligation. |
| `RFC4659-3.2.1.2-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | SHALL | 3.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze constructs no IPv4-mapped-IPv6 next-hop for VPNv6 -- a plain IPv4 next-hop on a VPNv6 route emits a non-conformant 12-octet zero-RD+IPv4 next-hop, and no ::ffff:a.b.c.d mapping exists in the BGP path (internal/component/bgp/message/update_build_vpn.go:229; Is4In6 appears only in ISIS/OSPF) |
| `RFC4659-8-4` | When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same missing IPv4-mapped-IPv6 next-hop construction as RFC4659-3.2.1.2-1; ze never emits a zero-RD + ::ffff:a.b.c.d VPNv6 next-hop (internal/component/bgp/message/update_build_vpn.go:246; no IPv4-mapped VPNv6 next-hop producer) |
| `RFC4659-8-5` | When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |
| `RFC4659-3.2.1.1-2` | As a consequence, a BGP speaker that advertises a route to an internal peer may modify the Network Address of Next Hop field by removing the link-local IPv6 address of the next hop. (§3.2.1.1) | MAY | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.2.1.1-3` | The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildMPReachVPN in internal/component/bgp/message/update_build_vpn.go has no 48-octet global-plus-link-local encoder or common-subnet emission decision; the iff behavior remains unmet. |
| `RFC4659-1-1` | Where both IPv4 VPNs and IPv6 VPN services are supported over an IPv4 core, the same single set of MP-BGP peering relationships and the same single PE-PE tunnel mesh MAY be used for both. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-1` | When a site is IPv4 capable and IPv6 capable, the same RD MAY be used for the advertisement of IPv6 addresses and IPv4 addresses. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-2` | Alternatively, a different RD MAY be used for the advertisement of the IPv4 addresses and of the IPv6 addresses. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.1-1` | For IPv6 VPN, the iBGP connections MAY be over IPv4 or over IPv6. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-4-8` | The ingress PE MAY optionally allow, through explicit configuration, the use of IPv6 tunneling when the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4- mapped IPv6 address. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4659-4-1`](#rfc4659-4-1) The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: this is an ingress-PE data-plane forwarding behavior; ze is a BGP control-plane speaker with no VPNv6 VRF-to-backbone tunneling path |
| [`RFC4659-4-2`](#rfc4659-4-2) When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane transport-selection decision for a forwarding PE; ze performs no VPNv6 data-plane forwarding, and no IPv4-mapped detection exists in the BGP path |
| [`RFC4659-4-3`](#rfc4659-4-3) When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane transport-selection decision for a forwarding PE, a role ze does not perform |
| [`RFC4659-4-4`](#rfc4659-4-4) When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane encapsulation behavior; ze installs no VPNv6 IPv4-tunnel forwarding entries |
| [`RFC4659-4-5`](#rfc4659-4-5) When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane encapsulation behavior; ze installs no VPNv6 IPv6-tunnel forwarding entries |
| [`RFC4659-4-6`](#rfc4659-4-6) When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: an MPLS data-plane label-imposition behavior for a forwarding PE; ze has no VPNv6 VRF-to-LSP forwarding path |
| [`RFC4659-4-7`](#rfc4659-4-7) To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: binds the ingress-PE VPN-data-over-LDP-LSP forwarding role; ze has LDP label distribution and an MPLS FIB but performs no VPNv6 customer-data forwarding |
| [`RFC4659-8-1`](#rfc4659-8-1) The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8) | no test | no test carries this requirement id; annotated {not-applicable}: the inter-provider option-A back-to-back-VRF ASBR role; ze has no per-VPN VRF inter-AS exchange, and the referenced RFC 2545 IPv6 next-hop wire behavior is enrolled under its own RFC |
| [`RFC4659-8-2`](#rfc4659-8-2) The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8) | no test | no test carries this requirement id; annotated {not-applicable}: the inter-provider option-B ASBR label-swap/redistribution role; ze implements the VPNv6 NLRI and next-hop encodings but performs no inter-AS VPN ASBR redistribution |
| [`RFC4659-8-3`](#rfc4659-8-3) An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1) | {gap}, no test | ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1) When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | {gap}, no test | ze constructs no IPv4-mapped-IPv6 next-hop for VPNv6 -- a plain IPv4 next-hop on a VPNv6 route emits a non-conformant 12-octet zero-RD+IPv4 next-hop, and no ::ffff:a.b.c.d mapping exists in the BGP path (internal/component/bgp/message/update_build_vpn.go:229; Is4In6 appears only in ISIS/OSPF) |
| [`RFC4659-8-4`](#rfc4659-8-4) When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | {gap}, no test | the same missing IPv4-mapped-IPv6 next-hop construction as RFC4659-3.2.1.2-1; ze never emits a zero-RD + ::ffff:a.b.c.d VPNv6 next-hop (internal/component/bgp/message/update_build_vpn.go:246; no IPv4-mapped VPNv6 next-hop producer) |
| [`RFC4659-8-5`](#rfc4659-8-5) When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | {gap}, no test | the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |
| [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3) The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1) | {gap}, no test | buildMPReachVPN in internal/component/bgp/message/update_build_vpn.go has no 48-octet global-plus-link-local encoder or common-subnet emission decision; the iff behavior remains unmet. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4659-3.2-1`](#rfc4659-3.2-1)

When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c34 judge). Both operator entry points that originate an IPv6 VPN route are proven. API: plugins/cmd/update/rfc4659_vpnv6_label_test.go, ParseUpdateText ipv6/mpls-vpn rd 65000:100 label 1000 yields the NLRI literal 88 003E81 0000FDE800000064 20010DB80001 (judge checked 1000<<4|BoS = 0x3E81, 136 bits) (positive); no label is refused with route.ErrMissingLabel and a nil result (negative). Config: bgp/config/rfc4659_vpnv6_label_test.go, YANG parse + PeersFromConfigTree keeps Labels [1000] on the VPN static route (positive; its wire form is asserted by the RFC4659-3.4-1 send unit, label 003e81 in MP_REACH), and a labelless configured route is refused at load with "requires at least one label" (negative). Records: parseVPNNLRI and patchStaticRoutes reverts, all four observed red; author probes dropping each label check red only the matching negative (scratch c34p/4659-a, 4659-b). message.BuildVPN still writes a labelless NLRI for empty Labels, but every shipped caller is guarded; its other caller is internal/chaos test tooling. HEAD vpn_test units stay as unrecorded supplementary tags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99) | unit/verify | revert, verified |
| negative | [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) | unit/verify | unproven |
| positive | [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47) | unit/verify | unproven |

### [`RFC4659-3.2-2`](#rfc4659-3.2-2)

The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: MP_REACH for a VPNv6 route with AFI != 2 or SAFI != 128. internal/component/bgp/message/update_build_test.go::TestUpdateBuilder_BuildVPN_IPv6: afi != 2 and value[2] != 128 are Errorf. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L597) | unit/verify | unproven |

### [`RFC4659-3.4-1`](#rfc4659-3.4-1)

In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c34 judge). DEFECT fixed (D-8): sendStaticRoutes sent every configured route whatever the session negotiated. Fix at the producer, judge-read: Peer.negotiatedStaticRoutes (reactor/peer_static_wire.go, RFC 4659 Section 3.4 quote above it) filters by routeFamily against the negotiated set at the top of sendStaticRoutes, so the per-route path, the grouped path and the reload delta (deliverStaticRouteDelta) are all covered; a skipped route is not in the answer, so it is not recorded in staticWire, a later reload never withdraws it (withdrawStaticRoutes reads only staticWire), and a new session with the family negotiated re-sends it from config. Implicit IPv4 unicast (no MP capability on either side) stays negotiated (capability.Negotiate FamilyImplicit), so plain IPv4 peers are unaffected. Send: rfc4659_vpnv6_negotiated_send_test.go, a session negotiating 2/128 carries MP_REACH 800e2f 000280 18 RD0 2001:db8::1 00 88 003e81 0000fde800000064 20010db80001 (positive); an ipv4-only session puts no UPDATE naming 2/128 on the wire, route or End-of-RIB (negative; failing-first observed before the fix, scratch c34p/4659s-green.log, and the filter-disabled probe reds only this unit). Receive: validateUpdateFamilies accepts a 2/128 MP_REACH on a session that negotiated it and refuses it with ErrFamilyNotNegotiated on one that did not. Records: negotiatedStaticRoutes and validateUpdateFamilies reverts, all four observed red. HEAD session_negotiate_test units stay as unrecorded supplementary tags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L87) | unit/verify | revert, verified |
| negative | [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L141) | unit/verify | revert, verified |
| negative | [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L217) | unit/verify | unproven |
| positive | [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L172) | unit/verify | unproven |

### [`RFC4659-4-1`](#rfc4659-4-1)

The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-1, so no unit is bound to it.

### [`RFC4659-4-2`](#rfc4659-4-2)

When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-2, so no unit is bound to it.

### [`RFC4659-4-3`](#rfc4659-4-3)

When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-3, so no unit is bound to it.

### [`RFC4659-4-4`](#rfc4659-4-4)

When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-4, so no unit is bound to it.

### [`RFC4659-4-5`](#rfc4659-4-5)

When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-5, so no unit is bound to it.

### [`RFC4659-4-6`](#rfc4659-4-6)

When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-6, so no unit is bound to it.

### [`RFC4659-4-7`](#rfc4659-4-7)

To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-7, so no unit is bound to it.

### [`RFC4659-8-1`](#rfc4659-8-1)

The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-1, so no unit is bound to it.

### [`RFC4659-8-2`](#rfc4659-8-2)

The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-2, so no unit is bound to it.

### [`RFC4659-8-3`](#rfc4659-8-3)

An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-3, so no unit is bound to it.

### [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1)

When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker. (§3.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged the full widened RFC 4659 Section 3.2.1.1 obligation. TestRFC4659ConfiguredIPv6TransportNextHopWire in internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go closes the former originator-identity proof gap: native configuration starts the real Peer and recipient Session over TCP6, with self and explicit selection under both grouping modes. Independent interface enumeration in separate namespaces establishes speaker ownership; the accepted endpoint supplies the self oracle and a distinct speaker-owned loopback supplies the explicit oracle. Literal recipient assertions require AFI 2/SAFI 128, length 24, every RD byte zero, the independently established speaker global address, both intended VPN NLRIs and completion at VPNv6 EOR. buildMPReachVPN is reached through configured static sending rather than called by the integration test. The earlier builder test remains an encoding control, and TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair remains contextual exception evidence. Section 3.2.1 leaves transport-policy definition to the operator; this tests its existing configured control-plane expression, not tunnel creation. Distinct /128s exclude the separate common-subnet/48-octet origination obligation. IPv4-mapped transport, PE dataplane and ASBR gaps remain unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L646) | unit/verify | unproven |
| positive | [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_link_local_peering_test.go#L40) | unit/verify | revert, verified |

### [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1)

When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-3.2.1.2-1, so no unit is bound to it.

### [`RFC4659-8-4`](#rfc4659-8-4)

When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-4, so no unit is bound to it.

### [`RFC4659-8-5`](#rfc4659-8-5)

When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independently judged RFC 4659 Section 8 in its option-(b) context: When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. The SHALL remains binding when that condition holds; BGP peering may use either transport family, and Section 3.2.1 leaves the transport policy to the operator. Ze has reachable generic VPNv6 origination and cross-peer forwarding: buildMPReachVPN encodes a supplied address and labels, while mpReachNextHopHandler preserves the received NLRI when replacing the next hop. Neither supplies an ASBR-local label binding or the integrated inter-AS forwarding relationship. checkRouteBestChange excludes native VPN routes from Loc-RIB mirroring, and sysrib.processEvent does not turn their native-NLRI publications into FIB entries; the separate LDP/RSVP-TE MPLS channel is not a BGP option-(b) binding producer. Retain the explicit option-(b) implementation gap under the existing no-ASBR-feature scope. Do not infer non-applicability from missing ASBR names or full enforcement from the generic IPv6 encoder or Section 3.2.1.1 origination test. No ASBR implementation is commissioned.

No test carries RFC4659-8-5, so no unit is bound to it.

### [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Independent first judgment of RFC 4659 Section 3.2.1.1: The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. Both directions remain SHALL obligations. buildMPReachVPN emits only one RD plus one address, using length 24 for an IPv6 next hop; it has no 48-octet two-VPN-address emission or recipient common-subnet decision. buildStaticRouteUpdateNew receives a linkLocal argument but its VPN branch does not pass it into VPNParams or BuildVPN. Consequently the required inclusion when the speaker and recipient share a common subnet is unimplemented. Always omitting the link-local address can satisfy the off-subnet exclusion case but does not implement the full iff. No tagged test proves this row. RFC2545 plain-unicast pair tests, preservation of the special received link-local-only VPN pair, and the configured global-only VPNv6 integration test do not establish this missing VPN origination behavior; the integration fixture deliberately uses distinct /128s. Retain the separate internal-peer stripping MAY and link-local-only unspecified-global exception without using either to erase the condition. The existing 48-octet/common-subnet implementation gap remains explicit; no new feature is commissioned.

No test carries RFC4659-3.2.1.1-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc4659.txt |
| Source fingerprint | 675026b53c92cac3 |
| Record | rfc/extraction/rfc4659.json |
| Mapped sentences | 16 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `3.2.1.1` | not stated | 1 | walked | not stated |
| `3.2.1.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `4` | not stated | 7 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 4 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |
| `16` | not stated | 0 | walked | not stated |
| `16.1` | not stated | 0 | walked | not stated |
| `16.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 4659 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4659, so its obligations are stated where they were written.
