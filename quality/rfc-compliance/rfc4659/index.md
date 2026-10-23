# RFC 4659 - BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN

Partial. Every requirement this repository extracted from RFC 4659, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 3 of 18 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 2 of 18 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 18 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 75.0% | 21 of 28 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| No test at all | 22.2% | 4 of 18 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 6 | of 18 gated MUSTs judged | 1 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

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
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 28 |
| Tagged units | 28 |
| Recorded audit verdicts | 6 |
| Discrimination records | 21 |
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

The IPv4-mapped-IPv6 VPN next hop is written for every IPv4 next hop of a VPN-IPv6 route, but Ze never evaluates the IPv4-tunneling condition and no tagged recipient-wire test proves it ([`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4)); [`RFC4659-8-3`](#rfc4659-8-3) is not credited by the 48-octet global+link-local next-hop and common-subnet decision now implemented for [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3). Data-plane PE tunneling (Section 4) and integrated multi-AS ASBR options (Section 8 a/b) are not performed. [`RFC4659-8-5`](#rfc4659-8-5) remains an explicit option-(b) implementation gap: generic VPNv6 IPv6 next-hop encoding and cross-peer forwarding are implemented, but the integrated inter-ASBR label-binding/forwarding path is not. The conditional SHALL is retained without whole-requirement credit; no ASBR feature is commissioned. The widened [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1) has been independently rejudged against configured recipient-wire proof; the link-local-only unspecified-global exception remains.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC4659-3.2-1`](#rfc4659-3.2-1), [`RFC4659-3.4-1`](#rfc4659-3.4-1), [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

**Annotated (including scoped evidence) (15):** [`RFC4659-3.2-2`](#rfc4659-3.2-2), [`RFC4659-4-1`](#rfc4659-4-1), [`RFC4659-4-2`](#rfc4659-4-2), [`RFC4659-4-3`](#rfc4659-4-3), [`RFC4659-4-4`](#rfc4659-4-4), [`RFC4659-4-5`](#rfc4659-4-5), [`RFC4659-4-6`](#rfc4659-4-6), [`RFC4659-4-7`](#rfc4659-4-7), [`RFC4659-8-1`](#rfc4659-8-1), [`RFC4659-8-2`](#rfc4659-8-2), [`RFC4659-8-3`](#rfc4659-8-3), [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1), [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4), [`RFC4659-8-5`](#rfc4659-8-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4659-3.2-1` | When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75). **positive:** `unit/verify` [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23). **positive:** `unit/verify` [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47). **negative:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99). **negative:** `unit/verify` [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50). **negative:** `unit/verify` [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) |
| `RFC4659-3.2-2` | The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L607). **negative:** no negative test. **{single-polarity}:** the obligation is to SET AFI=2/SAFI=128 when advertising a VPNv6 route, so the only conforming assertion is that the emitted fields equal 2/128 and a MUST-NOT-set-other-values companion is degenerate (internal/component/bgp/message/update_build_vpn.go:221, internal/component/bgp/plugins/nlri/vpn/types.go:37) |
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
| `RFC4659-3.2.1.1-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L48). **positive:** `unit/verify` [`TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_link_local_peering_test.go#L40). **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L656). **negative:** no negative test. **{single-polarity}:** the obligation is sender emission under the operator-selected IPv6 transport policy; native configured origination now checks the zero-RD field and independently speaker-owned global IPv6 address on recipient Session output for self and explicit selection under both grouping modes. The separate link-local-peering exception remains governed by the surrounding Section 3.2.1.1 text; this is not a receiver-rejection obligation. |
| `RFC4659-3.2.1.2-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | SHALL | 3.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze writes the 24-octet zero-RD + ::ffff:a.b.c.d form whenever a VPN-IPv6 route carries an IPv4 next hop -- configured routes (internal/component/bgp/message/update_build_vpn.go vpnNextHopOctets), the API batch announce (internal/core/bgp/attribute/mpnlri.go MPReachNLRI.wireNextHop) and next-hop self or explicit on the forwarding rails (internal/component/bgp/reactor/peer_forward_facts.go precomputeVPNNextHop) -- but it chooses the form from the next hop's address family alone, never from an IPv4-tunneling transport decision, and no tagged recipient-wire test carries the requirement |
| `RFC4659-8-4` | When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** as RFC4659-3.2.1.2-1 -- the IPv4-mapped VPN-IPv6 next hop is written for every IPv4 next hop (vpnNextHopOctets, MPReachNLRI.wireNextHop, precomputeVPNNextHop), but no IPv4-tunneling transport decision selects it and no tagged recipient-wire test carries the requirement |
| `RFC4659-8-5` | When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |
| `RFC4659-3.2.1.1-2` | As a consequence, a BGP speaker that advertises a route to an internal peer may modify the Network Address of Next Hop field by removing the link-local IPv6 address of the next hop. (§3.2.1.1) | MAY | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.2.1.1-3` | The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L115). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87). **positive:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L78). **positive:** `unit/verify` [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38). **negative:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L116). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88). **negative:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L79). **positive:** `functional/verify` [`rfc4659-vpnv6-link-local-off-subnet.ci`](https://github.com/ze-software/ze/blob/main/test/encode/rfc4659-vpnv6-link-local-off-subnet.ci#L5). **positive:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L600). **negative:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L601) |
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
| [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1) When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | {gap}, no test | ze writes the 24-octet zero-RD + ::ffff:a.b.c.d form whenever a VPN-IPv6 route carries an IPv4 next hop -- configured routes (internal/component/bgp/message/update_build_vpn.go vpnNextHopOctets), the API batch announce (internal/core/bgp/attribute/mpnlri.go MPReachNLRI.wireNextHop) and next-hop self or explicit on the forwarding rails (internal/component/bgp/reactor/peer_forward_facts.go precomputeVPNNextHop) -- but it chooses the form from the next hop's address family alone, never from an IPv4-tunneling transport decision, and no tagged recipient-wire test carries the requirement |
| [`RFC4659-8-4`](#rfc4659-8-4) When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | {gap}, no test | as RFC4659-3.2.1.2-1 -- the IPv4-mapped VPN-IPv6 next hop is written for every IPv4 next hop (vpnNextHopOctets, MPReachNLRI.wireNextHop, precomputeVPNNextHop), but no IPv4-tunneling transport decision selects it and no tagged recipient-wire test carries the requirement |
| [`RFC4659-8-5`](#rfc4659-8-5) When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | {gap}, no test | the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4659-3.2-1`](#rfc4659-3.2-1)

When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Re-judged 2026-10-02 (BGP c34 judge). Both operator entry points that originate an IPv6 VPN route are proven. API: plugins/cmd/update/rfc4659_vpnv6_label_test.go, ParseUpdateText ipv6/mpls-vpn rd 65000:100 label 1000 yields the NLRI literal 88 003E81 0000FDE800000064 20010DB80001 (judge checked 1000<<4|BoS = 0x3E81, 136 bits) (positive); no label is refused with route.ErrMissingLabel and a nil result (negative). Config: bgp/config/rfc4659_vpnv6_label_test.go, YANG parse + PeersFromConfigTree keeps Labels [1000] on the VPN static route (positive; its wire form is asserted by the RFC4659-3.4-1 send unit, label 003e81 in MP_REACH), and a labelless configured route is refused at load with "requires at least one label" (negative). Records: parseVPNNLRI and patchStaticRoutes reverts, all four observed red; author probes dropping each label check red only the matching negative (scratch c34p/4659-a, 4659-b). message.BuildVPN still writes a labelless NLRI for empty Labels, but every shipped caller is guarded; its other caller is internal/chaos test tooling. HEAD vpn_test units stay as unrecorded supplementary tags. Rejudged 2026-10-17 (independent reader, not the author of 3f6bb1965e) after the owner-approved command-text change in 3f6bb1965e (injection grammar: nlri <family> became mp <family> for MP families and bare nlri for IPv4 unicast): both cmd/update units changed only the section keyword of their ParseUpdateText argument (nlri ipv6/mpls-vpn became mp ipv6/mpls-vpn); the positive still asserts the exact labeled NLRI 88 003E81 0000FDE800000064 20010DB80001 and the negative still requires route.ErrMissingLabel and a nil result, so each still fails if the label is dropped or the refusal goes. Package re-run green 2026-10-17. RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." The two records on these units are re-recorded in the same change. Verdict stands.

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
| positive | [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L607) | unit/verify | unproven |

### [`RFC4659-3.4-1`](#rfc4659-3.4-1)

In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-14 (independent auditor). Only TestOpenAdvertisesVPNv6Capability changed, mechanically: it now packs ps.Capabilities through capability.PackOptionalParams (the one packer buildOpen uses) instead of the removed buildOptionalParams, then requires Multiprotocol AFI 2/SAFI 128 in the decoded OPEN parameters for a configured ipv6/mpls-vpn family. The send and receive units in rfc4659_vpnv6_negotiated_send_test.go (route sent only when AFI 2/SAFI 128 negotiated, UPDATE refused when not) and the negotiateWith negative are unchanged and still carry the enforcement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L87) | unit/verify | revert, verified |
| negative | [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L141) | unit/verify | revert, verified |
| negative | [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L217) | unit/verify | unproven |
| positive | [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L172) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently rejudged 2026-10-22. RFC 4659 Section 3.2.1.1: "When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker." Read all three tagged carriers and producing paths. TestRFC4659ConfiguredIPv6TransportNextHopWire remains the principal sender-identity proof: native configuration creates a real Peer and recipient Session over TCP6, with self/explicit selection and both grouping modes. rfc4659RequireOwnedHostAddresses and rfc4659RefuseLocalAddress independently establish namespace ownership; the accepted endpoint supplies the self oracle, and a distinct speaker-owned loopback supplies the explicit oracle. rfc4659ReadConfiguredRoutes/rfc4659ConfiguredMPReach require AFI 2/SAFI 128, length 24, every zero-RD byte, the exact independently owned global, both literal labeled VPN NLRIs, no duplicate and completion only at VPNv6 EOR. buildMPReachVPN clears the RD and copies the selected address; vpnNextHopOctets selects the 24-byte global form for these distinct /128 fixtures. TestUpdateBuilder_BuildVPN_IPv6_NextHop remains an encoding control, not independent address-ownership proof. The changed TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair is contextual exception evidence: negotiateForwardTestFamilies now publishes identical explicit MP negotiation to Peer and Session, so the VPN family is admitted rather than absent from Session state. Its exact 48-byte zero-RD/unspecified-global/zero-RD/link-local assertion, endpoint distinction, unicast exclusion and invalid-case withdrawals are unchanged. Section 3.2.1.1 also says: "If the BGP speakers peer using only their link-local IPv6 address (for example, in the case where an IPv6 CE peers with an IPv6 PE, where the CE does not have any IPv6 global address, and where eBGP peering is achieved over the link-local addresses), the "unspecified address" ([V6ADDR]) is used by the advertising BGP speaker to indicate the absence of the global IPv6 address in the Next Hop Network Address field." applyEgressNextHopScope, vpnIPv6LinkLocalPeering and egressNextHopWithheld consume that exception on both forwarding rails; it does not license an unspecified IPv6-unicast global. Preserve the positive-only annotation: this is a conditional sender-emission obligation, not receiver rejection. Section 3.2.1 leaves transport-policy definition to the operator. Evidence remains control-plane configured origination, not tunnel creation, IPv4-mapped transport, whole 48-byte origination or PE/ASBR dataplane support. The old comment-only-change narrative is superseded by the real negotiated-family repair. No new semantic defect found; runtime renewal and stamping remain parent-owned, and no checks were run here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L656) | unit/verify | unproven |
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

Audit verdict: unimplemented (no code path enforces the requirement), stale-unit: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange moved. Re-judged 2026-10-21 (independent auditor, authored none of the change). RFC 4659 Section 8: "When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address." mpReachNextHopHandler moved in 7b0bf3a723 and f70588873f, and both hunks are confined to the noNextHop arms (noNextHop = !fam.NeedsNextHop(), FlowSpec only): a 4- or 16-octet Set is now kept for a FlowSpec redirect-to-IP target, any other width is written as the empty field, and the comment above it changed to match. The arms a next-hop family takes (the width table 4/12/16/24/32/48 and familyAdmitsNextHopOctets) are byte-identical, so a VPN-IPv6 or labeled-unicast rewrite behaves as at the last judgement. No ASBR-local label binding, inter-AS forwarding relationship or tunnel-transport-driven next-hop decision exists; buildMPReachVPN encodes the supplied next hop, checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and sysrib processEvent skips a change carrying NLRI with no valid prefix. No test is tagged. The {gap} annotation is still accurate; the gap stands and does not authorize implementation.

No test carries RFC4659-8-5, so no unit is bound to it.

### [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-19 (independent auditor, after 2a5ac0ce0b). RFC 4659 Section 3.2.1.1: "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]." and "In all other cases, a BGP speaker shall advertise to its peer in the Next Hop Network Address field only the global IPv6 address of the next hop." Since 990744517c (owner decision 2026-10-19, rfc-gap-difficulty-2026-10-18 #22) linkLocalConditionFor gives AFI 2/SAFI 128 the peer-only condition and every other family RFC 2545 Section 3's joint one; linkScope.holdsCommonSubnet answers it from peerOnLink and linkLocalNextHop still requires a speaker-owned global and a configured link-local. Paths traced: (1) configured: sendStaticRoutes/Grouped/withdraw pass route.Family() to linkLocalNextHopFor, buildStaticRouteUpdateNew -> vpnNextHopOctets/buildMPReachVPN write 24 or 48 with zero RDs, then the session gate re-applies the decision; (2) API: AnnounceNLRIBatch -> exportFilterForBody -> normalizeOrdinaryNextHop routes 24/48-octet SAFI 128 fields to normalizeVPNNextHop -> applyEgressNextHopScope(fam); (3) relayed: both forwarding rails call applyEgressNextHopScope, which raises a zero-RD 24-octet field for a speaker-owned global on the peer's subnet and trims a 48-octet pair only off it (link-local-only unspecified-global exception kept); (4) next-hop self and (5) explicit: applyLinkLocalNextHop decides the VPN pair on linkLocalConditionPeer independently of nhMode, vpnIPv6NextHop slices nhVPNPair[:48] or [:24]; (6) default route: sendDefaultOriginateRoutes always calls BuildUnicast with SAFI unset, so it emits no VPN-IPv6 route and this requirement has no default-route path (a VPN family passes the peer-only condition into a plain IPv6 body, but exportFilterForBody -> normalizeOrdinaryNextHop trims the 32-octet form on RFC 2545's joint condition). Peer.SendAnnounce has no non-test caller. Tagged units: TestRFC4659VPNv6LinkLocalLoopbackGlobal (new: loopback global off the shared /64, 48 octets on configured, relay, next-hop-self; 24 off-link), TestRFC4659StaticVPNv6LinkLocalCommonSubnet, TestRFC4659APIVPNv6LinkLocalCommonSubnet, TestRFC4659ForwardVPNv6LinkLocalCommonSubnet, TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD, test/encode/rfc4659-vpnv6-link-local-off-subnet.ci (now the 48-octet positive with the 24-octet form rejected); each asserts the exact field. Discrimination records (rfc/discrimination/rfc4659.json) are whole-body panics re-recorded by 990744517c; the checkVPNFRR tags carry none (two producers). Auditor overlay mutants (tags ze_core ze_bgp, go test -overlay), all killed: old joint rule for VPN (loopback test configured/relay/next-hop-self red, plus untagged RFC2545ReceivedPairSubnetConditions/vpn/global-off-link); never-48 (static, API, forward, rewrite, loopback tagged tests red); always-48 regardless of subnet (static, API, forward, rewrite, loopback off-link red); and the contrast mutant giving plain IPv6 the peer-only condition (RFC2545 unicast/global-off-link and second-address tests red). Baseline green. The RFC 2545 contrast is correct per its own text, Section 3: "if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to" (not judged here). Interop: 2a5ac0ce0b's checkVPNFRR positive (BIRD 2.15.1 holds Ze global plus link-local, both RDs zero) is a genuine independent receiver; its negative is NOT this requirement's negative: BIRD and FRR share the lab /64 with Ze, so the omitted link-local there is the speaker-owned/third-party gate, a neighbouring rule (RFC 2545 third-party gap), not "no common subnet". Residuals, not defects under the owner's reading: a third-party global sent to an on-link peer leaves without the speaker's link-local, which the iff's inclusion clause read alone would demand; Section 3.2.1.1 describes the global as "the global IPv6 address of the advertising BGP speaker", so the section does not contemplate it and the owner's #22 decision covers only the speaker's own global (open owner question). The .ci and the interop scenario were not re-run by this auditor. The encode fixture's common subnet is ::1/128.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L116) | unit/verify | revert, verified |
| negative | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88) | unit/verify | revert, verified |
| negative | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L601) | interop/nightly | unproven |
| positive | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87) | unit/verify | revert, verified |
| positive | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L600) | interop/nightly | unproven |
| positive | [`rfc4659-vpnv6-link-local-off-subnet.ci`](https://github.com/ze-software/ze/blob/main/test/encode/rfc4659-vpnv6-link-local-off-subnet.ci#L5) | functional/verify | revert, verified |

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
