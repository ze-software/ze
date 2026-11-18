# RFC 4659 - BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN

VPNv6. Every requirement listed for RFC 4659, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 18 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 18 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 18 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 18 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 13 | 13 of 28 tests linked to requirements, 0 without a fault test and 8 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 13 | of 18 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 6 | of 18 mandatory requirements reviewed | 2 weak, wrong or unimplemented, 4 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 18 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 18 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 18 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 18 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 18 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 18 | of 24 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 18 mandatory requirements: they add up to 18. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | VPNv6 |
| Automatic checks | Included |
| Requirements | 24 |
| Mandatory requirements (MUST level) | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 13 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 28 |
| Tests linked to requirements | 28 |
| Recorded audit verdicts | 6 |
| Fault test records | 21 |
| Summary | `rfc/short/rfc4659.md` |
| Requirement table | `rfc/requirements/rfc4659.md` |
| RFC text | `rfc/full/rfc4659.txt` |

## Automatic checks

Included in automatic checks: BGP-MPLS IPv6 VPN / VPNv6 (RFC 4659): labeled VPNv6 NLRI, AFI2/SAFI128 capability negotiation, and configured zero-RD global-IPv6 next-hop emission. The widened originator-address obligation has independent recipient-wire proof. IPv4-mapped-IPv6 and global-plus-link-local next-hop gaps remain explicit; data-plane PE tunneling and the integrated multi-AS ASBR service remain outside the implemented baseline. Generic VPNv6 origination and cross-peer forwarding do not establish the option-(b) service.

## Published support status

**Area:** VPNv6

**Test verification:** 3 of 18 MUST proven

**Evidence:** interop 1, unit 2

**Unmet:** RFC4659-3.2.1.2-1, RFC4659-4-1, RFC4659-4-2, RFC4659-4-3, RFC4659-4-4, RFC4659-4-5, RFC4659-4-6, RFC4659-4-7, RFC4659-8-1, RFC4659-8-2, RFC4659-8-3, RFC4659-8-4, RFC4659-8-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 15 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **18** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC4659-3.2-1`](#rfc4659-3.2-1), [`RFC4659-3.4-1`](#rfc4659-3.4-1), [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

**Annotated (including scoped evidence) (15):** [`RFC4659-3.2-2`](#rfc4659-3.2-2), [`RFC4659-4-1`](#rfc4659-4-1), [`RFC4659-4-2`](#rfc4659-4-2), [`RFC4659-4-3`](#rfc4659-4-3), [`RFC4659-4-4`](#rfc4659-4-4), [`RFC4659-4-5`](#rfc4659-4-5), [`RFC4659-4-6`](#rfc4659-4-6), [`RFC4659-4-7`](#rfc4659-4-7), [`RFC4659-8-1`](#rfc4659-8-1), [`RFC4659-8-2`](#rfc4659-8-2), [`RFC4659-8-3`](#rfc4659-8-3), [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1), [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4), [`RFC4659-8-5`](#rfc4659-8-5)

## Figures

- **MUST bar**: Ze verified 3, Tests awaiting verification 2, Not built 13
- **Overview bar**: Ze MUST verified 3, Tests awaiting verification 2, Not assessed 6, Not built 13

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 18 |
| MUST rows kept (the MUST bar) | 18 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 2 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 13 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4659.json](../../../data/rfc/rfc4659.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4659-3.2-1` | When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75). **positive:** `unit/verify` [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23). **positive:** `unit/verify` [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47). **negative:** `unit/verify` [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99). **negative:** `unit/verify` [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50). **negative:** `unit/verify` [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) |
| `RFC4659-3.2-2` | The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L607). **negative:** no negative test. **{single-polarity}:** the obligation is to SET AFI=2/SAFI=128 when advertising a VPNv6 route, so the only conforming assertion is that the emitted fields equal 2/128 and a MUST-NOT-set-other-values companion is degenerate (internal/component/bgp/message/update_build_vpn.go:221, internal/component/bgp/plugins/nlri/vpn/types.go:37) |
| `RFC4659-3.4-1` | In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L173). **positive:** `unit/verify` [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68). **positive:** `unit/verify` [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L129). **negative:** `unit/verify` [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L218). **negative:** `unit/verify` [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L88). **negative:** `unit/verify` [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L143) |
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
| `RFC4659-3.2.1.1-3` | The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L120). **positive:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87). **positive:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L83). **positive:** `unit/verify` [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38). **negative:** `unit/verify` [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L121). **negative:** `unit/verify` [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88). **negative:** `unit/verify` [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L84). **positive:** `functional/verify` [`rfc4659-vpnv6-link-local-off-subnet.ci`](https://github.com/ze-software/ze/blob/main/test/encode/rfc4659-vpnv6-link-local-off-subnet.ci#L5). **positive:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L646). **negative:** `interop/nightly` [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L647) |
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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4659-3.2-1`](#rfc4659-3.2-1)

When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 (independent auditor, authored none of dfe4e3aa8d). RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." dfe4e3aa8d dropped the SAFI argument of parseVPNAnnounce, which now calls parseVPN(afi, SAFIVPN, data, addpath, false), the value the test passed before; TestVPNv6WireRoundTrip keeps every assertion (labels equal, NotEmpty, label 200). The negative and the config and API carriers are unchanged. Scoped test green 2026-10-31. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." 238bdb62e9 renamed the parser the unit calls, ParseVPN to parseVPNAnnounce, which is parseVPN(afi, safi, data, addpath, withdraw=false): the announce layout the old call read, labels included. TestVPNv6WireRoundTrip still round-trips an IPv6 VPN route with its label stack and asserts the label, and the labelless refusal carriers are unchanged, so both polarities stand. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-12 (independent auditor). SHIFTED by 6e063fb498 (tests: give ten assert-nothing tests a real assertion), whose hunks sit in untagged sibling tests and the import block. The tagged unit is byte-identical (gate-verified) and no helper it calls changed. Verdict stands. Re-judged 2026-10-02 (BGP c34 judge). Both operator entry points that originate an IPv6 VPN route are proven. API: plugins/cmd/update/rfc4659_vpnv6_label_test.go, ParseUpdateText ipv6/mpls-vpn rd 65000:100 label 1000 yields the NLRI literal 88 003E81 0000FDE800000064 20010DB80001 (judge checked 1000<<4|BoS = 0x3E81, 136 bits) (positive); no label is refused with route.ErrMissingLabel and a nil result (negative). Config: bgp/config/rfc4659_vpnv6_label_test.go, YANG parse + PeersFromConfigTree keeps Labels [1000] on the VPN static route (positive; its wire form is asserted by the RFC4659-3.4-1 send unit, label 003e81 in MP_REACH), and a labelless configured route is refused at load with "requires at least one label" (negative). Records: parseVPNNLRI and patchStaticRoutes reverts, all four observed red; author probes dropping each label check red only the matching negative (scratch c34p/4659-a, 4659-b). message.BuildVPN still writes a labelless NLRI for empty Labels, but every shipped caller is guarded; its other caller is internal/chaos test tooling. HEAD vpn_test units stay as unrecorded supplementary tags. Rejudged 2026-10-17 (independent reader, not the author of 3f6bb1965e) after the owner-approved command-text change in 3f6bb1965e (injection grammar: nlri <family> became mp <family> for MP families and bare nlri for IPv4 unicast): both cmd/update units changed only the section keyword of their ParseUpdateText argument (nlri ipv6/mpls-vpn became mp ipv6/mpls-vpn); the positive still asserts the exact labeled NLRI 88 003E81 0000FDE800000064 20010DB80001 and the negative still requires route.ErrMissingLabel and a nil result, so each still fails if the label is dropped or the refusal goes. Package re-run green 2026-10-17. RFC 4659 Section 3.2: "When distributing IPv6 VPN routes, the advertising PE router MUST assign and distribute MPLS labels with the IPv6 VPN routes." The two records on these units are re-recorded in the same change. Verdict stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4659ConfiguredVPNv6RouteWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L99) | unit/verify | revert, verified |
| negative | [`TestRFC4659VPNv6AnnouncementWithoutLabelRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L83) | unit/verify | not verified |
| positive | [`TestRFC4659ConfiguredVPNv6RouteKeepsItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc4659_vpnv6_label_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC4659VPNv6AnnouncementCarriesItsLabel`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/rfc4659_vpnv6_label_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go#L47) | unit/verify | not verified |

### [`RFC4659-3.2-2`](#rfc4659-3.2-2)

The Address Family Identifier (AFI) and Subsequent Address Family Identifier (SAFI) fields MUST be set as follows: - AFI: 2; for IPv6 - SAFI: 128; for MPLS labeled VPN-IPv6 (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: MP_REACH for a VPNv6 route with AFI != 2 or SAFI != 128. internal/component/bgp/message/update_build_test.go::TestUpdateBuilder_BuildVPN_IPv6: afi != 2 and value[2] != 128 are Errorf. Single-polarity marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L607) | unit/verify | not verified |

### [`RFC4659-3.4-1`](#rfc4659-3.4-1)

In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs. This is done as specified in [BGP-MP] and [BGP-CAP], by using capability code 1 (multiprotocol BGP), with AFI and SAFI values as specified above, in Section 3.2. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go::TestRFC4659VPNv6RouteSentWhenNegotiated, internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go::TestRFC4659VPNv6RouteWithheldWhenNotNegotiated moved. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 4659 Section 3.4 read: "In order for two PEs to exchange labeled IPv6 VPN NLRIs, they MUST use BGP Capabilities Negotiation to ensure that they both are capable of properly processing such NLRIs." Units read: TestRFC4659VPNv6RouteSentWhenNegotiated and TestRFC4659VPNv6RouteWithheldWhenNotNegotiated; only their initial-sync call changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-14 (independent auditor). Only TestOpenAdvertisesVPNv6Capability changed, mechanically: it now packs ps.Capabilities through capability.PackOptionalParams (the one packer buildOpen uses) instead of the removed buildOptionalParams, then requires Multiprotocol AFI 2/SAFI 128 in the decoded OPEN parameters for a configured ipv6/mpls-vpn family. The send and receive units in rfc4659_vpnv6_negotiated_send_test.go (route sent only when AFI 2/SAFI 128 negotiated, UPDATE refused when not) and the negotiateWith negative are unchanged and still carry the enforcement.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4659VPNv6RouteWithheldWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L88) | unit/verify | mutant, verified |
| negative | [`TestRFC4659VPNv6UpdateRefusedWhenNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L143) | unit/verify | revert, verified |
| negative | [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L218) | unit/verify | not verified |
| positive | [`TestRFC4659VPNv6RouteSentWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L68) | unit/verify | mutant, verified |
| positive | [`TestRFC4659VPNv6UpdateAcceptedWhenNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpnv6_negotiated_send_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L173) | unit/verify | revert, verified |

### [`RFC4659-4-1`](#rfc4659-4-1)

The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-1, so there is no linked test.

### [`RFC4659-4-2`](#rfc4659-4-2)

When the 16-octet IPv6 address contained in the BGP Next Hop field is encoded as an IPv4-mapped IPv6 address (see Section 3.2.1.2), the ingress PE MUST use IPv4 tunneling unless explicitly configured to do otherwise. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-2, so there is no linked test.

### [`RFC4659-4-3`](#rfc4659-4-3)

When the 16-octet IPv6 address contained in the BGP Next Hop field is not encoded as an IPv4-mapped address (see Section 3.2.1.1), the ingress PE MUST use IPv6 tunneling. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-3, so there is no linked test.

### [`RFC4659-4-4`](#rfc4659-4-4)

When tunneling is done using IPv4 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv4 address that is encoded in the IPv4-mapped IPv6 address field of the BGP next hop field as the destination address of the prepended IPv4 tunneling header. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-4, so there is no linked test.

### [`RFC4659-4-5`](#rfc4659-4-5)

When tunneling is done using IPv6 tunnels (whether IPsec secured or not), the Ingress PE Router MUST use the IPv6 address that is contained in the IPv6 address field of the BGP next hop field as the destination address of the prepended IPv6 tunneling header. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-5, so there is no linked test.

### [`RFC4659-4-6`](#rfc4659-4-6)

When tunneling is done using MPLS LSPs, the ingress PE Router MUST directly push the LSP tunnel label on the label stack of the labeled IPv6 VPN packet (i.e., without prepending any IPv4 or IPv6 header). (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-6, so there is no linked test.

### [`RFC4659-4-7`](#rfc4659-4-7)

To ensure interoperability among systems that implement this VPN architecture, all such systems MUST support tunneling using MPLS LSPs established by LDP [LDP]. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-4-7, so there is no linked test.

### [`RFC4659-8-1`](#rfc4659-8-1)

The exchange of IPv6 routes MUST be carried out as per [BGP-IPv6]. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-8-1, so there is no linked test.

### [`RFC4659-8-2`](#rfc4659-8-2)

The exchange of labeled VPN-IPv6 routes MUST be carried out as per [BGP-IPv6] and [MPLS-BGP]. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-8-2, so there is no linked test.

### [`RFC4659-8-3`](#rfc4659-8-3)

An example scenario where both the global IPv6 address and the link- local IPv6 address shall be included in the BGP Next Hop address field is that where the IPv6 VPN service is supported over a multi- Autonomous System (AS) backbone with redistribution of labeled VPN- IPv6 routes between Autonomous System Border Routers (ASBR) of different ASes sharing a common IPv6 subnet: in that case, both the global IPv6 address and the link-local IPv6 address shall be advertised by the ASBRs. (§3.2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-8-3, so there is no linked test.

### [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1)

When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker. (§3.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go::TestRFC4659ConfiguredIPv6TransportNextHopWire moved. Re-judged 2026-10-30 by an independent auditor (authored none of the userns spec commits d8427e4380, b00e3c6710, 68b723ec4f, 7d11a51269). userns.Enter (internal/test/userns/userns_linux.go) re-executes the one test in a new user+net+mount namespace and returns false in the parent only after the child's own PASS line; a child failure, a zero-exit failure or a child that selected nothing fails the parent, and a child skip skips it, so the guard cannot turn a red into a green. Section 3.2.1.1: "When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker." TestRFC4659ConfiguredIPv6TransportNextHopWire changed by the guard only; the fixture's namespace refusal now fails instead of skipping, and rfc4659ConfiguredSettings gained `filter { export { final accept; } }` because the recipient is EBGP and RFC 8212 default-reject would otherwise keep the routes out of Adj-RIB-Out. That is an operator-path policy, not a relaxation: every assertion in rfc4659ReadConfiguredRoutes (exact zero RD, speaker-owned global for self, selected address for explicit, both grouping modes over TCP6 into a real Session) is unchanged. TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair unchanged. {single-polarity: positive} stays correct: sender emission, not receiver rejection. Control-plane evidence only, not tunnel creation. Record (buildMPReachVPN revert) re-taken in 7d11a51269.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L656) | unit/verify | not verified |
| positive | [`TestRFC4659ConfiguredIPv6TransportNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_configured_ipv6_transport_integration_linux_test.go#L54) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_link_local_peering_test.go#L40) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1)

When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv4 tunneling (e.g., IPv4 MPLS LSPs, IPsec-protected IPv4 tunnels), the BGP speaker SHALL advertise to its peer a Next Hop Network Address field containing a VPN-IPv6 address: - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is encoded as an IPv4-mapped IPv6 address [V6ADDR] containing the IPv4 address of the advertising BGP speaker. (§3.2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-3.2.1.2-1, so there is no linked test.

### [`RFC4659-8-4`](#rfc4659-8-4)

When the VPN-IPv6 traffic is to be transported using IPv4 tunneling, the BGP Next Hop Field SHALL contain an IPv4 address encoded as an IPv4-mapped IPv6 address. (Section 8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4659-8-4, so there is no linked test.

### [`RFC4659-8-5`](#rfc4659-8-5)

When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address. (§8)

Audit verdict: unimplemented (no code path enforces the requirement), stale-unit: internal/component/bgp/plugins/rib/rib_bestchange.go::RIBManager.checkRouteBestChange moved. Independent AC12 rejudgment on 2026-11-16 against RFC4659 Section8 approach(b): the IPv6-transport-conditioned SHALL still applies. The current buildMPReachVPN selects the registered profile and writes the supplied next-hop addresses and labels; mpReachNextHopHandler materializes a next-hop Set while preserving the remaining NLRI. Neither selects IPv6 tunnel transport or installs an ASBR-local VPN label/forwarding relationship. RIB checkRouteBestChange now handles opaque route selection too, but only CIDR families mirror into prefix-keyed Loc-RIB; selectedProjection.prepare requires a registered IP family and valid unmapped unzoned CIDR prefix. Thus generic VPN control-plane handling is implemented while the integrated option-(b) service remains absent. No positive/negative carriers tag this row. Retain unimplemented and the explicit gap, with no exemption or whole-row credit. The annotation still names retired sysrib.go::processEvent and should be corrected to the actual projection_path.go::selectedProjection.prepare guard without changing the RFC quote or narrowing applicability. No run or proof replay performed.

Bounded source rejudgment on 2026-11-17 of the changed mpReachNextHopHandler binding only. The full current handler was read against the cited RFC condition. Width admission now calls familyAdmitsNextHopOctets, which reads MPNextHopProfile and AdmitsNextHopOctets: the same six complete-field widths are checked before the family Lengths. The rewrite still keeps the source Reserved octet and all NLRI when no replacement is supplied; the no-operation branch still copies the complete source attribute. These changes supply neither a transport-conditioned ASBR service nor a recipient label-count decision, previous-route lookup, or paired withdrawal. All earlier gap scopes and every producer binding remain; no exemption, feature implementation, partial credit, or whole-row credit follows. This is source judgment only, not an assertion of a test run, proof replay, or native audit freshness. The main thread must stamp only after inspecting exact-row native freshness; unchanged sibling bindings are retained, not newly claimed as changed.

No test carries RFC4659-8-5, so there is no linked test.

### [`RFC4659-3.2.1.1-3`](#rfc4659-3.2.1.1-3)

The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]. (§3.2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), stale-unit: internal/component/bgp/reactor/forward_next_hop.go::applyEgressNextHopScope moved. Independent AC12 rejudgment on 2026-11-16 against RFC 4659 Section 3.2.1.1 and every current tagged carrier: the Go units assert exact zero-RD 48-octet pairs for speaker-owned globals with a configured usable LinkLocal on a common subnet, and exact 24-octet fields for recipients on no connected subnet. The loopback-global controls correctly distinguish VPN peer-only scope from RFC2545 joint scope. The functional positive covers the same configured case. The interop positive requires BIRD exact ordered global and topology-owned Link-Local; its negative is only third-party omission on those same on-link sessions, not a no-common-subnet negative. These narrow assertions remain useful. Whole-row enforced is unsupported: applyLinkLocal accepts an absent leaf, the only production assignment of PeerSettings.LinkLocal is that parser, and linkScope.linkLocalNextHop returns no address for the speaker when that configured value is absent. The shared normalizer and builder then retain the global-only field even when the actual speaker/peer common-subnet condition holds. No tagged carrier exercises interface-derived own Link-Local without this override. This is a source-demonstrated pre-existing implementation/testing gap, not an observed runtime failure. Existing spec-bgp-link-local-only-next-hop AC9-19/AC20 owns interface-derived address selection and actual-interface binding. Verdict weak until that required case is proven and fixed; a later valid partial annotation may preserve the tested scope with an explicit gap, but no partial or enforced claim is made now. No tests, records, audit stamps, compilation or runtime checks were executed by this reviewer.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L79) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| negative | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L84) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| negative | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L88) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L647) | interop/nightly | not verified |
| positive | [`TestRFC4659APIVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_api_vpn_link_local_test.go#L78) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| positive | [`TestRFC4659VPNv6LinkLocalLoopbackGlobal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_loopback_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC4659ForwardVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestRFC4659StaticVPNv6LinkLocalCommonSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_link_local_test.go#L83) | unit/verify | revert, producer-gone (the producer the break was applied to is no longer in the tree) |
| positive | [`TestRFC4659ForwardVPNv6NextHopRewriteCarriesRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4659_vpn_next_hop_rewrite_test.go#L87) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`checkVPNFRR`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L646) | interop/nightly | not verified |
| positive | [`rfc4659-vpnv6-link-local-off-subnet.ci`](https://github.com/ze-software/ze/blob/main/test/encode/rfc4659-vpnv6-link-local-off-subnet.ci#L5) | functional/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 2 | reviewed | not stated |
| `3.2.1` | not stated | 0 | reviewed | not stated |
| `3.2.1.1` | not stated | 1 | reviewed | not stated |
| `3.2.1.2` | not stated | 1 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 7 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 4 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `15` | not stated | 0 | reviewed | not stated |
| `16` | not stated | 0 | reviewed | not stated |
| `16.1` | not stated | 0 | reviewed | not stated |
| `16.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 4659 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4659, so its obligations are stated where they were written.
