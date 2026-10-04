# RFC 5340 - OSPF for IPv6

Partial. Every requirement this repository extracted from RFC 5340, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 69.6% | 16 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.3% | 1 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 23 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 35.6% | 16 of 45 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 46 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 23 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 4.3% | 1 of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 23 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 23 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 21.7% | 5 of 23 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 23 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 46 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 45 |
| Tagged units | 45 |
| Recorded audit verdicts | 13 |
| Discrimination records | 16 |
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
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC5340-2.5-1`](#rfc5340-2.5-1), [`RFC5340-2.8-3`](#rfc5340-2.8-3), [`RFC5340-2.8-4`](#rfc5340-2.8-4), [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2), [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1), [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2), [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1), [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5), [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2), [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1), [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2), [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1), [`RFC5340-C.3-1`](#rfc5340-c.3-1), [`RFC5340-C.3-2`](#rfc5340-c.3-2), [`RFC5340-C.3-3`](#rfc5340-c.3-3), [`RFC5340-C.3-4`](#rfc5340-c.3-4)

**Annotated (including scoped evidence) (7):** [`RFC5340-2.5-2`](#rfc5340-2.5-2), [`RFC5340-2.8-2`](#rfc5340-2.8-2), [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1), [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2), [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3), [`RFC5340-4.9-1`](#rfc5340-4.9-1), [`RFC5340-4.9-2`](#rfc5340-4.9-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5340-2.5-1` | On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L519). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L555) |
| `RFC5340-2.5-2` | In particular, link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs (Section 4.4.3.4), AS-external-LSAs (Section 4.4.3.6), NSSA-LSAs (Section 4.4.3.7), or intra-area-prefix- LSAs (Section 4.4.3.9). (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the ban holds for intra-area-prefix-LSAs on both origination paths (interfaceIPv6Prefixes, origination_v6.go:564; v6HostPrefixes, origination_v6.go:432; v6AggregatedLinkPrefixes, origination_v6_link.go:153), but the ABR summary path copies every prefix out of a received intra-area-prefix-LSA into an inter-area-prefix-LSA with no link-local filter (v6SummaryNetworks, origination_v6_summary.go:136-147), and the ASBR path wire-encodes a redistributed prefix with no link-local filter either (v6InjectExternal -> netipToV6Prefix, origination_v6_external.go:55 and origination_v6.go:592), so a link-local supplied by a peer or by redistribution reaches an inter-area-prefix-LSA / AS-external-LSA / NSSA-LSA. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-2` | Receivers MUST concatenate all the router-LSAs originated by a given router when running the SPF calculation. (§2.8) | MUST | 2.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPFv3 graph build keys a router vertex by Advertising Router alone and ASSIGNS rather than concatenates, so a second Router-LSA from the same router (a different Link State ID) replaces the first instead of aggregating its links (v6Strategy.BuildGraph, afstrategy_v6.go:113-117). Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-3` | a network-LSA MUST list all routers connected to the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L189). **negative:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L192) |
| `RFC5340-2.8-4` | a link-LSA MUST list all of a router's addresses on the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L227). **negative:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L231) |
| `RFC5340-4.1.2-2` | For IPv6, the IPv6 address appearing in the source of OSPF packets sent on the interface is almost always a link-local address. The one exception is for virtual links that MUST use one of the router's own global IPv6 addresses as IP interface address. (§4.1.2) | MUST | 4.1.2 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L524). **negative:** `unit/verify` [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) |
| `RFC5340-4.2.1.1-1` | Before the Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74). **negative:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) |
| `RFC5340-4.2.1.1-2` | Those that MUST be set correctly in Hello packets are as follows. The E-bit is set if and only if the interface attaches to a regular area, i.e., not a stub or NSSA area. Similarly, the N-bit is set if and only if the interface attaches to an NSSA area (see [NSSA]). Finally, the DC-bit is set if and only if the router wishes to suppress the sending of future Hellos over the interface (see [DEMAND]). (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119). **negative:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) |
| `RFC5340-4.2.1.2-1` | Those that MUST be set correctly in Database Description packets are as follows. The DC-bit is set if and only if the router wishes to suppress the sending of Hellos over the interface (see [DEMAND]). (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154). **negative:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L160) |
| `RFC5340-4.2.2-1` | In order for the packet to be passed to OSPF for processing, the following tests must be performed on the encapsulating IPv6 headers: o The packet's IP destination address MUST be one of the IPv6 unicast addresses associated with the receiving interface (this includes link-local addresses), one of the IPv6 multicast addresses AllSPFRouters or AllDRouters, or an IPv6 global address (for virtual links). (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no destination-address acceptance check exists. The v3 backend records the datagram destination from the IPV6_PKTINFO control message (backend_linux.go:307) and the dispatcher consumes it ONLY as pseudo-header input to the checksum (dispatcher.go:65), never comparing it against the interface's addresses or ff02::5 / ff02::6; grep for `.Dst` over internal/plugins/ospf finds no other reader. A protocol-89 datagram sent to a group the host already joins (for example ff02::1) is therefore accepted, since its sender computed a checksum for that same destination. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.2.2-2` | The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13). **negative:** no negative test. **{single-polarity}:** the transport opens its raw socket on "ip6:89" (listenNetwork, v3/transport/backend_linux.go:28), so the kernel stamps Next Header 89 on every send and demultiplexes only Next Header 89 to this socket on receive. ze never sees a non-89 datagram, so it has no reject path of its own to exercise |
| `RFC5340-4.2.2-3` | o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** AH and ESP are processed by the kernel XFRM inbound transform before the datagram reaches the socket; ze installs the require-policy that makes that happen (buildIPsecPolicies SADirIn, ipsec_install.go:449) and only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32). ze never parses or verifies an AH/ESP header, matching the RFC 4552 rows for the same delegation |
| `RFC5340-4.2.2-5` | The version number field MUST specify protocol version 3 (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L31). **negative:** `unit/verify` [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L80) |
| `RFC5340-4.9-1` | o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.9-2` | If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-A.3.1-2` | These fields are reserved. They SHOULD be set to 0 when sending protocol packets and MUST be ignored when receiving protocol packets. (§A.3.1) | MUST | A.3.1 | **positive:** `unit/verify` [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L373). **positive:** `unit/verify` [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L25). **negative:** `unit/verify` [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L377). **negative:** `unit/verify` [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L29) |
| `RFC5340-A.4.7-1` | Forwarding address A fully qualified IPv6 address (128 bits). Included in the LSA if and only if bit F has been set. If included, data traffic for the advertised destination will be forwarded to this address. It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10). (§A.4.7) | MUST NOT | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L298). **positive:** `unit/verify` [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L45). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300). **negative:** `unit/verify` [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L56) |
| `RFC5340-A.4.7-2` | While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address. (§A.4.7) | MUST | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L306). **positive:** `unit/verify` [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L67). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L310). **negative:** `unit/verify` [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L74) |
| `RFC5340-A.4.8-1` | A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers. (§A.4.8) | MUST | A.4.8 | **positive:** `unit/verify` [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L78). **positive:** `unit/verify` [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L62). **positive:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L342). **negative:** `unit/verify` [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L68). **negative:** `unit/verify` [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L50). **negative:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L345) |
| `RFC5340-C.3-1` | The interface output cost MUST always be greater than 0 (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L430). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L433) |
| `RFC5340-C.3-2` | InfTransDelay The estimated number of seconds it takes to transmit a Link State Update packet over this interface. LSAs contained in the update packet must have their age incremented by this amount before transmission. This value should take into account the transmission and propagation delays of the interface. It MUST be greater than 0. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L66). **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L436). **negative:** `unit/verify` [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L73). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L439) |
| `RFC5340-C.3-3` | HelloInterval The length of time, in seconds, between Hello packets that the router sends on the interface. This value is advertised in the router's Hello packets. It MUST be the same for all routers attached to a common link. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L472). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L475) |
| `RFC5340-C.3-4` | RouterDeadInterval After ceasing to hear a router's Hello packets, the number of seconds before its neighbors declare the router down. This is also advertised in the router's Hello packets in their RouterDeadInterval field. This should be some multiple of the HelloInterval (e.g., 4). This value again MUST be the same for all routers attached to a common link. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L478). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L480) |
| `RFC5340-2.11-1` | The Router ID of 0.0.0.0 is reserved and SHOULD NOT be used. (§2.11) | SHOULD NOT | 2.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.2.2-4` | The fields specified in the header must match those configured for the receiving OSPFv3 interface. If they do not, the packet SHOULD be discarded (§4.2.2) | SHOULD | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.2.2-6` | Locally originated packets SHOULD NOT be processed by OSPF except for support of multiple interfaces attached to the same link as described in Section 4.9. (§4.2.2) | SHOULD NOT | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.8-1` | Hence, a link-LSA SHOULD NOT be originated for a virtual link since the virtual link has no link-local address or associated prefixes. (§4.7) | SHOULD NOT | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-2` | Prefixes having the NU-bit and/or LA-bit set in their Options field SHOULD NOT be copied, nor should link-local addresses be copied. (§4.4.3.9) | SHOULD NOT | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-3` | However, any prefixes that would normally have the LA-bit set SHOULD be advertised independent of whether or not the interface is advertised as a transit link. (§4.4.3.9) | SHOULD | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.8.1-1` | A prefix advertisement whose NU-bit is set SHOULD NOT be included in the routing calculation (§4.8.1) | SHOULD NOT | 4.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.9-3` | If the Active Interface fails, a new Active Interface will have to take over. The new Active Interface SHOULD form all new neighbor adjacencies with routers on the link. (§4.9) | SHOULD | 4.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.1-1` | OSPF is IP protocol 89. This number SHOULD be inserted in the Next Header field of the encapsulating IPv6 header. (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.1-2` | If routers in the OSPF routing domain map their IPv6 Traffic Class octet to the Differentiated Services Code Point (DSCP) as specified in [DIFF-SERV], then OSPFv3 packets SHOULD be sent with their DSCP set to CS6 (B'110000'), as specified in [SERV-CLASS]. (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.3.1-1` | These fields are reserved. They SHOULD be set to 0 when sending protocol packets (§A.3.1) | SHOULD | A.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-2.8-1` | Router interface information MAY be spread across multiple router- LSAs. (§2.8) | MAY | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.1.2-1` | For example, some implementations MAY be able to use the MIB-II IfIndex ([INTFMIB]) as the Interface ID. (§4.1.2) | MAY | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.2-1` | A router MAY originate one or more router- LSAs for a given area. (§4.4.3.2) | MAY | 4.4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.3.9-1` | A router MAY originate multiple intra-area-prefix-LSAs for a given area (§4.4.3.9) | MAY | 4.4.3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.4.4-1` | In general, the new information advertised in future LSAs should not be used unless the OSPFv3 router originating the LSA is reachable. However, depending on the application and the data advertised, this reachability validation MAY be done less frequently than every SPF calculation. (§4.4.4) | MAY | 4.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.5.2-1` | However, all LS types MAY not be understood by all routers. (§4.5.2) | MAY | 4.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-4.5.2-2` | For example, a new LSA type with its U-bit set to 0 MAY only be understood by a subset of routers. (§4.5.2) | MAY | 4.5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.3.6-1` | Multiple LSAs MAY be acknowledged in a single Link State Acknowledgment packet (§A.3.6) | MAY | A.3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.1.1-1` | An implementation MAY also set the LA-bit for prefixes advertised with a host PrefixLength (128) (§A.4.1.1) | MAY | A.4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.7-3` | External Route Tag A 32-bit field that MAY be used to communicate additional information between AS boundary routers. (§A.4.7) | MAY | A.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-A.4.7-4` | All, none, or some of the fields labeled Forwarding address, External Route Tag, and Referenced Link State ID MAY be present in the AS- external-LSA (as indicated by the setting of bit F, bit T, and Referenced LS Type respectively). (§A.4.7) | MAY | A.4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5340-C.3-5` | The default value is "disabled" for interface types described in this specification. It is implicitly "disabled" if the interface type is broadcast or NBMA. Future interface types MAY specify a different default. (§C.3) | MAY | C.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5340-2.5-2`](#rfc5340-2.5-2) In particular, link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs (Section 4.4.3.4), AS-external-LSAs (Section 4.4.3.6), NSSA-LSAs (Section 4.4.3.7), or intra-area-prefix- LSAs (Section 4.4.3.9). (§2.5) | {gap}, no test | the ban holds for intra-area-prefix-LSAs on both origination paths (interfaceIPv6Prefixes, origination_v6.go:564; v6HostPrefixes, origination_v6.go:432; v6AggregatedLinkPrefixes, origination_v6_link.go:153), but the ABR summary path copies every prefix out of a received intra-area-prefix-LSA into an inter-area-prefix-LSA with no link-local filter (v6SummaryNetworks, origination_v6_summary.go:136-147), and the ASBR path wire-encodes a redistributed prefix with no link-local filter either (v6InjectExternal -> netipToV6Prefix, origination_v6_external.go:55 and origination_v6.go:592), so a link-local supplied by a peer or by redistribution reaches an inter-area-prefix-LSA / AS-external-LSA / NSSA-LSA. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-2.8-2`](#rfc5340-2.8-2) Receivers MUST concatenate all the router-LSAs originated by a given router when running the SPF calculation. (§2.8) | {gap}, no test | the OSPFv3 graph build keys a router vertex by Advertising Router alone and ASSIGNS rather than concatenates, so a second Router-LSA from the same router (a different Link State ID) replaces the first instead of aggregating its links (v6Strategy.BuildGraph, afstrategy_v6.go:113-117). Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1) In order for the packet to be passed to OSPF for processing, the following tests must be performed on the encapsulating IPv6 headers: o The packet's IP destination address MUST be one of the IPv6 unicast addresses associated with the receiving interface (this includes link-local addresses), one of the IPv6 multicast addresses AllSPFRouters or AllDRouters, or an IPv6 global address (for virtual links). (§4.2.2) | {gap}, no test | no destination-address acceptance check exists. The v3 backend records the datagram destination from the IPV6_PKTINFO control message (backend_linux.go:307) and the dispatcher consumes it ONLY as pseudo-header input to the checksum (dispatcher.go:65), never comparing it against the interface's addresses or ff02::5 / ff02::6; grep for `.Dst` over internal/plugins/ospf finds no other reader. A protocol-89 datagram sent to a group the host already joins (for example ff02::1) is therefore accepted, since its sender computed a checksum for that same destination. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3) o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: AH and ESP are processed by the kernel XFRM inbound transform before the datagram reaches the socket; ze installs the require-policy that makes that happen (buildIPsecPolicies SADirIn, ipsec_install.go:449) and only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go:32). ze never parses or verifies an AH/ESP header, matching the RFC 4552 rows for the same delegation |
| [`RFC5340-4.9-1`](#rfc5340-4.9-1) o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9) | {gap}, no test | ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.9-2`](#rfc5340-4.9-2) If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9) | {gap}, no test | there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5340-2.5-1`](#rfc5340-2.5-1)

On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L555) | unit/verify | unproven |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L519) | unit/verify | unproven |

### [`RFC5340-2.5-2`](#rfc5340-2.5-2)

In particular, link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs (Section 4.4.3.4), AS-external-LSAs (Section 4.4.3.6), NSSA-LSAs (Section 4.4.3.7), or intra-area-prefix- LSAs (Section 4.4.3.9). (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-2.5-2, so no unit is bound to it.

### [`RFC5340-2.8-2`](#rfc5340-2.8-2)

Receivers MUST concatenate all the router-LSAs originated by a given router when running the SPF calculation. (§2.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-2.8-2, so no unit is bound to it.

### [`RFC5340-2.8-3`](#rfc5340-2.8-3)

a network-LSA MUST list all routers connected to the link (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Network-LSA that omits a router attached to the link. TestRFC5340NetworkLSAListsAttachedRouters drives v6OriginateNetwork, decodes the installed Network-LSA and asserts got[self] (the DR) and got[full] (a Full neighbor), each red on omission; got[twoWay] false and Len 2 bound the list to the routers RFC 2328 12.4.2 counts as attached (fully adjacent), which RFC 5340 leaves unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L189) | unit/verify | unproven |

### [`RFC5340-2.8-4`](#rfc5340-2.8-4)

a link-LSA MUST list all of a router's addresses on the link (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Link-LSA that omits one of the router's addresses on the link. TestRFC5340LinkLSAListsLinkAddresses decodes the originated Link-LSA and asserts the link-local fe80::1 and both global prefixes 2001:db8:1::/64 and 2001:db8:2::/64, each red on omission; the eth1 half asserts each link's LSA carries its own link-local and prefix and not the other link's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L231) | unit/verify | unproven |
| positive | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L227) | unit/verify | unproven |

### [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2)

For IPv6, the IPv6 address appearing in the source of OSPF packets sent on the interface is almost always a link-local address. The one exception is for virtual links that MUST use one of the router's own global IPv6 addresses as IP interface address. (§4.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a virtual link using anything but one of this router's own global IPv6 addresses as its IP interface address. TestV6VirtualEndpointResolvesGlobalAddress asserts v6ResolveVirtualEndpointLocked returns this router's global 2001:db8:1::1 as source (red on another address); TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress asserts that with only fe80::1 advertised the endpoint does not resolve (red if a link-local is adopted). The 'almost always a link-local' sentence is descriptive and states no obligation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) | unit/verify | unproven |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L524) | unit/verify | unproven |

### [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1)

Before the Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet. (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Hello sent without this interface's own Interface ID. TestRFC5340HelloCarriesInterfaceID sends real Hellos through SendHello and the v6 encoder and asserts Interface ID 42 on eth0 and 43 on eth1 (red on a missing or shared value), and after a neighbor Hello carrying 99 asserts the next sent Hello still carries 42 (red if the neighbor's value is copied in).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) | unit/verify | unproven |
| positive | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74) | unit/verify | unproven |

### [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2)

Those that MUST be set correctly in Hello packets are as follows. The E-bit is set if and only if the interface attaches to a regular area, i.e., not a stub or NSSA area. Similarly, the N-bit is set if and only if the interface attaches to an NSSA area (see [NSSA]). Finally, the DC-bit is set if and only if the router wishes to suppress the sending of future Hellos over the interface (see [DEMAND]). (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Hello whose E, N or DC bit does not follow the area type or the wish to suppress Hellos. TestRFC5340HelloOptionsBits sends Hellos through the real send path and asserts E set and N clear for a regular area, N set and E clear for NSSA, both clear for stub (both directions of each 'if and only if'), and DC zero in all three; ze has no demand-circuit support and never wishes to suppress Hellos, so DC clear is the only correct value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) | unit/verify | unproven |
| positive | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119) | unit/verify | unproven |

### [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1)

Those that MUST be set correctly in Database Description packets are as follows. The DC-bit is set if and only if the router wishes to suppress the sending of Hellos over the interface (see [DEMAND]). (§4.2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): TestRFC5340DBDescOptionsBits tags now claim only the DC-bit: DDs of a regular and a stub area carry DC clear (Ze suppresses no Hellos, so 'if and only if' requires clear), and neutral Options carrying DC handed to the encoder still yield DC clear. Revert records on encoder_v6.go::neutralToV6Options observed red. The 'set when suppressing' direction has no Ze path (no demand circuits).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154) | unit/verify | revert, verified |

### [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1)

In order for the packet to be passed to OSPF for processing, the following tests must be performed on the encapsulating IPv6 headers: o The packet's IP destination address MUST be one of the IPv6 unicast addresses associated with the receiving interface (this includes link-local addresses), one of the IPv6 multicast addresses AllSPFRouters or AllDRouters, or an IPv6 global address (for virtual links). (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.2.2-1, so no unit is bound to it.

### [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2)

The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13) | unit/verify | unproven |

### [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3)

o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.2.2-3, so no unit is bound to it.

### [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5)

The version number field MUST specify protocol version 3 (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L80) | unit/verify | unproven |
| positive | [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L31) | unit/verify | unproven |

### [`RFC5340-4.9-1`](#rfc5340-4.9-1)

o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.9-1, so no unit is bound to it.

### [`RFC5340-4.9-2`](#rfc5340-4.9-2)

If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5340-4.9-2, so no unit is bound to it.

### [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2)

These fields are reserved. They SHOULD be set to 0 when sending protocol packets and MUST be ignored when receiving protocol packets. (§A.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): receive clause unchanged (TestRFC5340ReservedHeaderOctetIgnoredOnReceive). Send clause now proven: TestRFC5340ReservedHeaderOctetZeroOnSend reads offset 15 of v6Encoder Hello, DBDesc and LSAck with Instance ID 0 and 0xFF, and a Header.WriteTo over a 0xFF-filled buffer, all 0. Revert records on encoder_v6.go::EncodeHello observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L29) | unit/verify | revert, verified |
| negative | [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L377) | unit/verify | unproven |
| positive | [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L373) | unit/verify | unproven |

### [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1)

Forwarding address A fully qualified IPv6 address (128 bits). Included in the LSA if and only if bit F has been set. If included, data traffic for the advertised destination will be forwarded to this address. It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10). (§A.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): translateNSSAV6 changed only at its election call (RFC 3101 3.1 reachability, decided flag); the v6UsableForwardingAddress gate on translated forwarding addresses is unchanged and the tagged units are byte-identical; the fixture declares every router reached. Prior judgement: Local origination: v6UsableForwardingAddress rejects fe80::1, ::, ::1, ff02::5 and v6OriginateNSSALSA drops an all-zero address. Translation: translateNSSAV6 (nssa.go) now refuses to translate a received NSSA-LSA whose forwarding address fails v6UsableForwardingAddress. TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding runs translateNSSA on an elected v6 NSSA ABR: a global forwarding address is carried into the Type-5 unchanged, and fe80::2 produces no Type-5. Re-read 2026-09-27 (DF-OSPF-B): translateNSSAV6 changed only in its RFC 3101 Section 3.2 yield test (equivalentType5V6); the A.4.7 forwarding-address gate is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300) | unit/verify | unproven |
| positive | [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L298) | unit/verify | unproven |

### [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2)

While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address. (§A.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): translateNSSAV6 changed only at its election call; the global-forwarding gate is unchanged. Prior judgement: Origination selects only a global address (interfaceIPv6ForwardingAddress via v6UsableForwardingAddress). The translator now advertises only a forwarding address that passes the same rule: TestRFC5340TranslatorAdvertisesGlobalForwarding requires the translated Type-5 forwarding address to be global unicast, and requires no Type-5 for fe80::3. Re-read 2026-09-27 (DF-OSPF-B): translateNSSAV6 changed only in its RFC 3101 Section 3.2 yield test (equivalentType5V6); the A.4.7 forwarding-address gate is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L74) | unit/verify | revert, verified |
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L310) | unit/verify | unproven |
| positive | [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L306) | unit/verify | unproven |

### [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1)

A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers. (§A.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, ospf c13). Appendix A.4.8: 'A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers.' The c12 gap is closed: NEW TestRFC5340NSSAForwardingAddressFromKernelInterface (linux, own user+net namespace) calls the production interfaceIPv6ForwardingAddress over a real dummy link: - a link with only fe80::1 and lo with only ::1 yield no address; + after adding 2001:db8:9::1 the lookup returns exactly that address. Judge overlay removing the v6UsableForwardingAddress filter from interfaceIPv6ForwardingAddress reds the negative ('ospffa0 with only fe80::1 yielded forwarding address fe80::1'); base green. The unit also caught a defect the author fixed: the lookup never loaded the iface backend (siblings at interface_addr.go:29/46 do), so it answered no address everywhere and a negative would have passed vacuously; EnsureBackend added. Selection across interfaces stays proven by the seam unit TestRFC5340NSSAForwardingAddressSelectsGlobal; propagation gating by TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress (mutant records on v6OriginateNSSALSA). Residual: the kernel unit Skips where a dummy link cannot be created.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L345) | unit/verify | mutant, verified |
| positive | [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L342) | unit/verify | mutant, verified |

### [`RFC5340-C.3-1`](#rfc5340-c.3-1)

The interface output cost MUST always be greater than 0 (§C.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L433) | unit/verify | unproven |
| positive | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L430) | unit/verify | unproven |

### [`RFC5340-C.3-2`](#rfc5340-c.3-2)

InfTransDelay The estimated number of seconds it takes to transmit a Link State Update packet over this interface. LSAs contained in the update packet must have their age incremented by this amount before transmission. This value should take into account the transmission and propagation delays of the interface. It MUST be greater than 0. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both obligations proven. 'MUST be greater than 0': TestRFC5340IPv6InterfaceCostAndTransmitDelay accepts transmit-delay 1 and refuses 0 with ErrTransmitDelayZero. 'must have their age incremented by this amount before transmission': TestRFC5340FloodIncrementsAgeByInfTransDelay drives the shared lsdb flood path through v6Encoder and decodes the OSPFv3 LS Updates from the wire with ospfv3packet.DecodePacket: a Link-LSA originated at age 0 leaves with 7 (InfTransDelay 7) and its retransmission 5 s later with 12 (held 5 + 7 once), exact values that fail if either send skips the increment; an InfTransDelay of 65535 leaves at MaxAge, never past (the RFC 2328 13.3 (5) cap RFC 5340 keeps). Revert records on lsdb/flooding.go::floodCopy (+/-) observed red. The 'should take into account' sentence is advice on the configured value, not a checkable obligation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L439) | unit/verify | unproven |
| positive | [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L436) | unit/verify | unproven |

### [`RFC5340-C.3-3`](#rfc5340-c.3-3)

HelloInterval The length of time, in seconds, between Hello packets that the router sends on the interface. This value is advertised in the router's Hello packets. It MUST be the same for all routers attached to a common link. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a neighbor whose HelloInterval differs on the common link. TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink asserts ReceiveDecodedHello accepts a matching HelloInterval (empty reason) and drops DefaultHelloInterval+1 with reason hello-interval; accepting the mismatch turns it red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L475) | unit/verify | unproven |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L472) | unit/verify | unproven |

### [`RFC5340-C.3-4`](#rfc5340-c.3-4)

RouterDeadInterval After ceasing to hear a router's Hello packets, the number of seconds before its neighbors declare the router down. This is also advertised in the router's Hello packets in their RouterDeadInterval field. This should be some multiple of the HelloInterval (e.g., 4). This value again MUST be the same for all routers attached to a common link. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a neighbor whose RouterDeadInterval differs on the common link. TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink asserts a matching RouterDeadInterval is accepted (empty reason) and DefaultDeadInterval+1 is dropped with reason dead-interval; accepting the mismatch turns it red. The 'multiple of the HelloInterval' clause is a lower-case should.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L480) | unit/verify | unproven |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L478) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5340.txt |
| Source fingerprint | 6bec6b95b48aaca3 |
| Record | rfc/extraction/rfc5340.json |
| Mapped sentences | 22 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 2 | walked | not stated |
| `2.6` | not stated | 0 | walked | not stated |
| `2.7` | not stated | 0 | walked | not stated |
| `2.8` | not stated | 2 | walked | not stated |
| `2.9` | not stated | 0 | walked | not stated |
| `2.10` | not stated | 0 | walked | not stated |
| `2.11` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 1 | walked | not stated |
| `4.1.3` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.1.1` | not stated | 2 | walked | not stated |
| `4.2.1.2` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 4 | walked | not stated |
| `4.2.2.1` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 0 | walked | not stated |
| `4.4.2` | not stated | 0 | walked | not stated |
| `4.4.3` | not stated | 0 | walked | not stated |
| `4.4.3.1` | not stated | 0 | walked | not stated |
| `4.4.3.2` | not stated | 0 | walked | not stated |
| `4.4.3.3` | not stated | 0 | walked | not stated |
| `4.4.3.4` | not stated | 1 | walked | not stated |
| `4.4.3.5` | not stated | 0 | walked | not stated |
| `4.4.3.6` | not stated | 0 | walked | not stated |
| `4.4.3.7` | not stated | 0 | walked | not stated |
| `4.4.3.8` | not stated | 0 | walked | not stated |
| `4.4.3.9` | not stated | 0 | walked | not stated |
| `4.4.4` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.5.1` | not stated | 0 | walked | not stated |
| `4.5.2` | not stated | 0 | walked | not stated |
| `4.5.3` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 1 | walked | not stated |
| `4.8` | not stated | 1 | walked | not stated |
| `4.8.1` | not stated | 2 | walked | not stated |
| `4.8.2` | not stated | 0 | walked | not stated |
| `4.8.3` | not stated | 0 | walked | not stated |
| `4.8.4` | not stated | 0 | walked | not stated |
| `4.8.5` | not stated | 0 | walked | not stated |
| `4.9` | not stated | 2 | walked | not stated |
| `4.9.1` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |
| `A.3.1` | not stated | 1 | walked | not stated |
| `A.3.2` | not stated | 0 | walked | not stated |
| `A.3.3` | not stated | 0 | walked | not stated |
| `A.3.4` | not stated | 0 | walked | not stated |
| `A.3.5` | not stated | 0 | walked | not stated |
| `A.3.6` | not stated | 0 | walked | not stated |
| `A.4` | not stated | 0 | walked | not stated |
| `A.4.1` | not stated | 0 | walked | not stated |
| `A.4.1.1` | not stated | 0 | walked | not stated |
| `A.4.2` | not stated | 0 | walked | not stated |
| `A.4.2.1` | not stated | 0 | walked | not stated |
| `A.4.3` | not stated | 0 | walked | not stated |
| `A.4.4` | not stated | 0 | walked | not stated |
| `A.4.5` | not stated | 0 | walked | not stated |
| `A.4.6` | not stated | 0 | walked | not stated |
| `A.4.7` | not stated | 2 | walked | not stated |
| `A.4.8` | not stated | 1 | walked | not stated |
| `A.4.9` | not stated | 0 | walked | not stated |
| `A.4.10` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |
| `C.1` | not stated | 0 | walked | not stated |
| `C.2` | not stated | 0 | walked | not stated |
| `C.3` | not stated | 4 | walked | not stated |
| `C.4` | not stated | 0 | walked | not stated |
| `C.5` | not stated | 0 | walked | not stated |
| `C.6` | not stated | 0 | walked | not stated |
| `C.7` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.4.3.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the link-local advertisement ban restated for inter-area-prefix-LSAs, which the row already names as its Section 4.4.3.4 restatement | o The NU-bit in the PrefixOptions field should be clear. o Link-local addresses MUST never be advertised in inter-area- prefix-LSAs. |
| `4.7:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the virtual link global-scope address rule restated in the virtual links section, which the row already names | o The IPv6 interface address of a virtual link MUST be an IPv6 address having global scope, instead of the link-local addresses used by other interface types. |
| `4.8:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the aggregate router-LSA rule restated for the shortest-path calculation, which the row already names as its Section 4.8 reaffirmation | These router-LSAs MUST be treated as a single aggregate by the area's shortest-path calculation (see Section 4.8.1). |
| `4.8.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the aggregate router-LSA rule restated in the SPF step for vertex V, which the row already names as its Section 4.8.1 reaffirmation | All router-LSAs with the Advertising Router set to V's OSPF Router ID MUST be processed as an aggregate, treating them as fragments of a single large router-LSA. |
| `4.8.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same aggregate router-LSA rule stated again for vertex W in the same SPF step | All router-LSAs with the Advertising Router set to W's OSPF Router ID MUST be processed as an aggregate, treating them as fragments of a single large router- LSA. |

## Superseded

No document obsoletes RFC 5340, so its obligations are stated where they were written.
