# RFC 4659 - BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN

Partial. Every requirement this repository extracted from RFC 4659, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 28.6% | 2 of 7 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 28.6% | 2 of 7 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 7 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 22 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 9 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 42.9% | 3 of 7 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 7 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 22 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 7 |
| Not applicable, so out of scope | 9 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4659.md` |
| Requirement shard | `rfc/requirements/rfc4659.md` |
| RFC text | `rfc/full/rfc4659.txt` |

## Enrolment

Enrolled: BGP-MPLS IPv6 VPN / VPNv6 (RFC 4659): 2 MET (labeled VPNv6 NLRI, AFI2/SAFI128 capability negotiation) + 2 single-polarity positive (AFI/SAFI set, 24-octet zero-RD global-IPv6 next-hop) + 3 gap (IPv4-mapped-IPv6 next-hop, 48-octet global+link-local next-hop) + 9 not-applicable (data-plane PE tunneling + multi-AS ASBR)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

VPNv6 NLRI (RD + MPLS label + IPv6 prefix) encode/decode, AFI=2/SAFI=128 capability negotiation, and the zero-RD + global-IPv6 24-octet next-hop.

**What the ledger says remains:**

No IPv4-mapped-IPv6 next-hop for IPv4 transport ([`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), 8-4); no 48-octet global+link-local next-hop ([`RFC4659-8-3`](#rfc4659-8-3)). Data-plane PE tunneling (Section 4) and multi-AS ASBR options (Section 8 a/b) are not performed.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4659-3.2-1`](#rfc4659-3.2-1), [`RFC4659-3.4-1`](#rfc4659-3.4-1)

**Annotated instead of tested (14):** [`RFC4659-3.2-2`](#rfc4659-3.2-2), [`RFC4659-4-1`](#rfc4659-4-1), [`RFC4659-4-2`](#rfc4659-4-2), [`RFC4659-4-3`](#rfc4659-4-3), [`RFC4659-4-4`](#rfc4659-4-4), [`RFC4659-4-5`](#rfc4659-4-5), [`RFC4659-4-6`](#rfc4659-4-6), [`RFC4659-4-7`](#rfc4659-4-7), [`RFC4659-8-1`](#rfc4659-8-1), [`RFC4659-8-2`](#rfc4659-8-2), [`RFC4659-8-3`](#rfc4659-8-3), [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1), [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1), [`RFC4659-8-4`](#rfc4659-8-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4659-3.2-1` | PE routers MUST assign and distribute MPLS labels with the IPv6 VPN routes (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/vpn_test.go#L47). **negative:** `unit/verify` [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/vpn_test.go#L81) |
| `RFC4659-3.2-2` | AFI and SAFI fields MUST be set to AFI=2, SAFI=128 (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L597). **negative:** no negative test. **{single-polarity}:** the obligation is to SET AFI=2/SAFI=128 when advertising a VPNv6 route, so the only conforming assertion is that the emitted fields equal 2/128 and a MUST-NOT-set-other-values companion is degenerate (internal/component/bgp/message/update_build_vpn.go:221, internal/component/bgp/plugins/nlri/vpn/types.go:37) |
| `RFC4659-3.4-1` | Two PEs MUST use BGP Capabilities Negotiation (capability code 1, AFI=2, SAFI=128) to ensure both can process VPN-IPv6 NLRIs (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L169). **negative:** `unit/verify` [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L214) |
| `RFC4659-4-1` | The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is an ingress-PE data-plane forwarding behavior; ze is a BGP control-plane speaker with no VPNv6 VRF-to-backbone tunneling path |
| `RFC4659-4-2` | When Next Hop is an IPv4-mapped IPv6 address, ingress PE MUST use IPv4 tunneling unless explicitly configured otherwise (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane transport-selection decision for a forwarding PE; ze performs no VPNv6 data-plane forwarding, and no IPv4-mapped detection exists in the BGP path |
| `RFC4659-4-3` | When Next Hop is not an IPv4-mapped IPv6 address, ingress PE MUST use IPv6 tunneling (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane transport-selection decision for a forwarding PE, a role ze does not perform |
| `RFC4659-4-4` | When tunneling using IPv4, MUST use the IPv4 address encoded in the IPv4-mapped field as the tunnel destination (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane encapsulation behavior; ze installs no VPNv6 IPv4-tunnel forwarding entries |
| `RFC4659-4-5` | When tunneling using IPv6, MUST use the IPv6 address from the Next Hop as the tunnel destination (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a data-plane encapsulation behavior; ze installs no VPNv6 IPv6-tunnel forwarding entries |
| `RFC4659-4-6` | When tunneling using MPLS LSPs, MUST directly push the LSP tunnel label on the label stack (no IP header) (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** an MPLS data-plane label-imposition behavior for a forwarding PE; ze has no VPNv6 VRF-to-LSP forwarding path |
| `RFC4659-4-7` | All systems MUST support tunneling using MPLS LSPs established by LDP (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** binds the ingress-PE VPN-data-over-LDP-LSP forwarding role; ze has LDP label distribution and an MPLS FIB but performs no VPNv6 customer-data forwarding |
| `RFC4659-8-1` | Multi-AS approach (a): Exchange of IPv6 routes MUST be carried out as per RFC 2545 (Section 8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the inter-provider option-A back-to-back-VRF ASBR role; ze has no per-VPN VRF inter-AS exchange, and the referenced RFC 2545 IPv6 next-hop wire behavior is enrolled under its own RFC |
| `RFC4659-8-2` | Multi-AS approach (b): Exchange of labeled VPN-IPv6 routes MUST be carried out as per RFC 2545 and RFC 3107 (Section 8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the inter-provider option-B ASBR label-swap/redistribution role; ze implements the VPNv6 NLRI and next-hop encodings but performs no inter-AS VPN ASBR redistribution |
| `RFC4659-8-3` | Multi-AS approach (b) with IPv6 tunneling: Next Hop Field MUST contain global IPv6 address; when ASBRs share IPv6 subnet, MUST include both global and link-local (Section 8, Section 3.2.1.1) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| `RFC4659-3.2.1.1-1` | When requesting IPv6 transport, BGP speaker SHALL advertise a Next Hop containing a VPN-IPv6 address with zero RD and global IPv6 address (Section 3.2.1.1) | SHALL | 3.2.1.1 | **positive:** `unit/verify` [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L646). **negative:** no negative test. **{single-polarity}:** the obligation is to EMIT a 24-octet zero-RD + global-IPv6 next-hop, which ze produces for a VPNv6 route with an IPv6 next-hop; the decode side is not RD-aware and is not a gated obligation (internal/component/bgp/message/update_build_vpn.go:246, internal/component/bgp/rib/commit.go:498) |
| `RFC4659-3.2.1.2-1` | When requesting IPv4 transport, BGP speaker SHALL advertise a Next Hop containing a VPN-IPv6 address with zero RD and IPv4-mapped IPv6 address (Section 3.2.1.2) | SHALL | 3.2.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze constructs no IPv4-mapped-IPv6 next-hop for VPNv6 -- a plain IPv4 next-hop on a VPNv6 route emits a non-conformant 12-octet zero-RD+IPv4 next-hop, and no ::ffff:a.b.c.d mapping exists in the BGP path (internal/component/bgp/message/update_build_vpn.go:229; Is4In6 appears only in ISIS/OSPF) |
| `RFC4659-8-4` | Multi-AS approach (b) with IPv4 tunneling: Next Hop Field SHALL contain an IPv4-mapped IPv6 address (Section 8) | SHALL | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same missing IPv4-mapped-IPv6 next-hop construction as RFC4659-3.2.1.2-1; ze never emits a zero-RD + ::ffff:a.b.c.d VPNv6 next-hop (internal/component/bgp/message/update_build_vpn.go:246; no IPv4-mapped VPNv6 next-hop producer) |
| `RFC4659-3.2.1.1-2` | Remove link-local from Next Hop when advertising to internal peer not on a common subnet (Section 3.2.1.1) | SHOULD | 3.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-1-1` | Same single set of MP-BGP peering relationships and same PE-PE tunnel mesh MAY be used for both IPv4 and IPv6 VPNs (Section 1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-1` | Same RD MAY be used for IPv6 and IPv4 addresses from the same site (Section 2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-2-2` | Different RD MAY be used for IPv4 and IPv6 addresses (Section 2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-3.1-1` | For IPv6 VPN route distribution, PEs MAY use iBGP over IPv4 or IPv6 (Section 3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4659-4-8` | Ingress PE MAY optionally allow IPv6 tunneling when Next Hop is IPv4-mapped via explicit configuration (Section 4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4659-4-1`](#rfc4659-4-1) The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: this is an ingress-PE data-plane forwarding behavior; ze is a BGP control-plane speaker with no VPNv6 VRF-to-backbone tunneling path |
| [`RFC4659-4-2`](#rfc4659-4-2) When Next Hop is an IPv4-mapped IPv6 address, ingress PE MUST use IPv4 tunneling unless explicitly configured otherwise (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane transport-selection decision for a forwarding PE; ze performs no VPNv6 data-plane forwarding, and no IPv4-mapped detection exists in the BGP path |
| [`RFC4659-4-3`](#rfc4659-4-3) When Next Hop is not an IPv4-mapped IPv6 address, ingress PE MUST use IPv6 tunneling (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane transport-selection decision for a forwarding PE, a role ze does not perform |
| [`RFC4659-4-4`](#rfc4659-4-4) When tunneling using IPv4, MUST use the IPv4 address encoded in the IPv4-mapped field as the tunnel destination (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane encapsulation behavior; ze installs no VPNv6 IPv4-tunnel forwarding entries |
| [`RFC4659-4-5`](#rfc4659-4-5) When tunneling using IPv6, MUST use the IPv6 address from the Next Hop as the tunnel destination (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: a data-plane encapsulation behavior; ze installs no VPNv6 IPv6-tunnel forwarding entries |
| [`RFC4659-4-6`](#rfc4659-4-6) When tunneling using MPLS LSPs, MUST directly push the LSP tunnel label on the label stack (no IP header) (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: an MPLS data-plane label-imposition behavior for a forwarding PE; ze has no VPNv6 VRF-to-LSP forwarding path |
| [`RFC4659-4-7`](#rfc4659-4-7) All systems MUST support tunneling using MPLS LSPs established by LDP (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: binds the ingress-PE VPN-data-over-LDP-LSP forwarding role; ze has LDP label distribution and an MPLS FIB but performs no VPNv6 customer-data forwarding |
| [`RFC4659-8-1`](#rfc4659-8-1) Multi-AS approach (a): Exchange of IPv6 routes MUST be carried out as per RFC 2545 (Section 8) | no test | no test carries this requirement id; annotated {not-applicable}: the inter-provider option-A back-to-back-VRF ASBR role; ze has no per-VPN VRF inter-AS exchange, and the referenced RFC 2545 IPv6 next-hop wire behavior is enrolled under its own RFC |
| [`RFC4659-8-2`](#rfc4659-8-2) Multi-AS approach (b): Exchange of labeled VPN-IPv6 routes MUST be carried out as per RFC 2545 and RFC 3107 (Section 8) | no test | no test carries this requirement id; annotated {not-applicable}: the inter-provider option-B ASBR label-swap/redistribution role; ze implements the VPNv6 NLRI and next-hop encodings but performs no inter-AS VPN ASBR redistribution |
| [`RFC4659-8-3`](#rfc4659-8-3) Multi-AS approach (b) with IPv6 tunneling: Next Hop Field MUST contain global IPv6 address; when ASBRs share IPv6 subnet, MUST include both global and link-local (Section 8, Section 3.2.1.1) | {gap}, no test | ze's VPNv6 next-hop encoder emits only the single global-IPv6 24-octet form and never the 48-octet global+link-local next-hop, so the shared-subnet clause is unmet (internal/component/bgp/message/update_build_vpn.go:229; no 48-octet producer exists) |
| [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1) When requesting IPv4 transport, BGP speaker SHALL advertise a Next Hop containing a VPN-IPv6 address with zero RD and IPv4-mapped IPv6 address (Section 3.2.1.2) | {gap}, no test | ze constructs no IPv4-mapped-IPv6 next-hop for VPNv6 -- a plain IPv4 next-hop on a VPNv6 route emits a non-conformant 12-octet zero-RD+IPv4 next-hop, and no ::ffff:a.b.c.d mapping exists in the BGP path (internal/component/bgp/message/update_build_vpn.go:229; Is4In6 appears only in ISIS/OSPF) |
| [`RFC4659-8-4`](#rfc4659-8-4) Multi-AS approach (b) with IPv4 tunneling: Next Hop Field SHALL contain an IPv4-mapped IPv6 address (Section 8) | {gap}, no test | the same missing IPv4-mapped-IPv6 next-hop construction as RFC4659-3.2.1.2-1; ze never emits a zero-RD + ::ffff:a.b.c.d VPNv6 next-hop (internal/component/bgp/message/update_build_vpn.go:246; no IPv4-mapped VPNv6 next-hop producer) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4659-3.2-1`](#rfc4659-3.2-1)

PE routers MUST assign and distribute MPLS labels with the IPv6 VPN routes (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVPNv6RejectsLabellessEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/vpn_test.go#L81) | unit/verify | unproven |
| positive | [`TestVPNv6WireRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/vpn/vpn_test.go#L47) | unit/verify | unproven |

### [`RFC4659-3.2-2`](#rfc4659-3.2-2)

AFI and SAFI fields MUST be set to AFI=2, SAFI=128 (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L597) | unit/verify | unproven |

### [`RFC4659-3.4-1`](#rfc4659-3.4-1)

Two PEs MUST use BGP Capabilities Negotiation (capability code 1, AFI=2, SAFI=128) to ensure both can process VPN-IPv6 NLRIs (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateWith_VPNv6NotActiveWithoutPeerCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L214) | unit/verify | unproven |
| positive | [`TestOpenAdvertisesVPNv6Capability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_negotiate_test.go#L169) | unit/verify | unproven |

### [`RFC4659-4-1`](#rfc4659-4-1)

The ingress PE Router MUST tunnel IPv6 VPN data over the backbone towards the Egress PE router (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-1, so no unit is bound to it.

### [`RFC4659-4-2`](#rfc4659-4-2)

When Next Hop is an IPv4-mapped IPv6 address, ingress PE MUST use IPv4 tunneling unless explicitly configured otherwise (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-2, so no unit is bound to it.

### [`RFC4659-4-3`](#rfc4659-4-3)

When Next Hop is not an IPv4-mapped IPv6 address, ingress PE MUST use IPv6 tunneling (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-3, so no unit is bound to it.

### [`RFC4659-4-4`](#rfc4659-4-4)

When tunneling using IPv4, MUST use the IPv4 address encoded in the IPv4-mapped field as the tunnel destination (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-4, so no unit is bound to it.

### [`RFC4659-4-5`](#rfc4659-4-5)

When tunneling using IPv6, MUST use the IPv6 address from the Next Hop as the tunnel destination (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-5, so no unit is bound to it.

### [`RFC4659-4-6`](#rfc4659-4-6)

When tunneling using MPLS LSPs, MUST directly push the LSP tunnel label on the label stack (no IP header) (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-6, so no unit is bound to it.

### [`RFC4659-4-7`](#rfc4659-4-7)

All systems MUST support tunneling using MPLS LSPs established by LDP (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-4-7, so no unit is bound to it.

### [`RFC4659-8-1`](#rfc4659-8-1)

Multi-AS approach (a): Exchange of IPv6 routes MUST be carried out as per RFC 2545 (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-1, so no unit is bound to it.

### [`RFC4659-8-2`](#rfc4659-8-2)

Multi-AS approach (b): Exchange of labeled VPN-IPv6 routes MUST be carried out as per RFC 2545 and RFC 3107 (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-2, so no unit is bound to it.

### [`RFC4659-8-3`](#rfc4659-8-3)

Multi-AS approach (b) with IPv6 tunneling: Next Hop Field MUST contain global IPv6 address; when ASBRs share IPv6 subnet, MUST include both global and link-local (Section 8, Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-3, so no unit is bound to it.

### [`RFC4659-3.2.1.1-1`](#rfc4659-3.2.1.1-1)

When requesting IPv6 transport, BGP speaker SHALL advertise a Next Hop containing a VPN-IPv6 address with zero RD and global IPv6 address (Section 3.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUpdateBuilder_BuildVPN_IPv6_NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build_test.go#L646) | unit/verify | unproven |

### [`RFC4659-3.2.1.2-1`](#rfc4659-3.2.1.2-1)

When requesting IPv4 transport, BGP speaker SHALL advertise a Next Hop containing a VPN-IPv6 address with zero RD and IPv4-mapped IPv6 address (Section 3.2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-3.2.1.2-1, so no unit is bound to it.

### [`RFC4659-8-4`](#rfc4659-8-4)

Multi-AS approach (b) with IPv4 tunneling: Next Hop Field SHALL contain an IPv4-mapped IPv6 address (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4659-8-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4659, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4659, so its obligations are stated where they were written.
