# RFC 4659 - BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN

VPNv6. Every requirement this repository extracted from RFC 4659, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 18 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 18 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 18 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 18 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 21 | 21 of 28 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 18 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 18 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 18 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 18 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 18 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 13 | of 18 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 6 | of 18 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 9 counts marked as a part above are the whole of the 18 gated MUSTs: they add up to 18. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | VPNv6 |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
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

**Area:** VPNv6

**Proof:** 4 of 18 MUST proven

**Evidence:** interop 1, unit 3

**Unmet:** RFC4659-3.2.1.2-1, RFC4659-4-1, RFC4659-4-2, RFC4659-4-3, RFC4659-4-4, RFC4659-4-5, RFC4659-4-6, RFC4659-4-7, RFC4659-8-1, RFC4659-8-2, RFC4659-8-3, RFC4659-8-4, RFC4659-8-5


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

## Figures

- **MUST bar**: Ze proven 4, Ze correctness gap 14
- **Overview bar**: Ze MUST proven 4, SHOULD not decided 6, Ze correctness gap 14

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 18 |
| MUST rows kept (the MUST bar) | 18 |
| Ze proven | 4 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 14 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 6 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc4659.json](../../../data/rfc/rfc4659.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4659-3.2-1` | When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75). **positive:** `unit/verify` [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23). **positive:** `unit/verify` [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47). **negative:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99). **negative:** `unit/verify` [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50). **negative:** `unit/verify` [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) |
| `RFC4659-3.2-2` | The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L607). **negative:** no negative test. **{single-polarity}:** the obligation is to SET AFI=2/SAFI=128 when advertising a VPNv6 route, so the only conforming assertion is that the emitted fields equal 2/128 and a MUST-NOT-set-other-values companion is degenerate (internal/component/bgp/message/update_build_vpn.go:221, internal/component/bgp/plugins/nlri/vpn/types.go:37) |
| `RFC4659-3.4-1` | In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L172). **positive:** `unit/verify` [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68). **positive:** `unit/verify` [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L127). **negative:** `unit/verify` [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L217). **negative:** `unit/verify` [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L87). **negative:** `unit/verify` [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L141) |
| `RFC4659-4-1` | The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tunnels no IPv6 VPN data over the backbone. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-2` | When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze selects no tunnel for VPN data by the next-hop encoding, IPv4-mapped or not. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-3` | When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze selects no tunnel for VPN data by the next-hop encoding. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-4` | When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no IPv4 tunnel header for VPN data. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-5` | When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no IPv6 tunnel header for VPN data. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-6` | When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze pushes no LSP tunnel label onto a labeled IPv6 VPN packet. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-4-7` | To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs LDP (internal/plugins/ldp) and has an MPLS FIB, but carries no VPN data over an LDP LSP. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-8-1` | The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no VRF, so it has no back-to-back VRF exchange of IPv6 routes for option (a); its plain IPv6 route exchange is judged in rfc/short/rfc2545.md. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-8-2` | The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze exchanges labeled VPN-IPv6 routes over BGP, but as an ASBR it allocates no label and installs no label swap for a VPN route it readvertises with its own next hop. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| `RFC4659-8-3` | An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1) | MUST | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| `RFC4659-3.2.1.1-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L54). **positive:** `unit/verify` [`TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_link_local_peering_test.go#L40). **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L656). **negative:** no negative test. **{single-polarity}:** the obligation is sender emission under the operator-selected IPv6 transport policy; native configured origination now checks the zero-RD field and independently speaker-owned global IPv6 address on recipient Session output for self and explicit selection under both grouping modes. The separate link-local-peering exception remains governed by the surrounding Section 3.2.1.1 text; this is not a receiver-rejection obligation. |
| `RFC4659-3.2.1.2-1` | When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | SHALL | 3.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze writes the 24-octet zero-RD + ::ffff:a.b.c.d form whenever a VPN-IPv6 route carries an IPv4 next hop -- configured routes (internal/component/bgp/message/update_build_vpn.go vpnNextHopOctets), the API batch announce (internal/core/bgp/attribute/mpnlri.go MPReachNLRI.wireNextHop) and next-hop self or explicit on the forwarding rails (internal/component/bgp/reactor/peer_forward_facts.go precomputeVPNNextHop) -- but it chooses the form from the next hop's address family alone, never from an IPv4-tunneling transport decision, and no tagged recipient-wire test carries the requirement |
| `RFC4659-8-4` | When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** as RFC4659-3.2.1.2-1 -- the IPv4-mapped VPN-IPv6 next hop is written for every IPv4 next hop (vpnNextHopOctets, MPReachNLRI.wireNextHop, precomputeVPNNextHop), but no IPv4-tunneling transport decision selects it and no tagged recipient-wire test carries the requirement |
| `RFC4659-8-5` | When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |
| `RFC4659-3.2.1.1-2` | As a consequence, a BGP speaker that advertises a route to an internal peer may modify the Network Address of Next Hop field by removing the link-local IPv6 address of the next hop. (§3.2.1.1) | MAY | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.2.1.1-3` | The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L120). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87). **positive:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L83). **positive:** `unit/verify` [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38). **negative:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L121). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88). **negative:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L84). **positive:** `functional/verify` [`rfc4659-vpnv6-link-local-off-subnet.ci`](https://github.com/ze-software/ze/blob/main/test/encode/rfc4659-vpnv6-link-local-off-subnet.ci#L5). **positive:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L602). **negative:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L603) |
| `RFC4659-1-1` | Where both IPv4 VPNs and IPv6 VPN services are supported over an IPv4 core, the same single set of MP-BGP peering relationships and the same single PE-PE tunnel mesh MAY be used for both. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-1` | When a site is IPv4 capable and IPv6 capable, the same RD MAY be used for the advertisement of IPv6 addresses and IPv4 addresses. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-2` | Alternatively, a different RD MAY be used for the advertisement of the IPv4 addresses and of the IPv6 addresses. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.1-1` | For IPv6 VPN, the iBGP connections MAY be over IPv4 or over IPv6. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-4-8` | The ingress PE MAY optionally allow, through explicit configuration, the use of IPv6 tunneling when the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4- mapped IPv6 address. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4659-4-1`](#rfc4659-4-1) The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | {gap}, no test | ze tunnels no IPv6 VPN data over the backbone. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-2`](#rfc4659-4-2) When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4) | {gap}, no test | ze selects no tunnel for VPN data by the next-hop encoding, IPv4-mapped or not. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-3`](#rfc4659-4-3) When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4) | {gap}, no test | ze selects no tunnel for VPN data by the next-hop encoding. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-4`](#rfc4659-4-4) When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4) | {gap}, no test | ze builds no IPv4 tunnel header for VPN data. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-5`](#rfc4659-4-5) When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4) | {gap}, no test | ze builds no IPv6 tunnel header for VPN data. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-6`](#rfc4659-4-6) When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4) | {gap}, no test | ze pushes no LSP tunnel label onto a labeled IPv6 VPN packet. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-4-7`](#rfc4659-4-7) To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4) | {gap}, no test | ze runs LDP (internal/plugins/ldp) and has an MPLS FIB, but carries no VPN data over an LDP LSP. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-8-1`](#rfc4659-8-1) The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8) | {gap}, no test | ze holds no VRF, so it has no back-to-back VRF exchange of IPv6 routes for option (a); its plain IPv6 route exchange is judged in rfc/short/rfc2545.md. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-8-2`](#rfc4659-8-2) The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8) | {gap}, no test | ze exchanges labeled VPN-IPv6 routes over BGP, but as an ASBR it allocates no label and installs no label swap for a VPN route it readvertises with its own next hop. ze fills the PE role on the control plane only: it originates VPN-IPv6 routes from configuration (internal/component/bgp/reactor/peer_static_routes.go through internal/component/bgp/message/update_build_vpn.go::BuildVPN), but holds no VRF: internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange keeps VPN routes out of the Loc-RIB that feeds the FIB because ze has no VRF plumbing, and the interface vrf leaf is refused as unimplemented (internal/component/iface/yang/ze-iface-conf.yang). No owner decision declines the VPN data plane, so this is owed with it |
| [`RFC4659-8-3`](#rfc4659-8-3) An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1) | {gap}, no test | ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1) When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2) | {gap}, no test | ze writes the 24-octet zero-RD + ::ffff:a.b.c.d form whenever a VPN-IPv6 route carries an IPv4 next hop -- configured routes (internal/component/bgp/message/update_build_vpn.go vpnNextHopOctets), the API batch announce (internal/core/bgp/attribute/mpnlri.go MPReachNLRI.wireNextHop) and next-hop self or explicit on the forwarding rails (internal/component/bgp/reactor/peer_forward_facts.go precomputeVPNNextHop) -- but it chooses the form from the next hop's address family alone, never from an IPv4-tunneling transport decision, and no tagged recipient-wire test carries the requirement |
| [`RFC4659-8-4`](#rfc4659-8-4) When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8) | {gap}, no test | as RFC4659-3.2.1.2-1 -- the IPv4-mapped VPN-IPv6 next hop is written for every IPv4 next hop (vpnNextHopOctets, MPReachNLRI.wireNextHop, precomputeVPNNextHop), but no IPv4-tunneling transport decision selects it and no tagged recipient-wire test carries the requirement |
| [`RFC4659-8-5`](#rfc4659-8-5) When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8) | {gap}, no test | the integrated option-(b) inter-ASBR service path remains unimplemented under the existing no-ASBR-feature scope. Generic VPNv6 origination and cross-peer forwarding exist: internal/component/bgp/message/update_build_vpn.go::buildMPReachVPN encodes the supplied next hop and labels, and internal/component/bgp/reactor/filter_delta_handlers.go::mpReachNextHopHandler retains the received NLRI during a next-hop rewrite. These do not supply an ASBR-local label binding or inter-AS forwarding relationship; internal/component/bgp/plugins/rib/rib_bestchange.go::checkRouteBestChange does not mirror native VPN routes to the Loc-RIB, and internal/component/sysrib/sysrib.go::processEvent does not turn their native-NLRI publications into FIB entries. The SHALL remains applicable when its IPv6-transport condition holds; this gap is not an exemption or permission to develop ASBR capability. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4659-3.2-1`](#rfc4659-3.2-1)

When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 (independent auditor, authored none of dfe4e3aa8d). RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." dfe4e3aa8d dropped the SAFI argument of parseVPNAnnounce, which now calls parseVPN(afi, SAFIVPN, data, addpath, false), the value the test passed before; TestVPNv6WireRoundTrip keeps every assertion (labels equal, NotEmpty, label 200). The negative and the config and API carriers are unchanged. Scoped test green 2026-10-31. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." 238bdb62e9 renamed the parser the unit calls, ParseVPN to parseVPNAnnounce, which is parseVPN(afi, safi, data, addpath, withdraw=false): the announce layout the old call read, labels included. TestVPNv6WireRoundTrip still round-trips an IPv6 VPN route with its label stack and asserts the label, and the labelless refusal carriers are unchanged, so both polarities stand. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Re-judged 2026-10-02 (BGP c34 judge). Both operator entry points that originate an IPv6 VPN route are proven. API: plugins/cmd/update/rfc4659_vpnv6_label_test.go, ParseUpdateText ipv6/mpls-vpn rd 65000:100 label 1000 yields the NLRI literal 88 003E81 0000FDE800000064 20010DB80001 (judge checked 1000<<4|BoS = 0x3E81, 136 bits) (positive); no label is refused with route.ErrMissingLabel and a nil result (negative). Config: bgp/config/rfc4659_vpnv6_label_test.go, YANG parse + PeersFromConfigTree keeps Labels [1000] on the VPN static route (positive; its wire form is asserted by the RFC4659-3.4-1 send unit, label 003e81 in MP_REACH), and a labelless configured route is refused at load with "requires at least one label" (negative). Records: parseVPNNLRI and patchStaticRoutes reverts, all four observed red; author probes dropping each label check red only the matching negative (scratch c34p/4659-a, 4659-b). message.BuildVPN still writes a labelless NLRI for empty Labels, but every shipped caller is guarded; its other caller is internal/chaos test tooling. HEAD vpn_test units stay as unrecorded supplementary tags. Rejudged 2026-10-17 (independent reader, not the author of 3f6bb1965e) after the owner-approved command-text change in 3f6bb1965e (injection grammar: nlri <family> became mp <family> for MP families and bare nlri for IPv4 unicast): both cmd/update units changed only the section keyword of their ParseUpdateText argument (nlri ipv6/mpls-vpn became mp ipv6/mpls-vpn); the positive still asserts the exact labeled NLRI 88 003E81 0000FDE800000064 20010DB80001 and the negative still requires route.ErrMissingLabel and a nil result, so each still fails if the label is dropped or the refusal goes. Package re-run green 2026-10-17. RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." The two records on these units are re-recorded in the same change. Verdict stands.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor (authored none of the userns spec commits d8427e4380, b00e3c6710, 68b723ec4f, 7d11a51269). userns.Enter (internal/test/userns/userns_linux.go) re-executes the one test in a new user+net+mount namespace and returns false in the parent only after the child's own PASS line; a child failure, a zero-exit failure or a child that selected nothing fails the parent, and a child skip skips it, so the guard cannot turn a red into a green. Section 3.2.1.1: "When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker." TestRFC4659ConfiguredIPv6TransportNextHopWire changed by the guard only; the fixture's namespace refusal now fails instead of skipping, and rfc4659ConfiguredSettings gained `filter { export { final accept; } }` because the recipient is EBGP and RFC 8212 default-reject would otherwise keep the routes out of Adj-RIB-Out. That is an operator-path policy, not a relaxation: every assertion in rfc4659ReadConfiguredRoutes (exact zero RD, speaker-owned global for self, selected address for explicit, both grouping modes over TCP6 into a real Session) is unchanged. TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair unchanged. {single-polarity: positive} stays correct: sender emission, not receiver rejection. Control-plane evidence only, not tunnel creation. Record (buildMPReachVPN revert) re-taken in 7d11a51269.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L656) | unit/verify | unproven |
| positive | [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L54) | unit/verify | revert, verified |
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

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 4659 Section 8: "When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address." 238bdb62e9 changed the cited producer rib_bestchange.go::checkRouteBestChange: a best change is now also published when only the winner's attribute set changes (sameAttributeSet over the storage fingerprint), and the entry carries the winner's attribute bytes (storedAttributes, setEntryPath). Its only consumer of those bytes is BMP Loc-RIB (bmp/bmp_locrib.go). Native VPN routes are still non-CIDR and not mirrored by prefix, and nothing builds an ASBR-local label binding or selects IPv6 tunnel transport, so the gap stands. Verdict unchanged: unimplemented. Earlier judgement follows. RFC 4659 Section 8, approach (b), rfc/full/rfc4659.txt:644-664: "When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address." No tagged carriers exist for this row. Independently read all four retained guard producers. buildMPReachVPN writes a supplied IPv6 next hop behind a zero RD and carries supplied labels; mpReachNextHopHandler preserves native NLRI while enforcing admitted next-hop lengths. Those generic control-plane facilities are implemented but unverified for this integrated transport-conditioned requirement, not absent generic VPNv6 support. checkRouteBestChange still excludes native VPN routes from its prefix-keyed Loc-RIB mirror, extracting labeled-unicast labels or separate attribute-40 SRv6 metadata rather than creating an ASBR-local VPN label binding; sysrib.processEvent skips native-NLRI changes without a valid prefix. The integrated option-(b) inter-ASBR label/forwarding relationship and IPv6-transport selection remain absent. Retain the explicit unimplemented boundary and every guard; the SHALL remains applicable when its transport condition holds. This is neither not-applicable nor permission to add ASBR scope. No checks run.

No test carries RFC4659-8-5, so no unit is bound to it.

### [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 (independent auditor, authored none of 8b062b45a8 or be95c10053). RFC 4659 Section 3.2.1.1: "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]." and "In all other cases, a BGP speaker shall advertise to its peer in the Next Hop Network Address field only the global IPv6 address of the next hop." 8b062b45a8 renamed egressSpeakerLinkLocal to egressLinkLocal and moved the configured-link-local checks into linkScope.linkLocalNextHop's nextHopRouterSpeaker case; that function now runs the global validation and holdsCommonSubnet(global, condition) first, then switches on the router. For AFI 2/SAFI 128 (linkLocalConditionPeer) the outcome is unchanged: the speaker's own global on an on-link peer still gets the configured link-local, and the new nextHopRouterThirdParty branch returns the zero Addr unless the condition is RFC 2545's linkLocalConditionNextHopAndPeer, so the neighbor-table lookup (thirdPartyLinkLocal) never feeds a VPN field. 7d5e28aaa5 and 85febd7f6c only thread the neighbor watch state and a once-per-peer warning flag; peer_forward_facts.go changes are the RS relay grants and the neighbor view parameter, not precomputeVPNNextHop, vpnIPv6NextHop or applyFactsNextHop; egress_inject_filter.go exportFilterForBody gained the RFC 8212 pre-publication branch, which suppresses or withdraws but never rewrites the next hop. Tagged units: the only change is be95c10053 giving vpnLLPeer an accepting export policy (FinalAccept) because the recipient is EBGP and RFC 8212 Section 3 would otherwise reject every route; the static, API, forward, rewrite and loopback tests keep their exact 48-octet and 24-octet field assertions, checkVPNFRR and the .ci are byte-unchanged. Discrimination: d9263b0f50 re-recorded both TestRFC4659ForwardVPNv6LinkLocalCommonSubnet records on egressLinkLocal (observed red). Scoped go test -race -run 'TestRFC4659|TestVPNv6' on reactor and nlri/vpn green 2026-10-31 (darwin: the linux-only integration file not run). Verdict unchanged: enforced. The residual stands as before: a third-party global sent to an on-link peer leaves without any link-local, which Section 3.2.1.1's description of the global as the advertising speaker's does not contemplate (open owner question). Earlier judgement follows. Re-judged 2026-10-19 (independent auditor, after 2a5ac0ce0b). RFC 4659 Section 3.2.1.1: "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]." and "In all other cases, a BGP speaker shall advertise to its peer in the Next Hop Network Address field only the global IPv6 address of the next hop." Since 990744517c (owner decision 2026-10-19, rfc-gap-difficulty-2026-10-18 #22) linkLocalConditionFor gives AFI 2/SAFI 128 the peer-only condition and every other family RFC 2545 Section 3's joint one; linkScope.holdsCommonSubnet answers it from peerOnLink and linkLocalNextHop still requires a speaker-owned global and a configured link-local. Paths traced: (1) configured: sendStaticRoutes/Grouped/withdraw pass route.Family() to linkLocalNextHopFor, buildStaticRouteUpdateNew -> vpnNextHopOctets/buildMPReachVPN write 24 or 48 with zero RDs, then the session gate re-applies the decision; (2) API: AnnounceNLRIBatch -> exportFilterForBody -> normalizeOrdinaryNextHop routes 24/48-octet SAFI 128 fields to normalizeVPNNextHop -> applyEgressNextHopScope(fam); (3) relayed: both forwarding rails call applyEgressNextHopScope, which raises a zero-RD 24-octet field for a speaker-owned global on the peer's subnet and trims a 48-octet pair only off it (link-local-only unspecified-global exception kept); (4) next-hop self and (5) explicit: applyLinkLocalNextHop decides the VPN pair on linkLocalConditionPeer independently of nhMode, vpnIPv6NextHop slices nhVPNPair[:48] or [:24]; (6) default route: sendDefaultOriginateRoutes always calls BuildUnicast with SAFI unset, so it emits no VPN-IPv6 route and this requirement has no default-route path (a VPN family passes the peer-only condition into a plain IPv6 body, but exportFilterForBody -> normalizeOrdinaryNextHop trims the 32-octet form on RFC 2545's joint condition). Peer.SendAnnounce has no non-test caller. Tagged units: TestRFC4659VPNv6LinkLocalLoopbackGlobal (new: loopback global off the shared /64, 48 octets on configured, relay, next-hop-self; 24 off-link), TestRFC4659StaticVPNv6LinkLocalCommonSubnet, TestRFC4659APIVPNv6LinkLocalCommonSubnet, TestRFC4659ForwardVPNv6LinkLocalCommonSubnet, TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD, test/encode/rfc4659-vpnv6-link-local-off-subnet.ci (now the 48-octet positive with the 24-octet form rejected); each asserts the exact field. Discrimination records (rfc/discrimination/rfc4659.json) are whole-body panics re-recorded by 990744517c; the checkVPNFRR tags carry none (two producers). Auditor overlay mutants (tags ze_core ze_bgp, go test -overlay), all killed: old joint rule for VPN (loopback test configured/relay/next-hop-self red, plus untagged RFC2545ReceivedPairSubnetConditions/vpn/global-off-link); never-48 (static, API, forward, rewrite, loopback tagged tests red); always-48 regardless of subnet (static, API, forward, rewrite, loopback off-link red); and the contrast mutant giving plain IPv6 the peer-only condition (RFC2545 unicast/global-off-link and second-address tests red). Baseline green. The RFC 2545 contrast is correct per its own text, Section 3: "if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to" (not judged here). Interop: 2a5ac0ce0b's checkVPNFRR positive (BIRD 2.15.1 holds Ze global plus link-local, both RDs zero) is a genuine independent receiver; its negative is NOT this requirement's negative: BIRD and FRR share the lab /64 with Ze, so the omitted link-local there is the speaker-owned/third-party gate, a neighbouring rule (RFC 2545 third-party gap), not "no common subnet". Residuals, not defects under the owner's reading: a third-party global sent to an on-link peer leaves without the speaker's link-local, which the iff's inclusion clause read alone would demand; Section 3.2.1.1 describes the global as "the global IPv6 address of the advertising BGP speaker", so the section does not contemplate it and the owner's #22 decision covers only the speaker's own global (open owner question). The .ci and the interop scenario were not re-run by this auditor. The encode fixture's common subnet is ::1/128.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88) | unit/verify | revert, verified |
| negative | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L603) | interop/nightly | unproven |
| positive | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87) | unit/verify | revert, verified |
| positive | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L602) | interop/nightly | unproven |
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
