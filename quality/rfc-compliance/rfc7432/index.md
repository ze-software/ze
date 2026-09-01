# RFC 7432 - BGP MPLS-Based Ethernet VPN

Partial. Every requirement this repository extracted from RFC 7432, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 3 of 18 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 18 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 82 | of 99 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 64 | of 82 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 83.3% | 15 of 18 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 18 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 99 |
| Gated MUST-level | 82 |
| Obligations that bind Ze | 18 |
| Not applicable, so out of scope | 64 |
| Declared gaps | 15 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7432.md` |
| Requirement shard | `rfc/requirements/rfc7432.md` |
| RFC text | `rfc/full/rfc7432.txt` |

## Enrolment

Enrolled: BGP MPLS-Based Ethernet VPN (EVPN)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

EVPN NLRI family (AFI 25 / SAFI 70), common `[route-type][length][body]` header, and the route-type 1-4 codecs: Route Distinguisher, the 10-octet Ethernet Segment Identifier, Ethernet Tag ID, the 48-bit / 6-octet 802.1Q MAC address, the 0/32/128-bit IP Address Length, Originating Router's IP Address, and the MPLS label stack, plus ADD-PATH framing and the EVPN MP_REACH_NLRI UPDATE builder ([`internal/component/bgp/plugins/nlri/evpn/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types.go), [`internal/component/bgp/message/update_build_evpn.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_evpn.go)). Tests bound per requirement in [`rfc/requirements/rfc7432.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7432.md).

**What the ledger says remains**

Fifteen MUST gaps annotated in [`rfc/short/rfc7432.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7432.md): [`RFC7432-7.9-1`](#rfc7432-7.9-1)/-2/-3 (the RD is per configured route, with no MAC-VRF binding and no uniqueness check), 8.2.1-1/-2 (no per-ES versus per-EVI Ethernet A-D distinction, so MAX-ET and the zero NLRI label are unenforced), 8.1.1-1 (an Ethernet Segment route accepts any RD type, not only Type 1), 9.2.1-2 (an EVPN route with no configured next hop is advertised with next hop 0.0.0.0 instead of the advertising PE address), 9.2.1-3 (Label1 is operator-supplied, with no local label allocation), and 8.2.1-8 / 9.2.1-4 / 8.4.1-1 / 11.1-2 (route targets are attached only when pre-packed by the caller), 8.2.1-3 and 8.1.1-2 (neither the Ethernet A-D per ES route nor the Ethernet Segment route can be originated from a session: `NewEVPNType1` and `NewEVPNType4` ([`internal/component/bgp/plugins/nlri/evpn/encode.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/encode.go)) are reached only from the offline `ze bgp encode` hex tool, while the session origination path `buildEVPNFromParams` admits route types 2, 3 and 5 only. No ESI Label extended community is attached on any path, and the codec defines no type 0x06 sub-type 0x01 value), plus 11.1-1 (the Inclusive Multicast Originating Router's IP is per route). Sixty-four further MUSTs bind PE roles ze does not play: it is a BGP speaker with no MAC-VRF or EVI model, no bridge or MAC learning, no ARP/ND cache, no EVPN forwarding plane, no ES-Import or MAC Mobility or Default Gateway extended community, no PMSI Tunnel attribute, no designated-forwarder election, and no split-horizon, aliasing, or BUM replication.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 79 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **82** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC7432-5-1`](#rfc7432-5-1), [`RFC7432-9.2.1-1`](#rfc7432-9.2.1-1), [`RFC7432-10-1`](#rfc7432-10-1)

**Annotated instead of tested (79):** [`RFC7432-7.9-1`](#rfc7432-7.9-1), [`RFC7432-7.9-2`](#rfc7432-7.9-2), [`RFC7432-7.9-3`](#rfc7432-7.9-3), [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1), [`RFC7432-8.2.1-2`](#rfc7432-8.2.1-2), [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3), [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4), [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5), [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6), [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7), [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8), [`RFC7432-8.1.1-1`](#rfc7432-8.1.1-1), [`RFC7432-8.1.1-2`](#rfc7432-8.1.1-2), [`RFC7432-7.6-1`](#rfc7432-7.6-1), [`RFC7432-7.6-2`](#rfc7432-7.6-2), [`RFC7432-11-1`](#rfc7432-11-1), [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2), [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3), [`RFC7432-9.1-1`](#rfc7432-9.1-1), [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4), [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1), [`RFC7432-10-2`](#rfc7432-10-2), [`RFC7432-10-3`](#rfc7432-10-3), [`RFC7432-10-4`](#rfc7432-10-4), [`RFC7432-11.1-1`](#rfc7432-11.1-1), [`RFC7432-11.1-2`](#rfc7432-11.1-2), [`RFC7432-11.2-1`](#rfc7432-11.2-1), [`RFC7432-11.2-2`](#rfc7432-11.2-2), [`RFC7432-11.2-3`](#rfc7432-11.2-3), [`RFC7432-6.1-1`](#rfc7432-6.1-1), [`RFC7432-6.1-2`](#rfc7432-6.1-2), [`RFC7432-6.2-1`](#rfc7432-6.2-1), [`RFC7432-6.3-1`](#rfc7432-6.3-1), [`RFC7432-6.3-2`](#rfc7432-6.3-2), [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1), [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1), [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1), [`RFC7432-15-1`](#rfc7432-15-1), [`RFC7432-15-2`](#rfc7432-15-2), [`RFC7432-15.1-1`](#rfc7432-15.1-1), [`RFC7432-15.1-2`](#rfc7432-15.1-2), [`RFC7432-15.2-1`](#rfc7432-15.2-1), [`RFC7432-10.1-1`](#rfc7432-10.1-1), [`RFC7432-17.3-1`](#rfc7432-17.3-1), [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1), [`RFC7432-8.4-1`](#rfc7432-8.4-1), [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2), [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3), [`RFC7432-5-3`](#rfc7432-5-3), [`RFC7432-5-4`](#rfc7432-5-4), [`RFC7432-5-5`](#rfc7432-5-5), [`RFC7432-5-6`](#rfc7432-5-6), [`RFC7432-5-7`](#rfc7432-5-7), [`RFC7432-5-8`](#rfc7432-5-8), [`RFC7432-5-9`](#rfc7432-5-9), [`RFC7432-5-10`](#rfc7432-5-10), [`RFC7432-5-11`](#rfc7432-5-11), [`RFC7432-5-12`](#rfc7432-5-12), [`RFC7432-5-13`](#rfc7432-5-13), [`RFC7432-12-1`](#rfc7432-12-1), [`RFC7432-12.1-1`](#rfc7432-12.1-1), [`RFC7432-12.1-2`](#rfc7432-12.1-2), [`RFC7432-12.2-1`](#rfc7432-12.2-1), [`RFC7432-13.1-1`](#rfc7432-13.1-1), [`RFC7432-13.1-2`](#rfc7432-13.1-2), [`RFC7432-13.1-3`](#rfc7432-13.1-3), [`RFC7432-13.1-4`](#rfc7432-13.1-4), [`RFC7432-13.1-5`](#rfc7432-13.1-5), [`RFC7432-14.1-1`](#rfc7432-14.1-1), [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2), [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3), [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1), [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2), [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1), [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2), [`RFC7432-10.1-4`](#rfc7432-10.1-4), [`RFC7432-9.2.1-5`](#rfc7432-9.2.1-5), [`RFC7432-6.2-2`](#rfc7432-6.2-2), [`RFC7432-6.3-3`](#rfc7432-6.3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7432-7.9-1` | RD MUST be set to the RD of the MAC-VRF advertising the NLRI (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the RD of an EVPN NLRI is whatever the route command supplies -- l2vpnRouteToEVPNParams parses it with ParseRDString and hands it straight to the NLRI constructors (internal/component/bgp/plugins/nlri/evpn/encode.go:71-78) -- and ze holds no MAC-VRF whose RD the advertisement could be required to match |
| `RFC7432-7.9-2` | An RD MUST be assigned for a given MAC-VRF on a PE (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseL2VPNArgs refuses a route with an empty route-distinguisher (internal/component/bgp/plugins/nlri/evpn/encode.go:240-242), so every advertised EVPN NLRI does carry an RD, but the unit of assignment is the configured route and ze has no MAC-VRF construct nor any per-MAC-VRF RD allocation |
| `RFC7432-7.9-3` | RD MUST be unique across all MAC-VRFs on a PE (§7.9) | MUST | 7.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** each route command's RD is parsed on its own (internal/component/bgp/plugins/nlri/evpn/encode.go:73-77) and no code compares it against the RDs already in use, so nothing enforces uniqueness across instances |
| `RFC7432-5-1` | Ethernet Segment Identifier MUST be a 10-octet entity (§5, §8.2.1) | MUST | 5 | **positive:** `unit/verify` [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L400). **negative:** `unit/verify` [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L440) |
| `RFC7432-8.2.1-1` | For per-ES A-D route, Ethernet Tag ID MUST be set to MAX-ET (0xFFFFFFFF) (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** NewEVPNType1 stores the operator-supplied Ethernet Tag unchanged (internal/component/bgp/plugins/nlri/evpn/encode.go:90-95) and parseEVPNType1 reads it back verbatim (internal/component/bgp/plugins/nlri/evpn/types.go:281); ze draws no per-ES versus per-EVI distinction and never forces MAX-ET |
| `RFC7432-8.2.1-2` | For per-ES A-D route, MPLS label in the NLRI MUST be set to 0 (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ethernet A-D encoder attaches whichever label the route command carries (internal/component/bgp/plugins/nlri/evpn/encode.go:91-95), with no rule that zeroes the NLRI label for a per-ES route |
| `RFC7432-8.2.1-3` | ESI Label extended community MUST be included in per-ES A-D route (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the route this obligation attaches to cannot be originated from a session, and the ESI Label extended community has no producer at all. NewEVPNType1 has one non-test caller, EncodeRoute (internal/component/bgp/plugins/nlri/evpn/encode.go:95), which is registered only as InProcessRouteEncoder (internal/component/bgp/plugins/nlri/evpn/register.go:42) and resolved only by registry.RouteEncoderByFamily (internal/component/plugin/registry/registry.go:856), whose one non-test caller is the offline hex tool cmdEncode (internal/component/bgp/cli/encode.go:149). The session origination path is buildEVPNFromParams (internal/component/bgp/plugins/nlri/evpn/plugin.go:227), which admits route types 2, 3 and 5 only, and the family registers no InProcessConfigRouteParser, so no configured route reaches route type 1 either. No ESI Label extended community is ever attached on any path: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches only unrelated OSPF SID/Label helpers, the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and BuildEVPN emits an EXTENDED_COMMUNITIES attribute only from the caller-supplied pre-packed ExtCommunityBytes (internal/component/bgp/message/update_build_evpn.go), which l2vpnRouteToEVPNParams never fills. Disclosed in docs/features/rfc-status.md |
| `RFC7432-8.2.1-4` | For all-active mode, Single-Active bit MUST be 0 and ESI label MUST be a valid downstream assigned MPLS label (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze signals no all-active redundancy mode |
| `RFC7432-8.2.1-5` | ESI label MUST have the same value in each Ethernet A-D per ES route for the ES (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so there is no per-ES label value to keep consistent |
| `RFC7432-8.2.1-6` | For P2MP LSPs, ESI label MUST be an upstream assigned MPLS label (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze sets up no P2MP LSP, so no upstream-assigned label is allocated |
| `RFC7432-8.2.1-7` | For single-active mode, Single-Active bit MUST be set to 1 (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze signals no single-active redundancy mode |
| `RFC7432-8.2.1-8` | Each Ethernet A-D per ES route MUST carry one or more RT attributes (§8.2.1) | MUST | 8.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so an Ethernet A-D per ES route configured without a route target is advertised carrying none |
| `RFC7432-8.1.1-1` | RD for Ethernet Segment route MUST be a Type 1 RD (§8.1.1) | MUST | 8.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ethernet Segment encoder accepts any RD type -- l2vpnRouteToEVPNParams passes the parsed RD to NewEVPNType4 unchecked (internal/component/bgp/plugins/nlri/evpn/encode.go:111-112) and parseEVPNType4 accepts every RD type ParseRouteDistinguisher returns (internal/component/bgp/plugins/nlri/evpn/types.go:642-646) |
| `RFC7432-8.1.1-2` | Ethernet Segment route MUST carry an ESI Label extended community (§8.1.1) | MUST | 8.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the route this obligation attaches to cannot be originated from a session, and the ESI Label extended community has no producer at all. NewEVPNType4 has one non-test caller, EncodeRoute (internal/component/bgp/plugins/nlri/evpn/encode.go:112), which is registered only as InProcessRouteEncoder (internal/component/bgp/plugins/nlri/evpn/register.go:42) and resolved only by registry.RouteEncoderByFamily (internal/component/plugin/registry/registry.go:856), whose one non-test caller is the offline hex tool cmdEncode (internal/component/bgp/cli/encode.go:149). The session origination path is buildEVPNFromParams (internal/component/bgp/plugins/nlri/evpn/plugin.go:227), which admits route types 2, 3 and 5 only, and the family registers no InProcessConfigRouteParser, so no configured route reaches route type 4 either. No ESI Label extended community is ever attached on any path: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches only unrelated OSPF SID/Label helpers, the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and BuildEVPN emits an EXTENDED_COMMUNITIES attribute only from the caller-supplied pre-packed ExtCommunityBytes (internal/component/bgp/message/update_build_evpn.go), which l2vpnRouteToEVPNParams never fills. Disclosed in docs/features/rfc-status.md |
| `RFC7432-7.6-1` | Ethernet Segment route filtering MUST ensure route reaches all PEs attached to the ES (§7.6) | MUST | 7.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no ES-Import route target and no Ethernet-segment-scoped distribution logic: grep -rni 'es-import\|esimport' --include=*.go over internal/ and pkg/ matches only an unrelated IANA service name in internal/core/portname/services_table.go |
| `RFC7432-7.6-2` | A BGP speaker implementing RT Constraint MUST apply the RT Constraint procedures to the ES-Import RT (§7.6) | MUST | 7.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no RT Constraint procedures: the rtc plugin is an NLRI codec alone (internal/component/bgp/plugins/nlri/rtc/types.go:97 parses, :186 writes) and grep -rni 'rtc' over internal/component/bgp/reactor, internal/component/bgp/rib and internal/component/bgp/plugins/rib matches only two comments naming RTC as a non-CIDR family, so no route-target-based filtering exists to extend to an ES-Import RT |
| `RFC7432-11-1` | Each PE MUST advertise an Inclusive Multicast Ethernet Tag route (§11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVI or MAC-VRF: grep -rn '\\bEVI\\b' --include=*.go over internal/component/bgp matches nothing and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing, so no per-instance origination duty is expressed anywhere; the Inclusive Multicast codec advertises only what an operator asks for |
| `RFC7432-9.2.1-1` | Encoding of a MAC address MUST be the 6-octet format specified by 802.1Q (§9.2.1) | MUST | 9.2.1 | **positive:** `unit/verify` [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L293). **negative:** `unit/verify` [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L315) |
| `RFC7432-9.2.1-2` | Next Hop field of MP_REACH_NLRI MUST be set to the IPv4 or IPv6 address of the advertising PE (§9.2.1, §11.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildMPReachEVPN writes the configured next hop but substitutes the 4-octet 0.0.0.0 whenever the next hop is unset (internal/component/bgp/message/update_build_evpn.go:161-165), and parseL2VPNArgs treats next-hop as an optional keyword (internal/component/bgp/plugins/nlri/evpn/encode.go:227-232), so an EVPN route reaches the wire with a next hop that is not the advertising PE's address |
| `RFC7432-9.2.1-3` | MPLS Label1 MUST be downstream assigned (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Label1 is the number typed into the route command (internal/component/bgp/plugins/nlri/evpn/encode.go:101-107); ze allocates no local label space for EVPN, so nothing makes the advertised label downstream assigned |
| `RFC7432-9.1-1` | PEs MUST support local data-plane learning using standard IEEE Ethernet learning procedures (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-9.2.1-4` | MAC/IP Advertisement route MUST carry one or more RT attributes (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so a MAC/IP Advertisement route configured without a route target is advertised carrying none |
| `RFC7432-8.4.1-1` | Ethernet A-D per EVI route MUST carry one or more RT attributes (§8.4.1) | MUST | 8.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so an Ethernet A-D per EVI route configured without a route target is advertised carrying none |
| `RFC7432-10-1` | IP Address Length field MUST be 0, 32, or 128 bits for ARP/ND purposes (§10) | MUST | 10 | **positive:** `unit/verify` [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L352). **negative:** `unit/verify` [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L374) |
| `RFC7432-10-2` | If a MAC is associated with multiple IPs, multiple MAC/IP Advertisement routes MUST be generated (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no IP-to-MAC binding table to enumerate: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-10-3` | When an IP-to-MAC binding is removed, the corresponding MAC/IP Advertisement route MUST be withdrawn (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no IP-to-MAC binding whose removal could drive a withdrawal: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-10-4` | On receiving MAC/IP withdrawal with IP but MAC remaining, PE MUST delete ARP table entry but not the MAC (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no ARP or ND cache to delete an entry from: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| `RFC7432-11.1-1` | Originating Router's IP Address MUST be common for all EVIs on the PE (§11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** NewEVPNType3 takes the Originating Router's IP Address from the per-route next hop (internal/component/bgp/plugins/nlri/evpn/encode.go:109-110), and ze holds no PE-wide originating address that would keep the field common across instances |
| `RFC7432-11.1-2` | RT assignment per Section 7.10 MUST be followed for Inclusive Multicast route (§11.1) | MUST | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), and no Section 7.10 route-target derivation exists, so an Inclusive Multicast route carries only the route targets an operator pre-packs |
| `RFC7432-11.2-1` | Inclusive Multicast route with P-tunnel MUST carry a PMSI Tunnel attribute (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-11.2-2` | Leaf Information Required flag of PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-11.2-3` | For ingress replication P-tunnel, the PMSI Tunnel attribute MUST carry a downstream assigned MPLS label (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| `RFC7432-6.1-1` | VLAN-based service: VID translation MUST be supported in data path and performed on disposition PE (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and no data path performs VID translation |
| `RFC7432-6.1-2` | VLAN-based service: Ethernet Tag ID in all EVPN routes MUST be set to 0 (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189) |
| `RFC7432-6.2-1` | VLAN bundle service: MAC addresses MUST be unique across all VLANs in the bundle (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no bundle of VLANs shares a MAC learning space |
| `RFC7432-6.3-1` | VLAN-aware bundle (single VID per domain, no translation): MPLS-encapsulated packet MUST carry that VID (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.3-2` | VLAN-aware bundle (VID translation required): normalized Ethernet Tag ID MUST be carried in EVPN BGP routes (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no VID normalization happens before origination |
| `RFC7432-8.3.1.1-1` | Non-DF ingress PE MUST include ESI label distributed by egress PE in BUM packets (§8.3.1.1) | MUST | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.3.1.2-1` | Penultimate hop popping MUST be disabled on P2MP LSPs used in MPLS transport (§8.3.1.2) | MUST | 8.3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no P2MP LSP and controls no penultimate hop popping: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-8.3.1-1` | ESI label MUST be programmed in forwarding plane by all PEs that receive ES route (§8.3.1) | MUST | 8.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-15-1` | Implementation MUST handle sequence number wraparound in MAC Mobility (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing |
| `RFC7432-15-2` | All advertisements of MAC reachable via previous ES MUST be withdrawn on MAC move (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no MAC move is observed and nothing is withdrawn |
| `RFC7432-15.1-1` | PE MUST alert operator and stop processing MAC/IP routes on duplicate MAC detection (N moves in M seconds) (§15.1) | MUST | 15.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no duplicate-MAC counter or operator alert exists |
| `RFC7432-15.1-2` | Values of M and N for duplicate MAC detection MUST be configurable (§15.1) | MUST | 15.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the M and N duplicate-detection parameters have nothing to configure |
| `RFC7432-15.2-1` | PE receiving sticky MAC and learning same MAC locally MUST alert operator (§15.2) | MUST | 15.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the sticky flag is never read and local learning never happens |
| `RFC7432-10.1-1` | Default gateway route MUST carry Default Gateway Extended Community (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Default Gateway extended community: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing, and no default-gateway route origination exists |
| `RFC7432-17.3-1` | On PE-to-CE link failure, PE MUST withdraw Ethernet A-D per ES, per EVI routes, and MAC/IP Advertisement routes (§17.3) | MUST | 17.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no PE-to-CE attachment circuit: grep -rni 'attachment circuit\|pe.to.ce' --include=*.go over internal/ matches nothing, so no link event drives an EVPN withdrawal |
| `RFC7432-14.1.1-1` | Single-active redundancy: ESI Label Extended Community MUST have Single-Active bit set (§14.1.1) | MUST | 14.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.4-1` | Ethernet A-D per EVI route MUST NOT be used for traffic forwarding until per-ES routes received (§8.4) | MUST NOT | 8.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no route ever becomes a forwarding entry to withhold |
| `RFC7432-8.3.1.1-2` | Ingress PE MUST NOT include ESI label in BUM packet sent to PE not on same ES (§8.3.1.1) | MUST NOT | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| `RFC7432-8.3.1.1-3` | When ESI label matches, egress PE MUST NOT forward packet onto that ES (split-horizon) (§8.3.1.1, §8.3.1.2) | MUST NOT | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code |
| `RFC7432-5-2` | Ethernet segment SHOULD have a non-reserved ESI (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-7.9-4` | Use Type 1 RD for MAC-VRF (§7.9) | SHOULD | 7.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.4-2` | Remote PE receiving MAC/IP route with non-reserved ESI SHOULD consider MAC reachable via all PEs advertising that ES (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10-5` | PE SHOULD perform ARP proxy when it has the MAC binding for a requested IP (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.5-1` | PE that becomes elected DF SHOULD trigger a MAC address flush notification (§8.5) | SHOULD | 8.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1-2` | ESI label SHOULD be distributed by all PEs in single-active mode (§8.3.1) | SHOULD | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1.1-4` | Ingress PE (DF or non-DF) SHOULD include ESI label for known unicast to multihomed ES (§8.3.1.1) | SHOULD | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.6-1` | DF election procedures SHOULD be supported even by single-homing PEs (§8.6) | SHOULD | 8.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10.1-2` | PE SHOULD notify operator on default gateway discrepancy (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-9.1-2` | PEs MAY learn MAC addresses via control plane or management plane integration (§9.1) | MAY | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-11.2-4` | A PE MAY aggregate two or more EVPN instances onto the same P-multicast tree (§11.2) | MAY | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-10.1-3` | Each PE acting as default gateway MAY advertise a MAC/IP route with Default Gateway Extended Community (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-5-3` | ESI Type 1: CE LACP System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-4` | ESI Type 1: CE LACP Port Key MUST be encoded in the 2 octets next to System MAC address (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-5` | ESI Type 2: Root Bridge MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-6` | ESI Type 2: Root Bridge Priority MUST be encoded in the 2 octets next to Root Bridge MAC address (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-7` | ESI Type 3: System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-8` | ESI Type 3: Local Discriminator value MUST be encoded in low-order 3 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-9` | ESI Type 4: Router ID MUST be encoded in high-order 4 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-10` | ESI Type 4: Local Discriminator MUST be encoded in 4 octets next to IP address (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-11` | ESI Type 5: AS number MUST be encoded in high-order 4 octets of ESI Value (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-12` | ESI Type 5: Local Discriminator MUST be encoded in 4 octets next to AS number (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| `RFC7432-5-13` | If CE(s) is not managed, operator MUST configure a non-reserved ESI for multihomed segments (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no multihomed Ethernet segment and no managed CE: ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code; the ESI is operator-provided text parsed by ParseESIString (internal/component/bgp/plugins/nlri/evpn/types.go:119) |
| `RFC7432-12-1` | PE receiving BUM packet from another PE MUST NOT send the frame to other PEs (§12) | MUST NOT | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-12.1-1` | PE receiving packet with ingress replication label MUST treat it as BUM traffic (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received label is classified as BUM |
| `RFC7432-12.1-2` | PE receiving unicast MAC in BUM-labeled packet MUST treat it as unknown unicast (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received frame is classified as unknown unicast |
| `RFC7432-12.2-1` | PE receiving packet on P2MP LSP from PMSI Tunnel attribute MUST treat it as BUM traffic (§12.2) | MUST | 12.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names, and ze receives no P2MP LSP traffic |
| `RFC7432-13.1-1` | When flooding unknown unicast, PE MUST flood to other PEs and MUST first encapsulate with ESI label (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-2` | If ingress replication for unknown unicast, packet MUST be replicated to each remote PE with the PMSI label (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-3` | If P2MP LSPs for unknown unicast, packet MUST be sent on the P2MP LSP of which PE is root (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-4` | If same P2MP LSP is used for all Ethernet tags, all PEs in the EVPN instance MUST be leaves (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| `RFC7432-13.1-5` | If no flooding allowed and MAC unknown, PE MUST drop the packet (§13.1) | MUST | 13.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no packet is dropped for an unknown MAC |
| `RFC7432-14.1-1` | When importing MAC/IP route for an ESI, remote PE MUST examine all imported Ethernet A-D routes for that ESI (§14.1) | MUST | 14.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so importing a MAC/IP route triggers no correlation with Ethernet A-D routes |
| `RFC7432-14.1.1-2` | If Single-Active flag is set in ESI Label extended community, remote PE MUST deduce Single-Active redundancy mode (§14.1.1) | MUST | 14.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| `RFC7432-14.1.1-3` | For Single-Active with multiple backup PEs, remote PE MUST use primary PE withdrawal to start flooding (§14.1.1) | MUST | 14.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no primary or backup PE state and no flooding decision exists |
| `RFC7432-14.1.2-1` | If no Single-Active flag in any ES route, remote PE MUST deduce All-Active redundancy mode (§14.1.2) | MUST | 14.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| `RFC7432-14.1.2-2` | Remote PE MUST use received MAC/IP and Ethernet A-D routes to build ECMP forwarding for All-Active ES (§14.1.2) | MUST | 14.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no ECMP set is built from EVPN routes |
| `RFC7432-9.2.2-1` | If ESI is reserved (0 or MAX-ESI), forwarding state MUST be based on MAC/IP route alone (§9.2.2) | MUST | 9.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-9.2.2-2` | If ESI is non-reserved, forwarding state for MAC MUST wait until both MAC/IP and Ethernet A-D per ES routes are received (§9.2.2) | MUST | 9.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-10.1-4` | Unless all PEs are known to be default gateways, MPLS label in default gateway route MUST be a valid downstream assigned label (§10.1) | MUST | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Default Gateway extended community and allocates no label space for one: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing |
| `RFC7432-9.2.1-5` | PE creating MAC forwarding state from received MAC/IP routes MUST enable forwarding to remote destinations (§9.2.1) | MUST | 9.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.2-2` | VLAN-bundle service: MAC addresses MUST remain tagged with originating VID in MPLS encapsulation (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.3-3` | VLAN-aware bundle with VID translation: Ethernet frames MUST remain tagged with normalized VID (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| `RFC7432-6.1-3` | VLAN-based service: Ethernet frames SHOULD remain tagged with originating VID in MPLS transport (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-17.3-2` | Remote PE receiving Ethernet A-D per ES withdrawal SHOULD consider all MAC/IP routes from that ESI as withdrawn (§17.3) | SHOULD | 17.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-8.3.1.1-5` | Non-DF PE receiving BUM from CE SHOULD not forward it back to CEs on same ES (§8.3.1.1) | SHOULD | 8.3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-18-1` | Preferred PW MPLS Control Word SHOULD NOT be used when sending EVPN-encapsulated packets over pseudowire (§18) | SHOULD NOT | 18 | **positive:** no positive test. **negative:** no negative test |
| `RFC7432-18-2` | Preferred PW MPLS Control Word SHOULD be used with the Ethernet pseudowire type (§18) | SHOULD | 18 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7432-7.9-1`](#rfc7432-7.9-1) RD MUST be set to the RD of the MAC-VRF advertising the NLRI (§7.9) | {gap}, no test | the RD of an EVPN NLRI is whatever the route command supplies -- l2vpnRouteToEVPNParams parses it with ParseRDString and hands it straight to the NLRI constructors (internal/component/bgp/plugins/nlri/evpn/encode.go:71-78) -- and ze holds no MAC-VRF whose RD the advertisement could be required to match |
| [`RFC7432-7.9-2`](#rfc7432-7.9-2) An RD MUST be assigned for a given MAC-VRF on a PE (§7.9) | {gap}, no test | parseL2VPNArgs refuses a route with an empty route-distinguisher (internal/component/bgp/plugins/nlri/evpn/encode.go:240-242), so every advertised EVPN NLRI does carry an RD, but the unit of assignment is the configured route and ze has no MAC-VRF construct nor any per-MAC-VRF RD allocation |
| [`RFC7432-7.9-3`](#rfc7432-7.9-3) RD MUST be unique across all MAC-VRFs on a PE (§7.9) | {gap}, no test | each route command's RD is parsed on its own (internal/component/bgp/plugins/nlri/evpn/encode.go:73-77) and no code compares it against the RDs already in use, so nothing enforces uniqueness across instances |
| [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1) For per-ES A-D route, Ethernet Tag ID MUST be set to MAX-ET (0xFFFFFFFF) (§8.2.1) | {gap}, no test | NewEVPNType1 stores the operator-supplied Ethernet Tag unchanged (internal/component/bgp/plugins/nlri/evpn/encode.go:90-95) and parseEVPNType1 reads it back verbatim (internal/component/bgp/plugins/nlri/evpn/types.go:281); ze draws no per-ES versus per-EVI distinction and never forces MAX-ET |
| [`RFC7432-8.2.1-2`](#rfc7432-8.2.1-2) For per-ES A-D route, MPLS label in the NLRI MUST be set to 0 (§8.2.1) | {gap}, no test | the Ethernet A-D encoder attaches whichever label the route command carries (internal/component/bgp/plugins/nlri/evpn/encode.go:91-95), with no rule that zeroes the NLRI label for a per-ES route |
| [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3) ESI Label extended community MUST be included in per-ES A-D route (§8.2.1) | {gap}, no test | the route this obligation attaches to cannot be originated from a session, and the ESI Label extended community has no producer at all. NewEVPNType1 has one non-test caller, EncodeRoute (internal/component/bgp/plugins/nlri/evpn/encode.go:95), which is registered only as InProcessRouteEncoder (internal/component/bgp/plugins/nlri/evpn/register.go:42) and resolved only by registry.RouteEncoderByFamily (internal/component/plugin/registry/registry.go:856), whose one non-test caller is the offline hex tool cmdEncode (internal/component/bgp/cli/encode.go:149). The session origination path is buildEVPNFromParams (internal/component/bgp/plugins/nlri/evpn/plugin.go:227), which admits route types 2, 3 and 5 only, and the family registers no InProcessConfigRouteParser, so no configured route reaches route type 1 either. No ESI Label extended community is ever attached on any path: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches only unrelated OSPF SID/Label helpers, the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and BuildEVPN emits an EXTENDED_COMMUNITIES attribute only from the caller-supplied pre-packed ExtCommunityBytes (internal/component/bgp/message/update_build_evpn.go), which l2vpnRouteToEVPNParams never fills. Disclosed in docs/features/rfc-status.md |
| [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4) For all-active mode, Single-Active bit MUST be 0 and ESI label MUST be a valid downstream assigned MPLS label (§8.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze signals no all-active redundancy mode |
| [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5) ESI label MUST have the same value in each Ethernet A-D per ES route for the ES (§8.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so there is no per-ES label value to keep consistent |
| [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6) For P2MP LSPs, ESI label MUST be an upstream assigned MPLS label (§8.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze sets up no P2MP LSP, so no upstream-assigned label is allocated |
| [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7) For single-active mode, Single-Active bit MUST be set to 1 (§8.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze signals no single-active redundancy mode |
| [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8) Each Ethernet A-D per ES route MUST carry one or more RT attributes (§8.2.1) | {gap}, no test | BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so an Ethernet A-D per ES route configured without a route target is advertised carrying none |
| [`RFC7432-8.1.1-1`](#rfc7432-8.1.1-1) RD for Ethernet Segment route MUST be a Type 1 RD (§8.1.1) | {gap}, no test | the Ethernet Segment encoder accepts any RD type -- l2vpnRouteToEVPNParams passes the parsed RD to NewEVPNType4 unchecked (internal/component/bgp/plugins/nlri/evpn/encode.go:111-112) and parseEVPNType4 accepts every RD type ParseRouteDistinguisher returns (internal/component/bgp/plugins/nlri/evpn/types.go:642-646) |
| [`RFC7432-8.1.1-2`](#rfc7432-8.1.1-2) Ethernet Segment route MUST carry an ESI Label extended community (§8.1.1) | {gap}, no test | the route this obligation attaches to cannot be originated from a session, and the ESI Label extended community has no producer at all. NewEVPNType4 has one non-test caller, EncodeRoute (internal/component/bgp/plugins/nlri/evpn/encode.go:112), which is registered only as InProcessRouteEncoder (internal/component/bgp/plugins/nlri/evpn/register.go:42) and resolved only by registry.RouteEncoderByFamily (internal/component/plugin/registry/registry.go:856), whose one non-test caller is the offline hex tool cmdEncode (internal/component/bgp/cli/encode.go:149). The session origination path is buildEVPNFromParams (internal/component/bgp/plugins/nlri/evpn/plugin.go:227), which admits route types 2, 3 and 5 only, and the family registers no InProcessConfigRouteParser, so no configured route reaches route type 4 either. No ESI Label extended community is ever attached on any path: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches only unrelated OSPF SID/Label helpers, the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and BuildEVPN emits an EXTENDED_COMMUNITIES attribute only from the caller-supplied pre-packed ExtCommunityBytes (internal/component/bgp/message/update_build_evpn.go), which l2vpnRouteToEVPNParams never fills. Disclosed in docs/features/rfc-status.md |
| [`RFC7432-7.6-1`](#rfc7432-7.6-1) Ethernet Segment route filtering MUST ensure route reaches all PEs attached to the ES (§7.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no ES-Import route target and no Ethernet-segment-scoped distribution logic: grep -rni 'es-import\|esimport' --include=*.go over internal/ and pkg/ matches only an unrelated IANA service name in internal/core/portname/services_table.go |
| [`RFC7432-7.6-2`](#rfc7432-7.6-2) A BGP speaker implementing RT Constraint MUST apply the RT Constraint procedures to the ES-Import RT (§7.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no RT Constraint procedures: the rtc plugin is an NLRI codec alone (internal/component/bgp/plugins/nlri/rtc/types.go:97 parses, :186 writes) and grep -rni 'rtc' over internal/component/bgp/reactor, internal/component/bgp/rib and internal/component/bgp/plugins/rib matches only two comments naming RTC as a non-CIDR family, so no route-target-based filtering exists to extend to an ES-Import RT |
| [`RFC7432-11-1`](#rfc7432-11-1) Each PE MUST advertise an Inclusive Multicast Ethernet Tag route (§11) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVI or MAC-VRF: grep -rn '\\bEVI\\b' --include=*.go over internal/component/bgp matches nothing and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing, so no per-instance origination duty is expressed anywhere; the Inclusive Multicast codec advertises only what an operator asks for |
| [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2) Next Hop field of MP_REACH_NLRI MUST be set to the IPv4 or IPv6 address of the advertising PE (§9.2.1, §11.1) | {gap}, no test | buildMPReachEVPN writes the configured next hop but substitutes the 4-octet 0.0.0.0 whenever the next hop is unset (internal/component/bgp/message/update_build_evpn.go:161-165), and parseL2VPNArgs treats next-hop as an optional keyword (internal/component/bgp/plugins/nlri/evpn/encode.go:227-232), so an EVPN route reaches the wire with a next hop that is not the advertising PE's address |
| [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3) MPLS Label1 MUST be downstream assigned (§9.2.1) | {gap}, no test | Label1 is the number typed into the route command (internal/component/bgp/plugins/nlri/evpn/encode.go:101-107); ze allocates no local label space for EVPN, so nothing makes the advertised label downstream assigned |
| [`RFC7432-9.1-1`](#rfc7432-9.1-1) PEs MUST support local data-plane learning using standard IEEE Ethernet learning procedures (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4) MAC/IP Advertisement route MUST carry one or more RT attributes (§9.2.1) | {gap}, no test | BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so a MAC/IP Advertisement route configured without a route target is advertised carrying none |
| [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1) Ethernet A-D per EVI route MUST carry one or more RT attributes (§8.4.1) | {gap}, no test | BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), so an Ethernet A-D per EVI route configured without a route target is advertised carrying none |
| [`RFC7432-10-2`](#rfc7432-10-2) If a MAC is associated with multiple IPs, multiple MAC/IP Advertisement routes MUST be generated (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no IP-to-MAC binding table to enumerate: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-10-3`](#rfc7432-10-3) When an IP-to-MAC binding is removed, the corresponding MAC/IP Advertisement route MUST be withdrawn (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no IP-to-MAC binding whose removal could drive a withdrawal: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-10-4`](#rfc7432-10-4) On receiving MAC/IP withdrawal with IP but MAC remaining, PE MUST delete ARP table entry but not the MAC (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no ARP or ND cache to delete an entry from: ze runs no bridge and keeps no MAC table: the EVPN feature is the NLRI codec at internal/component/bgp/plugins/nlri/evpn/types.go plus the UPDATE builder at internal/component/bgp/message/update_build_evpn.go, and grep -rni 'mac.vrf\|macvrf' --include=*.go matches nothing |
| [`RFC7432-11.1-1`](#rfc7432-11.1-1) Originating Router's IP Address MUST be common for all EVIs on the PE (§11.1) | {gap}, no test | NewEVPNType3 takes the Originating Router's IP Address from the per-route next hop (internal/component/bgp/plugins/nlri/evpn/encode.go:109-110), and ze holds no PE-wide originating address that would keep the field common across instances |
| [`RFC7432-11.1-2`](#rfc7432-11.1-2) RT assignment per Section 7.10 MUST be followed for Inclusive Multicast route (§11.1) | {gap}, no test | BuildEVPN emits an EXTENDED_COMMUNITIES attribute only when the caller hands it pre-packed bytes (internal/component/bgp/message/update_build_evpn.go:118-124), and l2vpnRouteToEVPNParams never fills ExtCommunityBytes (internal/component/bgp/plugins/nlri/evpn/encode.go:64-124), and no Section 7.10 route-target derivation exists, so an Inclusive Multicast route carries only the route targets an operator pre-packs |
| [`RFC7432-11.2-1`](#rfc7432-11.2-1) Inclusive Multicast route with P-tunnel MUST carry a PMSI Tunnel attribute (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-11.2-2`](#rfc7432-11.2-2) Leaf Information Required flag of PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-11.2-3`](#rfc7432-11.2-3) For ingress replication P-tunnel, the PMSI Tunnel attribute MUST carry a downstream assigned MPLS label (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names |
| [`RFC7432-6.1-1`](#rfc7432-6.1-1) VLAN-based service: VID translation MUST be supported in data path and performed on disposition PE (§6.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and no data path performs VID translation |
| [`RFC7432-6.1-2`](#rfc7432-6.1-2) VLAN-based service: Ethernet Tag ID in all EVPN routes MUST be set to 0 (§6.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189) |
| [`RFC7432-6.2-1`](#rfc7432-6.2-1) VLAN bundle service: MAC addresses MUST be unique across all VLANs in the bundle (§6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no bundle of VLANs shares a MAC learning space |
| [`RFC7432-6.3-1`](#rfc7432-6.3-1) VLAN-aware bundle (single VID per domain, no translation): MPLS-encapsulated packet MUST carry that VID (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.3-2`](#rfc7432-6.3-2) VLAN-aware bundle (VID translation required): normalized Ethernet Tag ID MUST be carried in EVPN BGP routes (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), so no VID normalization happens before origination |
| [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1) Non-DF ingress PE MUST include ESI label distributed by egress PE in BUM packets (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1) Penultimate hop popping MUST be disabled on P2MP LSPs used in MPLS transport (§8.3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no P2MP LSP and controls no penultimate hop popping: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1) ESI label MUST be programmed in forwarding plane by all PEs that receive ES route (§8.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-15-1`](#rfc7432-15-1) Implementation MUST handle sequence number wraparound in MAC Mobility (§15) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing |
| [`RFC7432-15-2`](#rfc7432-15-2) All advertisements of MAC reachable via previous ES MUST be withdrawn on MAC move (§15) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no MAC move is observed and nothing is withdrawn |
| [`RFC7432-15.1-1`](#rfc7432-15.1-1) PE MUST alert operator and stop processing MAC/IP routes on duplicate MAC detection (N moves in M seconds) (§15.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so no duplicate-MAC counter or operator alert exists |
| [`RFC7432-15.1-2`](#rfc7432-15.1-2) Values of M and N for duplicate MAC detection MUST be configurable (§15.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the M and N duplicate-detection parameters have nothing to configure |
| [`RFC7432-15.2-1`](#rfc7432-15.2-1) PE receiving sticky MAC and learning same MAC locally MUST alert operator (§15.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no MAC Mobility extended community and no MAC table: grep -rni 'mac.mobility\|macmobility' --include=*.go over internal/ and pkg/ matches nothing, so the sticky flag is never read and local learning never happens |
| [`RFC7432-10.1-1`](#rfc7432-10.1-1) Default gateway route MUST carry Default Gateway Extended Community (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Default Gateway extended community: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing, and no default-gateway route origination exists |
| [`RFC7432-17.3-1`](#rfc7432-17.3-1) On PE-to-CE link failure, PE MUST withdraw Ethernet A-D per ES, per EVI routes, and MAC/IP Advertisement routes (§17.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no PE-to-CE attachment circuit: grep -rni 'attachment circuit\|pe.to.ce' --include=*.go over internal/ matches nothing, so no link event drives an EVPN withdrawal |
| [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1) Single-active redundancy: ESI Label Extended Community MUST have Single-Active bit set (§14.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.4-1`](#rfc7432-8.4-1) Ethernet A-D per EVI route MUST NOT be used for traffic forwarding until per-ES routes received (§8.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no route ever becomes a forwarding entry to withhold |
| [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2) Ingress PE MUST NOT include ESI label in BUM packet sent to PE not on same ES (§8.3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value |
| [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3) When ESI label matches, egress PE MUST NOT forward packet onto that ES (split-horizon) (§8.3.1.1, §8.3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, and ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code |
| [`RFC7432-5-3`](#rfc7432-5-3) ESI Type 1: CE LACP System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-4`](#rfc7432-5-4) ESI Type 1: CE LACP Port Key MUST be encoded in the 2 octets next to System MAC address (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-5`](#rfc7432-5-5) ESI Type 2: Root Bridge MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-6`](#rfc7432-5-6) ESI Type 2: Root Bridge Priority MUST be encoded in the 2 octets next to Root Bridge MAC address (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-7`](#rfc7432-5-7) ESI Type 3: System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-8`](#rfc7432-5-8) ESI Type 3: Local Discriminator value MUST be encoded in low-order 3 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-9`](#rfc7432-5-9) ESI Type 4: Router ID MUST be encoded in high-order 4 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-10`](#rfc7432-5-10) ESI Type 4: Local Discriminator MUST be encoded in 4 octets next to IP address (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-11`](#rfc7432-5-11) ESI Type 5: AS number MUST be encoded in high-order 4 octets of ESI Value (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-12`](#rfc7432-5-12) ESI Type 5: Local Discriminator MUST be encoded in 4 octets next to AS number (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze treats the ESI as ten opaque octets: ESI is declared as [10]byte at internal/component/bgp/plugins/nlri/evpn/types.go:95, ParseESIString reads hex or colon-hex text without inspecting the type octet, and grep -rn 'ESIType\|esiType\|esi\\[0\\]' --include=*.go over internal/ matches only a formatting call in internal/component/bgp/plugins/cmd/update/update_text_evpn.go:335, so no type-specific ESI construction exists |
| [`RFC7432-5-13`](#rfc7432-5-13) If CE(s) is not managed, operator MUST configure a non-reserved ESI for multihomed segments (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no multihomed Ethernet segment and no managed CE: ze holds no Ethernet segment state and elects no designated forwarder: grep -rni 'designated.forwarder\|df.election\|split.horizon' --include=*.go over internal/ and pkg/ matches only a prose comment in the EVPN codec tests, never code; the ESI is operator-provided text parsed by ParseESIString (internal/component/bgp/plugins/nlri/evpn/types.go:119) |
| [`RFC7432-12-1`](#rfc7432-12-1) PE receiving BUM packet from another PE MUST NOT send the frame to other PEs (§12) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-12.1-1`](#rfc7432-12.1-1) PE receiving packet with ingress replication label MUST treat it as BUM traffic (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received label is classified as BUM |
| [`RFC7432-12.1-2`](#rfc7432-12.1-2) PE receiving unicast MAC in BUM-labeled packet MUST treat it as unknown unicast (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no received frame is classified as unknown unicast |
| [`RFC7432-12.2-1`](#rfc7432-12.2-1) PE receiving packet on P2MP LSP from PMSI Tunnel attribute MUST treat it as BUM traffic (§12.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze parses no PMSI Tunnel attribute: internal/core/bgp/attribute/wire.go:419 lists PMSI among the known attribute codes that have no parser, and grep -rni 'pmsi' --include=*.go finds only MVPN route-type names, and ze receives no P2MP LSP traffic |
| [`RFC7432-13.1-1`](#rfc7432-13.1-1) When flooding unknown unicast, PE MUST flood to other PEs and MUST first encapsulate with ESI label (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-2`](#rfc7432-13.1-2) If ingress replication for unknown unicast, packet MUST be replicated to each remote PE with the PMSI label (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-3`](#rfc7432-13.1-3) If P2MP LSPs for unknown unicast, packet MUST be sent on the P2MP LSP of which PE is root (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-4`](#rfc7432-13.1-4) If same P2MP LSP is used for all Ethernet tags, all PEs in the EVPN instance MUST be leaves (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing |
| [`RFC7432-13.1-5`](#rfc7432-13.1-5) If no flooding allowed and MAC unknown, PE MUST drop the packet (§13.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no BUM traffic for EVPN: it terminates BGP UPDATEs and has no bridge domain, no ingress replication, and no P2MP LSP; grep -rni 'evpn' --include=*.go over internal/plugins/fib/ matches nothing, so no packet is dropped for an unknown MAC |
| [`RFC7432-14.1-1`](#rfc7432-14.1-1) When importing MAC/IP route for an ESI, remote PE MUST examine all imported Ethernet A-D routes for that ESI (§14.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so importing a MAC/IP route triggers no correlation with Ethernet A-D routes |
| [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2) If Single-Active flag is set in ESI Label extended community, remote PE MUST deduce Single-Active redundancy mode (§14.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3) For Single-Active with multiple backup PEs, remote PE MUST use primary PE withdrawal to start flooding (§14.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no primary or backup PE state and no flooding decision exists |
| [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1) If no Single-Active flag in any ES route, remote PE MUST deduce All-Active redundancy mode (§14.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze carries no ESI Label extended community: grep -rni 'esi.label' --include=*.go over internal/ and pkg/ matches nothing, and the extended-community codec internal/core/bgp/attribute/community.go defines no type 0x06 sub-type 0x01 value, so a receiving speaker deduces no redundancy mode |
| [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2) Remote PE MUST use received MAC/IP and Ethernet A-D routes to build ECMP forwarding for All-Active ES (§14.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists, so no ECMP set is built from EVPN routes |
| [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1) If ESI is reserved (0 or MAX-ESI), forwarding state MUST be based on MAC/IP route alone (§9.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2) If ESI is non-reserved, forwarding state for MAC MUST wait until both MAC/IP and Ethernet A-D per ES routes are received (§9.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-10.1-4`](#rfc7432-10.1-4) Unless all PEs are known to be default gateways, MPLS label in default gateway route MUST be a valid downstream assigned label (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Default Gateway extended community and allocates no label space for one: grep -rni 'default.gateway' --include=*.go over internal/core/bgp/attribute matches nothing |
| [`RFC7432-9.2.1-5`](#rfc7432-9.2.1-5) PE creating MAC forwarding state from received MAC/IP routes MUST enable forwarding to remote destinations (§9.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.2-2`](#rfc7432-6.2-2) VLAN-bundle service: MAC addresses MUST remain tagged with originating VID in MPLS encapsulation (§6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |
| [`RFC7432-6.3-3`](#rfc7432-6.3-3) VLAN-aware bundle with VID translation: Ethernet frames MUST remain tagged with normalized VID (§6.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze models no EVPN service type: grep -rni 'vlan.based\|vlan.bundle\|vlan.aware' --include=*.go over internal/ matches nothing, and the Ethernet Tag ID is simply the value the route command supplies (internal/component/bgp/plugins/nlri/evpn/encode.go:189), and ze programs no EVPN forwarding plane: grep -rni 'evpn' --include=*.go over internal/plugins/fib/ and internal/component/iface/ matches nothing, so no encapsulation, label push, or bridge-port decision exists |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7432-7.9-1`](#rfc7432-7.9-1)

RD MUST be set to the RD of the MAC-VRF advertising the NLRI (§7.9)

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

Ethernet Segment Identifier MUST be a 10-octet entity (§5, §8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L440) | unit/verify | unproven |
| positive | [`TestEVPNESIIsTenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L400) | unit/verify | unproven |

### [`RFC7432-8.2.1-1`](#rfc7432-8.2.1-1)

For per-ES A-D route, Ethernet Tag ID MUST be set to MAX-ET (0xFFFFFFFF) (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-1, so no unit is bound to it.

### [`RFC7432-8.2.1-2`](#rfc7432-8.2.1-2)

For per-ES A-D route, MPLS label in the NLRI MUST be set to 0 (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-2, so no unit is bound to it.

### [`RFC7432-8.2.1-3`](#rfc7432-8.2.1-3)

ESI Label extended community MUST be included in per-ES A-D route (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-3, so no unit is bound to it.

### [`RFC7432-8.2.1-4`](#rfc7432-8.2.1-4)

For all-active mode, Single-Active bit MUST be 0 and ESI label MUST be a valid downstream assigned MPLS label (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-4, so no unit is bound to it.

### [`RFC7432-8.2.1-5`](#rfc7432-8.2.1-5)

ESI label MUST have the same value in each Ethernet A-D per ES route for the ES (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-5, so no unit is bound to it.

### [`RFC7432-8.2.1-6`](#rfc7432-8.2.1-6)

For P2MP LSPs, ESI label MUST be an upstream assigned MPLS label (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-6, so no unit is bound to it.

### [`RFC7432-8.2.1-7`](#rfc7432-8.2.1-7)

For single-active mode, Single-Active bit MUST be set to 1 (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-7, so no unit is bound to it.

### [`RFC7432-8.2.1-8`](#rfc7432-8.2.1-8)

Each Ethernet A-D per ES route MUST carry one or more RT attributes (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.2.1-8, so no unit is bound to it.

### [`RFC7432-8.1.1-1`](#rfc7432-8.1.1-1)

RD for Ethernet Segment route MUST be a Type 1 RD (§8.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.1.1-1, so no unit is bound to it.

### [`RFC7432-8.1.1-2`](#rfc7432-8.1.1-2)

Ethernet Segment route MUST carry an ESI Label extended community (§8.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.1.1-2, so no unit is bound to it.

### [`RFC7432-7.6-1`](#rfc7432-7.6-1)

Ethernet Segment route filtering MUST ensure route reaches all PEs attached to the ES (§7.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.6-1, so no unit is bound to it.

### [`RFC7432-7.6-2`](#rfc7432-7.6-2)

A BGP speaker implementing RT Constraint MUST apply the RT Constraint procedures to the ES-Import RT (§7.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-7.6-2, so no unit is bound to it.

### [`RFC7432-11-1`](#rfc7432-11-1)

Each PE MUST advertise an Inclusive Multicast Ethernet Tag route (§11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11-1, so no unit is bound to it.

### [`RFC7432-9.2.1-1`](#rfc7432-9.2.1-1)

Encoding of a MAC address MUST be the 6-octet format specified by 802.1Q (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L315) | unit/verify | unproven |
| positive | [`TestEVPNMACAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L293) | unit/verify | unproven |

### [`RFC7432-9.2.1-2`](#rfc7432-9.2.1-2)

Next Hop field of MP_REACH_NLRI MUST be set to the IPv4 or IPv6 address of the advertising PE (§9.2.1, §11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-2, so no unit is bound to it.

### [`RFC7432-9.2.1-3`](#rfc7432-9.2.1-3)

MPLS Label1 MUST be downstream assigned (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-3, so no unit is bound to it.

### [`RFC7432-9.1-1`](#rfc7432-9.1-1)

PEs MUST support local data-plane learning using standard IEEE Ethernet learning procedures (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.1-1, so no unit is bound to it.

### [`RFC7432-9.2.1-4`](#rfc7432-9.2.1-4)

MAC/IP Advertisement route MUST carry one or more RT attributes (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-4, so no unit is bound to it.

### [`RFC7432-8.4.1-1`](#rfc7432-8.4.1-1)

Ethernet A-D per EVI route MUST carry one or more RT attributes (§8.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.4.1-1, so no unit is bound to it.

### [`RFC7432-10-1`](#rfc7432-10-1)

IP Address Length field MUST be 0, 32, or 128 bits for ARP/ND purposes (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L374) | unit/verify | unproven |
| positive | [`TestEVPNIPAddressLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L352) | unit/verify | unproven |

### [`RFC7432-10-2`](#rfc7432-10-2)

If a MAC is associated with multiple IPs, multiple MAC/IP Advertisement routes MUST be generated (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-2, so no unit is bound to it.

### [`RFC7432-10-3`](#rfc7432-10-3)

When an IP-to-MAC binding is removed, the corresponding MAC/IP Advertisement route MUST be withdrawn (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-3, so no unit is bound to it.

### [`RFC7432-10-4`](#rfc7432-10-4)

On receiving MAC/IP withdrawal with IP but MAC remaining, PE MUST delete ARP table entry but not the MAC (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10-4, so no unit is bound to it.

### [`RFC7432-11.1-1`](#rfc7432-11.1-1)

Originating Router's IP Address MUST be common for all EVIs on the PE (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.1-1, so no unit is bound to it.

### [`RFC7432-11.1-2`](#rfc7432-11.1-2)

RT assignment per Section 7.10 MUST be followed for Inclusive Multicast route (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.1-2, so no unit is bound to it.

### [`RFC7432-11.2-1`](#rfc7432-11.2-1)

Inclusive Multicast route with P-tunnel MUST carry a PMSI Tunnel attribute (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-1, so no unit is bound to it.

### [`RFC7432-11.2-2`](#rfc7432-11.2-2)

Leaf Information Required flag of PMSI Tunnel attribute MUST be set to zero and MUST be ignored on receipt (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-2, so no unit is bound to it.

### [`RFC7432-11.2-3`](#rfc7432-11.2-3)

For ingress replication P-tunnel, the PMSI Tunnel attribute MUST carry a downstream assigned MPLS label (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-11.2-3, so no unit is bound to it.

### [`RFC7432-6.1-1`](#rfc7432-6.1-1)

VLAN-based service: VID translation MUST be supported in data path and performed on disposition PE (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.1-1, so no unit is bound to it.

### [`RFC7432-6.1-2`](#rfc7432-6.1-2)

VLAN-based service: Ethernet Tag ID in all EVPN routes MUST be set to 0 (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.1-2, so no unit is bound to it.

### [`RFC7432-6.2-1`](#rfc7432-6.2-1)

VLAN bundle service: MAC addresses MUST be unique across all VLANs in the bundle (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.2-1, so no unit is bound to it.

### [`RFC7432-6.3-1`](#rfc7432-6.3-1)

VLAN-aware bundle (single VID per domain, no translation): MPLS-encapsulated packet MUST carry that VID (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-1, so no unit is bound to it.

### [`RFC7432-6.3-2`](#rfc7432-6.3-2)

VLAN-aware bundle (VID translation required): normalized Ethernet Tag ID MUST be carried in EVPN BGP routes (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-2, so no unit is bound to it.

### [`RFC7432-8.3.1.1-1`](#rfc7432-8.3.1.1-1)

Non-DF ingress PE MUST include ESI label distributed by egress PE in BUM packets (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-1, so no unit is bound to it.

### [`RFC7432-8.3.1.2-1`](#rfc7432-8.3.1.2-1)

Penultimate hop popping MUST be disabled on P2MP LSPs used in MPLS transport (§8.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.2-1, so no unit is bound to it.

### [`RFC7432-8.3.1-1`](#rfc7432-8.3.1-1)

ESI label MUST be programmed in forwarding plane by all PEs that receive ES route (§8.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1-1, so no unit is bound to it.

### [`RFC7432-15-1`](#rfc7432-15-1)

Implementation MUST handle sequence number wraparound in MAC Mobility (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15-1, so no unit is bound to it.

### [`RFC7432-15-2`](#rfc7432-15-2)

All advertisements of MAC reachable via previous ES MUST be withdrawn on MAC move (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15-2, so no unit is bound to it.

### [`RFC7432-15.1-1`](#rfc7432-15.1-1)

PE MUST alert operator and stop processing MAC/IP routes on duplicate MAC detection (N moves in M seconds) (§15.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.1-1, so no unit is bound to it.

### [`RFC7432-15.1-2`](#rfc7432-15.1-2)

Values of M and N for duplicate MAC detection MUST be configurable (§15.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.1-2, so no unit is bound to it.

### [`RFC7432-15.2-1`](#rfc7432-15.2-1)

PE receiving sticky MAC and learning same MAC locally MUST alert operator (§15.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-15.2-1, so no unit is bound to it.

### [`RFC7432-10.1-1`](#rfc7432-10.1-1)

Default gateway route MUST carry Default Gateway Extended Community (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10.1-1, so no unit is bound to it.

### [`RFC7432-17.3-1`](#rfc7432-17.3-1)

On PE-to-CE link failure, PE MUST withdraw Ethernet A-D per ES, per EVI routes, and MAC/IP Advertisement routes (§17.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-17.3-1, so no unit is bound to it.

### [`RFC7432-14.1.1-1`](#rfc7432-14.1.1-1)

Single-active redundancy: ESI Label Extended Community MUST have Single-Active bit set (§14.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-1, so no unit is bound to it.

### [`RFC7432-8.4-1`](#rfc7432-8.4-1)

Ethernet A-D per EVI route MUST NOT be used for traffic forwarding until per-ES routes received (§8.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.4-1, so no unit is bound to it.

### [`RFC7432-8.3.1.1-2`](#rfc7432-8.3.1.1-2)

Ingress PE MUST NOT include ESI label in BUM packet sent to PE not on same ES (§8.3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-2, so no unit is bound to it.

### [`RFC7432-8.3.1.1-3`](#rfc7432-8.3.1.1-3)

When ESI label matches, egress PE MUST NOT forward packet onto that ES (split-horizon) (§8.3.1.1, §8.3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-8.3.1.1-3, so no unit is bound to it.

### [`RFC7432-5-3`](#rfc7432-5-3)

ESI Type 1: CE LACP System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-3, so no unit is bound to it.

### [`RFC7432-5-4`](#rfc7432-5-4)

ESI Type 1: CE LACP Port Key MUST be encoded in the 2 octets next to System MAC address (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-4, so no unit is bound to it.

### [`RFC7432-5-5`](#rfc7432-5-5)

ESI Type 2: Root Bridge MAC address MUST be encoded in high-order 6 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-5, so no unit is bound to it.

### [`RFC7432-5-6`](#rfc7432-5-6)

ESI Type 2: Root Bridge Priority MUST be encoded in the 2 octets next to Root Bridge MAC address (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-6, so no unit is bound to it.

### [`RFC7432-5-7`](#rfc7432-5-7)

ESI Type 3: System MAC address MUST be encoded in high-order 6 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-7, so no unit is bound to it.

### [`RFC7432-5-8`](#rfc7432-5-8)

ESI Type 3: Local Discriminator value MUST be encoded in low-order 3 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-8, so no unit is bound to it.

### [`RFC7432-5-9`](#rfc7432-5-9)

ESI Type 4: Router ID MUST be encoded in high-order 4 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-9, so no unit is bound to it.

### [`RFC7432-5-10`](#rfc7432-5-10)

ESI Type 4: Local Discriminator MUST be encoded in 4 octets next to IP address (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-10, so no unit is bound to it.

### [`RFC7432-5-11`](#rfc7432-5-11)

ESI Type 5: AS number MUST be encoded in high-order 4 octets of ESI Value (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-11, so no unit is bound to it.

### [`RFC7432-5-12`](#rfc7432-5-12)

ESI Type 5: Local Discriminator MUST be encoded in 4 octets next to AS number (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-12, so no unit is bound to it.

### [`RFC7432-5-13`](#rfc7432-5-13)

If CE(s) is not managed, operator MUST configure a non-reserved ESI for multihomed segments (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-5-13, so no unit is bound to it.

### [`RFC7432-12-1`](#rfc7432-12-1)

PE receiving BUM packet from another PE MUST NOT send the frame to other PEs (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12-1, so no unit is bound to it.

### [`RFC7432-12.1-1`](#rfc7432-12.1-1)

PE receiving packet with ingress replication label MUST treat it as BUM traffic (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.1-1, so no unit is bound to it.

### [`RFC7432-12.1-2`](#rfc7432-12.1-2)

PE receiving unicast MAC in BUM-labeled packet MUST treat it as unknown unicast (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.1-2, so no unit is bound to it.

### [`RFC7432-12.2-1`](#rfc7432-12.2-1)

PE receiving packet on P2MP LSP from PMSI Tunnel attribute MUST treat it as BUM traffic (§12.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-12.2-1, so no unit is bound to it.

### [`RFC7432-13.1-1`](#rfc7432-13.1-1)

When flooding unknown unicast, PE MUST flood to other PEs and MUST first encapsulate with ESI label (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-1, so no unit is bound to it.

### [`RFC7432-13.1-2`](#rfc7432-13.1-2)

If ingress replication for unknown unicast, packet MUST be replicated to each remote PE with the PMSI label (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-2, so no unit is bound to it.

### [`RFC7432-13.1-3`](#rfc7432-13.1-3)

If P2MP LSPs for unknown unicast, packet MUST be sent on the P2MP LSP of which PE is root (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-3, so no unit is bound to it.

### [`RFC7432-13.1-4`](#rfc7432-13.1-4)

If same P2MP LSP is used for all Ethernet tags, all PEs in the EVPN instance MUST be leaves (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-4, so no unit is bound to it.

### [`RFC7432-13.1-5`](#rfc7432-13.1-5)

If no flooding allowed and MAC unknown, PE MUST drop the packet (§13.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-13.1-5, so no unit is bound to it.

### [`RFC7432-14.1-1`](#rfc7432-14.1-1)

When importing MAC/IP route for an ESI, remote PE MUST examine all imported Ethernet A-D routes for that ESI (§14.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1-1, so no unit is bound to it.

### [`RFC7432-14.1.1-2`](#rfc7432-14.1.1-2)

If Single-Active flag is set in ESI Label extended community, remote PE MUST deduce Single-Active redundancy mode (§14.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-2, so no unit is bound to it.

### [`RFC7432-14.1.1-3`](#rfc7432-14.1.1-3)

For Single-Active with multiple backup PEs, remote PE MUST use primary PE withdrawal to start flooding (§14.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.1-3, so no unit is bound to it.

### [`RFC7432-14.1.2-1`](#rfc7432-14.1.2-1)

If no Single-Active flag in any ES route, remote PE MUST deduce All-Active redundancy mode (§14.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.2-1, so no unit is bound to it.

### [`RFC7432-14.1.2-2`](#rfc7432-14.1.2-2)

Remote PE MUST use received MAC/IP and Ethernet A-D routes to build ECMP forwarding for All-Active ES (§14.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-14.1.2-2, so no unit is bound to it.

### [`RFC7432-9.2.2-1`](#rfc7432-9.2.2-1)

If ESI is reserved (0 or MAX-ESI), forwarding state MUST be based on MAC/IP route alone (§9.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.2-1, so no unit is bound to it.

### [`RFC7432-9.2.2-2`](#rfc7432-9.2.2-2)

If ESI is non-reserved, forwarding state for MAC MUST wait until both MAC/IP and Ethernet A-D per ES routes are received (§9.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.2-2, so no unit is bound to it.

### [`RFC7432-10.1-4`](#rfc7432-10.1-4)

Unless all PEs are known to be default gateways, MPLS label in default gateway route MUST be a valid downstream assigned label (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-10.1-4, so no unit is bound to it.

### [`RFC7432-9.2.1-5`](#rfc7432-9.2.1-5)

PE creating MAC forwarding state from received MAC/IP routes MUST enable forwarding to remote destinations (§9.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-9.2.1-5, so no unit is bound to it.

### [`RFC7432-6.2-2`](#rfc7432-6.2-2)

VLAN-bundle service: MAC addresses MUST remain tagged with originating VID in MPLS encapsulation (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.2-2, so no unit is bound to it.

### [`RFC7432-6.3-3`](#rfc7432-6.3-3)

VLAN-aware bundle with VID translation: Ethernet frames MUST remain tagged with normalized VID (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7432-6.3-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7432, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7432, so its obligations are stated where they were written.
