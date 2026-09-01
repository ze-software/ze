# RFC 5340 - OSPF for IPv6

Partial. Every requirement this repository extracted from RFC 5340, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 72.7% | 16 of 22 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.5% | 1 of 22 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 33 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 46 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 22.7% | 5 of 22 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 22 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 46 |
| Gated MUST-level | 23 |
| Obligations that bind Ze | 22 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 5 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 33 |
| Tagged units | 33 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5340.md` |
| Requirement shard | `rfc/requirements/rfc5340.md` |
| RFC text | `rfc/full/rfc5340.txt` |

## Enrolment

Enrolled: OSPF for IPv6 (OSPFv3)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Native OSPFv3 as an IPv6 address-family engine sharing the OSPFv2 reactor through the codec seam: the 16-byte common header with Version-3 validation, the IPv6 upper-layer checksum bound to the datagram source/destination, the per-interface Instance ID demux, raw IPv6 protocol 89 with a link-local source and the ff02::5 / ff02::6 groups, per-link (not per-subnet) operation with Router-ID neighbor identity, the scope-typed LS Type registry with link-local-scope Link-LSAs kept on their own link, address-free Router-LSAs and Network-LSAs listing every fully adjacent router, Link-LSAs and Intra-Area-Prefix-LSAs carrying the word-padded prefix encoding, Inter-Area-Prefix / Inter-Area-Router / AS-External / NSSA LSAs, global-scope-only virtual-link endpoints, and the Appendix C.3 positive cost / InfTransDelay and matching HelloInterval / RouterDeadInterval checks. Requirements bound per line in [`rfc/short/rfc5340.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5340.md).

**What the ledger says remains**

Five MUST gaps, annotated in [`rfc/short/rfc5340.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5340.md) and gated by `./le rfc check`. [`RFC5340-2.5-2`](#rfc5340-2.5-2): intra-area-prefix-LSAs exclude link-local addresses, but the ABR inter-area summary path ([`internal/plugins/ospf/origination_v6_summary.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_summary.go)) and the ASBR redistribution path ([`internal/plugins/ospf/origination_v6_external.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/origination_v6_external.go)) apply no link-local filter. [`RFC5340-2.8-2`](#rfc5340-2.8-2): the SPF graph keys a router vertex by Advertising Router and assigns rather than concatenates, so a router that spreads its links across several Router-LSAs is aggregated only to its last one ([`internal/plugins/ospf/afstrategy_v6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/afstrategy_v6.go)). [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1): no destination-address acceptance check -- the datagram destination is used only as checksum pseudo-header input ([`internal/plugins/ospf/dispatcher.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/dispatcher.go)). [`RFC5340-4.9-1`](#rfc5340-4.9-1) and [`RFC5340-4.9-2`](#rfc5340-4.9-2): the §4.9 multiple-interfaces-to-one-link model (Active/Standby, shared Interface Instance ID, standby link-local LSA flush) has no producer. The feature also remains pre-production pending hardening and deployment evidence.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC5340-2.5-1`](#rfc5340-2.5-1), [`RFC5340-2.8-3`](#rfc5340-2.8-3), [`RFC5340-2.8-4`](#rfc5340-2.8-4), [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2), [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1), [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2), [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1), [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5), [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2), [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1), [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2), [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1), [`RFC5340-C.3-1`](#rfc5340-c.3-1), [`RFC5340-C.3-2`](#rfc5340-c.3-2), [`RFC5340-C.3-3`](#rfc5340-c.3-3), [`RFC5340-C.3-4`](#rfc5340-c.3-4)

**Annotated instead of tested (7):** [`RFC5340-2.5-2`](#rfc5340-2.5-2), [`RFC5340-2.8-2`](#rfc5340-2.8-2), [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1), [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2), [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3), [`RFC5340-4.9-1`](#rfc5340-4.9-1), [`RFC5340-4.9-2`](#rfc5340-4.9-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5340-2.5-1` | On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L439). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L475) |
| `RFC5340-2.5-2` | Link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs, AS-external-LSAs, NSSA-LSAs, or intra-area-prefix-LSAs; restated for inter-area-prefix-LSAs in §4.4.3.4 (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the ban holds for intra-area-prefix-LSAs on both origination paths (interfaceIPv6Prefixes, origination_v6.go:564; v6HostPrefixes, origination_v6.go:432; v6AggregatedLinkPrefixes, origination_v6_link.go:153), but the ABR summary path copies every prefix out of a received intra-area-prefix-LSA into an inter-area-prefix-LSA with no link-local filter (v6SummaryNetworks, origination_v6_summary.go:136-147), and the ASBR path wire-encodes a redistributed prefix with no link-local filter either (v6InjectExternal -> netipToV6Prefix, origination_v6_external.go:55 and origination_v6.go:592), so a link-local supplied by a peer or by redistribution reaches an inter-area-prefix-LSA / AS-external-LSA / NSSA-LSA. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-2` | Receivers MUST concatenate all the router-LSAs originated by a given router, treating them as a single aggregate, when running the SPF calculation; reaffirmed in §4.8 and §4.8.1 (§2.8) | MUST | 2.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPFv3 graph build keys a router vertex by Advertising Router alone and ASSIGNS rather than concatenates, so a second Router-LSA from the same router (a different Link State ID) replaces the first instead of aggregating its links (v6Strategy.BuildGraph, afstrategy_v6.go:113-117). Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-3` | A network-LSA MUST list all routers connected to the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L183). **negative:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L186) |
| `RFC5340-2.8-4` | A link-LSA MUST list all of a router's addresses on the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L221). **negative:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L225) |
| `RFC5340-4.1.2-2` | A virtual link MUST use one of the router's own global-scope IPv6 addresses as its IP interface address, instead of a link-local address; also §4.7 (§4.1.2) | MUST | 4.1.2 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L444). **negative:** `unit/verify` [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) |
| `RFC5340-4.2.1.1-1` | Before a Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74). **negative:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) |
| `RFC5340-4.2.1.1-2` | The Options bits that MUST be set correctly in Hello packets are the E-bit (regular area), N-bit (NSSA area), and DC-bit (demand circuit) (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119). **negative:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) |
| `RFC5340-4.2.1.2-1` | The Options bits that MUST be set correctly in Database Description packets include the DC-bit for demand circuits (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154). **negative:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L158) |
| `RFC5340-4.2.2-1` | A received packet's IP destination address MUST be a unicast address of the receiving interface, the AllSPFRouters or AllDRouters multicast address, or (for virtual links) an IPv6 global address (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no destination-address acceptance check exists. The v3 backend records the datagram destination from the IPV6_PKTINFO control message (backend_linux.go:307) and the dispatcher consumes it ONLY as pseudo-header input to the checksum (dispatcher.go:65), never comparing it against the interface's addresses or ff02::5 / ff02::6; grep for `.Dst` over internal/plugins/ospf finds no other reader. A protocol-89 datagram sent to a group the host already joins (for example ff02::1) is therefore accepted, since its sender computed a checksum for that same destination. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.2.2-2` | The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13). **negative:** no negative test. **{single-polarity}:** the transport opens its raw socket on "ip6:89" (listenNetwork, v3/transport/backend_linux.go:28), so the kernel stamps Next Header 89 on every send and demultiplexes only Next Header 89 to this socket on receive. ze never sees a non-89 datagram, so it has no reject path of its own to exercise |
| `RFC5340-4.2.2-3` | Any encapsulating IP Authentication Headers and IP Encapsulating Security Payloads MUST be processed and/or verified to ensure integrity and authentication/confidentiality (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** AH and ESP are processed by the kernel XFRM inbound transform before the datagram reaches the socket; ze installs the require-policy that makes that happen (buildIPsecPolicies SADirIn, ipsec_install.go:449) and only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32). ze never parses or verifies an AH/ESP header, matching the RFC 4552 rows for the same delegation |
| `RFC5340-4.2.2-5` | The version number field MUST specify protocol version 3 (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/header_test.go#L31). **negative:** `unit/verify` [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/header_test.go#L80) |
| `RFC5340-4.9-1` | Each of a router's multiple interfaces to a single link MUST be configured with the same Interface Instance ID to be considered on the same link (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.9-2` | When a Standby Interface goes down, the link-local scope LSAs originated for it MUST be flushed on the Active Interface (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-A.3.1-2` | The reserved header fields MUST be ignored when receiving protocol packets (§A.3.1) | MUST | A.3.1 | **positive:** `unit/verify` [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L366). **negative:** `unit/verify` [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L370) |
| `RFC5340-A.4.7-1` | An AS-external-LSA forwarding address MUST NOT be set to the IPv6 Unspecified Address or an IPv6 Link-Local Address (§A.4.7) | MUST NOT | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L292). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L294) |
| `RFC5340-A.4.7-2` | An OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address (§A.4.7) | MUST | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L304) |
| `RFC5340-A.4.8-1` | A global IPv6 address MUST be selected as the forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers (§A.4.8) | MUST | A.4.8 | **positive:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L336). **negative:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L339) |
| `RFC5340-C.3-1` | The interface output cost MUST always be greater than 0 (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L423). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L426) |
| `RFC5340-C.3-2` | InfTransDelay MUST be greater than 0 (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L429). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L432) |
| `RFC5340-C.3-3` | HelloInterval MUST be the same for all routers attached to a common link (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L465). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L468) |
| `RFC5340-C.3-4` | RouterDeadInterval MUST be the same for all routers attached to a common link (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L471). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L473) |
| `RFC5340-2.11-1` | The Router ID of 0.0.0.0 is reserved and SHOULD NOT be used; restated in §C.1 (§2.11) | SHOULD NOT | 2.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.2.2-4` | If the OSPF header fields do not match those configured for the receiving OSPFv3 interface, the packet SHOULD be discarded (§4.2.2) | SHOULD | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.2.2-6` | Locally originated packets SHOULD NOT be processed by OSPF, except in support of multiple interfaces attached to the same link per §4.9 (§4.2.2) | SHOULD NOT | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.8-1` | A link-LSA SHOULD NOT be originated for a virtual link, which has no link-local address or associated prefixes; also §4.7 (§4.4.3.8) | SHOULD NOT | 4.4.3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-2` | When building an intra-area-prefix-LSA, prefixes having the NU-bit and/or LA-bit set in their PrefixOptions SHOULD NOT be copied, nor should link-local addresses (§4.4.3.9) | SHOULD NOT | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-3` | Prefixes that would normally have the LA-bit set SHOULD be advertised independent of whether the interface is advertised as a transit link (§4.4.3.9) | SHOULD | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.8.1-1` | A prefix advertisement whose NU-bit is set SHOULD NOT be included in the routing calculation (§4.8.1) | SHOULD NOT | 4.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.9-3` | When the Active Interface fails, the new Active Interface SHOULD form all new neighbor adjacencies with routers on the link (§4.9) | SHOULD | 4.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.1-1` | The OSPF IP protocol number 89 SHOULD be inserted in the Next Header field of the encapsulating IPv6 header (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.1-2` | Where the IPv6 Traffic Class is mapped to DSCP, OSPFv3 packets SHOULD be sent with their DSCP set to CS6 (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.3.1-1` | The reserved header fields SHOULD be set to 0 when sending protocol packets (§A.3.1) | SHOULD | A.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-2.8-1` | Router interface information MAY be spread across multiple router-LSAs (§2.8) | MAY | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.1.2-1` | An implementation MAY use the MIB-II IfIndex as the Interface ID (§4.1.2) | MAY | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.2-1` | A router MAY originate one or more router-LSAs for a given area (§4.4.3.2) | MAY | 4.4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-1` | A router MAY originate multiple intra-area-prefix-LSAs for a given area (§4.4.3.9) | MAY | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.4-1` | Reachability validation for future non-SPF LSA types MAY be done less frequently than every SPF calculation (§4.4.4) | MAY | 4.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.5.2-1` | All LS types MAY not be understood by all routers (§4.5.2) | MAY | 4.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.5.2-2` | A new LSA type with its U-bit set to 0 MAY only be understood by a subset of routers (§4.5.2) | MAY | 4.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.3.6-1` | Multiple LSAs MAY be acknowledged in a single Link State Acknowledgment packet (§A.3.6) | MAY | A.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.1.1-1` | An implementation MAY also set the LA-bit for prefixes advertised with a host PrefixLength (128) (§A.4.1.1) | MAY | A.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.7-3` | The External Route Tag is a 32-bit field that MAY be used to communicate additional information between AS boundary routers (§A.4.7) | MAY | A.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.7-4` | All, none, or some of the Forwarding Address, External Route Tag, and Referenced Link State ID fields MAY be present in the AS-external-LSA (§A.4.7) | MAY | A.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-C.3-5` | Future interface types MAY specify a different default for LinkLSASuppression (§C.3) | MAY | C.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5340-2.5-2`](#rfc5340-2.5-2) Link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs, AS-external-LSAs, NSSA-LSAs, or intra-area-prefix-LSAs; restated for inter-area-prefix-LSAs in §4.4.3.4 (§2.5) | {gap}, no test | the ban holds for intra-area-prefix-LSAs on both origination paths (interfaceIPv6Prefixes, origination_v6.go:564; v6HostPrefixes, origination_v6.go:432; v6AggregatedLinkPrefixes, origination_v6_link.go:153), but the ABR summary path copies every prefix out of a received intra-area-prefix-LSA into an inter-area-prefix-LSA with no link-local filter (v6SummaryNetworks, origination_v6_summary.go:136-147), and the ASBR path wire-encodes a redistributed prefix with no link-local filter either (v6InjectExternal -> netipToV6Prefix, origination_v6_external.go:55 and origination_v6.go:592), so a link-local supplied by a peer or by redistribution reaches an inter-area-prefix-LSA / AS-external-LSA / NSSA-LSA. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-2.8-2`](#rfc5340-2.8-2) Receivers MUST concatenate all the router-LSAs originated by a given router, treating them as a single aggregate, when running the SPF calculation; reaffirmed in §4.8 and §4.8.1 (§2.8) | {gap}, no test | the OSPFv3 graph build keys a router vertex by Advertising Router alone and ASSIGNS rather than concatenates, so a second Router-LSA from the same router (a different Link State ID) replaces the first instead of aggregating its links (v6Strategy.BuildGraph, afstrategy_v6.go:113-117). Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1) A received packet's IP destination address MUST be a unicast address of the receiving interface, the AllSPFRouters or AllDRouters multicast address, or (for virtual links) an IPv6 global address (§4.2.2) | {gap}, no test | no destination-address acceptance check exists. The v3 backend records the datagram destination from the IPV6_PKTINFO control message (backend_linux.go:307) and the dispatcher consumes it ONLY as pseudo-header input to the checksum (dispatcher.go:65), never comparing it against the interface's addresses or ff02::5 / ff02::6; grep for `.Dst` over internal/plugins/ospf finds no other reader. A protocol-89 datagram sent to a group the host already joins (for example ff02::1) is therefore accepted, since its sender computed a checksum for that same destination. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3) Any encapsulating IP Authentication Headers and IP Encapsulating Security Payloads MUST be processed and/or verified to ensure integrity and authentication/confidentiality (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: AH and ESP are processed by the kernel XFRM inbound transform before the datagram reaches the socket; ze installs the require-policy that makes that happen (buildIPsecPolicies SADirIn, ipsec_install.go:449) and only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32). ze never parses or verifies an AH/ESP header, matching the RFC 4552 rows for the same delegation |
| [`RFC5340-4.9-1`](#rfc5340-4.9-1) Each of a router's multiple interfaces to a single link MUST be configured with the same Interface Instance ID to be considered on the same link (§4.9) | {gap}, no test | ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.9-2`](#rfc5340-4.9-2) When a Standby Interface goes down, the link-local scope LSAs originated for it MUST be flushed on the Active Interface (§4.9) | {gap}, no test | there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5340-2.5-1`](#rfc5340-2.5-1)

On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L475) | unit/verify | unproven |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L439) | unit/verify | unproven |

### [`RFC5340-2.5-2`](#rfc5340-2.5-2)

Link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs, AS-external-LSAs, NSSA-LSAs, or intra-area-prefix-LSAs; restated for inter-area-prefix-LSAs in §4.4.3.4 (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-2.5-2, so no unit is bound to it.

### [`RFC5340-2.8-2`](#rfc5340-2.8-2)

Receivers MUST concatenate all the router-LSAs originated by a given router, treating them as a single aggregate, when running the SPF calculation; reaffirmed in §4.8 and §4.8.1 (§2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-2.8-2, so no unit is bound to it.

### [`RFC5340-2.8-3`](#rfc5340-2.8-3)

A network-LSA MUST list all routers connected to the link (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L186) | unit/verify | unproven |
| positive | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L183) | unit/verify | unproven |

### [`RFC5340-2.8-4`](#rfc5340-2.8-4)

A link-LSA MUST list all of a router's addresses on the link (§2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L225) | unit/verify | unproven |
| positive | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L221) | unit/verify | unproven |

### [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2)

A virtual link MUST use one of the router's own global-scope IPv6 addresses as its IP interface address, instead of a link-local address; also §4.7 (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) | unit/verify | unproven |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L444) | unit/verify | unproven |

### [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1)

Before a Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74) | unit/verify | unproven |

### [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2)

The Options bits that MUST be set correctly in Hello packets are the E-bit (regular area), N-bit (NSSA area), and DC-bit (demand circuit) (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) | unit/verify | unproven |
| positive | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119) | unit/verify | unproven |

### [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1)

The Options bits that MUST be set correctly in Database Description packets include the DC-bit for demand circuits (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L158) | unit/verify | unproven |
| positive | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154) | unit/verify | unproven |

### [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1)

A received packet's IP destination address MUST be a unicast address of the receiving interface, the AllSPFRouters or AllDRouters multicast address, or (for virtual links) an IPv6 global address (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.2.2-1, so no unit is bound to it.

### [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2)

The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13) | unit/verify | unproven |

### [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3)

Any encapsulating IP Authentication Headers and IP Encapsulating Security Payloads MUST be processed and/or verified to ensure integrity and authentication/confidentiality (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.2.2-3, so no unit is bound to it.

### [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5)

The version number field MUST specify protocol version 3 (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/header_test.go#L80) | unit/verify | unproven |
| positive | [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/header_test.go#L31) | unit/verify | unproven |

### [`RFC5340-4.9-1`](#rfc5340-4.9-1)

Each of a router's multiple interfaces to a single link MUST be configured with the same Interface Instance ID to be considered on the same link (§4.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.9-1, so no unit is bound to it.

### [`RFC5340-4.9-2`](#rfc5340-4.9-2)

When a Standby Interface goes down, the link-local scope LSAs originated for it MUST be flushed on the Active Interface (§4.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.9-2, so no unit is bound to it.

### [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2)

The reserved header fields MUST be ignored when receiving protocol packets (§A.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L370) | unit/verify | unproven |
| positive | [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L366) | unit/verify | unproven |

### [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1)

An AS-external-LSA forwarding address MUST NOT be set to the IPv6 Unspecified Address or an IPv6 Link-Local Address (§A.4.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L294) | unit/verify | unproven |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L292) | unit/verify | unproven |

### [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2)

An OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address (§A.4.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L304) | unit/verify | unproven |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300) | unit/verify | unproven |

### [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1)

A global IPv6 address MUST be selected as the forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers (§A.4.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L339) | unit/verify | unproven |
| positive | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L336) | unit/verify | unproven |

### [`RFC5340-C.3-1`](#rfc5340-c.3-1)

The interface output cost MUST always be greater than 0 (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L426) | unit/verify | unproven |
| positive | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L423) | unit/verify | unproven |

### [`RFC5340-C.3-2`](#rfc5340-c.3-2)

InfTransDelay MUST be greater than 0 (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L432) | unit/verify | unproven |
| positive | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L429) | unit/verify | unproven |

### [`RFC5340-C.3-3`](#rfc5340-c.3-3)

HelloInterval MUST be the same for all routers attached to a common link (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L468) | unit/verify | unproven |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L465) | unit/verify | unproven |

### [`RFC5340-C.3-4`](#rfc5340-c.3-4)

RouterDeadInterval MUST be the same for all routers attached to a common link (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L473) | unit/verify | unproven |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L471) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5340, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5340, so its obligations are stated where they were written.
