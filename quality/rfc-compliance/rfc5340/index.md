# RFC 5340 - OSPF for IPv6

OSPFv3 (partial, experimental). Every requirement listed for RFC 5340, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16 | of 23 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 23 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 23 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 23 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 20 | 20 of 45 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 23 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 23 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 23 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 1 | of 23 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 23 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 23 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 23 | of 46 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 23 mandatory requirements: they add up to 23. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPFv3 (partial, experimental) |
| Automatic checks | Included |
| Requirements | 46 |
| Mandatory requirements (MUST level) | 23 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 45 |
| Tests linked to requirements | 45 |
| Recorded audit verdicts | 13 |
| Fault test records | 20 |
| Summary | `rfc/short/rfc5340.md` |
| Requirement table | `rfc/requirements/rfc5340.md` |
| RFC text | `rfc/full/rfc5340.txt` |

## Automatic checks

Included in automatic checks: OSPF for IPv6 (OSPFv3)

## Published support status

**Area:** OSPFv3 (partial, experimental)

**Test verification:** 6 of 23 MUST proven

**Evidence:** unit 6

**Unmet:** RFC5340-2.5-2, RFC5340-2.8-2, RFC5340-4.2.2-1, RFC5340-4.9-1, RFC5340-4.9-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 7 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **23** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (16):** [`RFC5340-2.5-1`](#rfc5340-2.5-1), [`RFC5340-2.8-3`](#rfc5340-2.8-3), [`RFC5340-2.8-4`](#rfc5340-2.8-4), [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2), [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1), [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2), [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1), [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5), [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2), [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1), [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2), [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1), [`RFC5340-C.3-1`](#rfc5340-c.3-1), [`RFC5340-C.3-2`](#rfc5340-c.3-2), [`RFC5340-C.3-3`](#rfc5340-c.3-3), [`RFC5340-C.3-4`](#rfc5340-c.3-4)

**Annotated (including scoped evidence) (7):** [`RFC5340-2.5-2`](#rfc5340-2.5-2), [`RFC5340-2.8-2`](#rfc5340-2.8-2), [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1), [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2), [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3), [`RFC5340-4.9-1`](#rfc5340-4.9-1), [`RFC5340-4.9-2`](#rfc5340-4.9-2)

## Figures

- **MUST bar**: Ze verified 6, Tests awaiting verification 11, OS testing gap 1, Not built 5
- **Overview bar**: Ze MUST verified 6, Tests awaiting verification 11, OS testing gap 1, Not assessed 23, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 23 |
| MUST rows kept (the MUST bar) | 23 |
| Ze verified | 6 |
| OS verified | 0 |
| Tests awaiting verification | 11 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 23 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 23 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5340.json](../../../data/rfc/rfc5340.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5340-2.5-1` | On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L521). **negative:** `unit/verify` [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L557) |
| `RFC5340-2.5-2` | In particular, link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs (Section 4.4.3.4), AS-external-LSAs (Section 4.4.3.6), NSSA-LSAs (Section 4.4.3.7), or intra-area-prefix- LSAs (Section 4.4.3.9). (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the ban holds for intra-area-prefix-LSAs on both origination paths (interfaceIPv6Prefixes, origination_v6.go:564; v6HostPrefixes, origination_v6.go:432; v6AggregatedLinkPrefixes, origination_v6_link.go:153), but the ABR summary path copies every prefix out of a received intra-area-prefix-LSA into an inter-area-prefix-LSA with no link-local filter (v6SummaryNetworks, origination_v6_summary.go:136-147), and the ASBR path wire-encodes a redistributed prefix with no link-local filter either (v6InjectExternal -> netipToV6Prefix, origination_v6_external.go:55 and origination_v6.go:592), so a link-local supplied by a peer or by redistribution reaches an inter-area-prefix-LSA / AS-external-LSA / NSSA-LSA. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-2` | Receivers MUST concatenate all the router-LSAs originated by a given router when running the SPF calculation. (§2.8) | MUST | 2.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the OSPFv3 graph build keys a router vertex by Advertising Router alone and ASSIGNS rather than concatenates, so a second Router-LSA from the same router (a different Link State ID) replaces the first instead of aggregating its links (v6Strategy.BuildGraph, afstrategy_v6.go:113-117). Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-2.8-3` | a network-LSA MUST list all routers connected to the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L189). **negative:** `unit/verify` [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L192) |
| `RFC5340-2.8-4` | a link-LSA MUST list all of a router's addresses on the link (§2.8) | MUST | 2.8 | **positive:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L227). **negative:** `unit/verify` [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L231) |
| `RFC5340-4.1.2-2` | For IPv6, the IPv6 address appearing in the source of OSPF packets sent on the interface is almost always a link-local address. The one exception is for virtual links that MUST use one of the router's own global IPv6 addresses as IP interface address. (§4.1.2) | MUST | 4.1.2 | **positive:** `unit/verify` [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L526). **negative:** `unit/verify` [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) |
| `RFC5340-4.2.1.1-1` | Before the Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74). **negative:** `unit/verify` [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) |
| `RFC5340-4.2.1.1-2` | Those that MUST be set correctly in Hello packets are as follows. The E-bit is set if and only if the interface attaches to a regular area, i.e., not a stub or NSSA area. Similarly, the N-bit is set if and only if the interface attaches to an NSSA area (see [NSSA]). Finally, the DC-bit is set if and only if the router wishes to suppress the sending of future Hellos over the interface (see [DEMAND]). (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119). **negative:** `unit/verify` [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) |
| `RFC5340-4.2.1.2-1` | Those that MUST be set correctly in Database Description packets are as follows. The DC-bit is set if and only if the router wishes to suppress the sending of Hellos over the interface (see [DEMAND]). (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154). **negative:** `unit/verify` [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L160) |
| `RFC5340-4.2.2-1` | In order for the packet to be passed to OSPF for processing, the following tests must be performed on the encapsulating IPv6 headers: o The packet's IP destination address MUST be one of the IPv6 unicast addresses associated with the receiving interface (this includes link-local addresses), one of the IPv6 multicast addresses AllSPFRouters or AllDRouters, or an IPv6 global address (for virtual links). (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no destination-address acceptance check exists. The v3 backend records the datagram destination from the IPV6_PKTINFO control message (backend_linux.go:307) and the dispatcher consumes it ONLY as pseudo-header input to the checksum (dispatcher.go:65), never comparing it against the interface's addresses or ff02::5 / ff02::6; grep for `.Dst` over internal/plugins/ospf finds no other reader. A protocol-89 datagram sent to a group the host already joins (for example ff02::1) is therefore accepted, since its sender computed a checksum for that same destination. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.2.2-2` | The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13). **negative:** no negative test. **{single-polarity}:** the transport opens its raw socket on "ip6:89" (listenNetwork, v3/transport/backend_linux.go:28), so the kernel stamps Next Header 89 on every send and demultiplexes only Next Header 89 to this socket on receive. ze never sees a non-89 datagram, so it has no reject path of its own to exercise |
| `RFC5340-4.2.2-3` | o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM AH and ESP input (net/ipv6/ah6.c, net/ipv6/esp6.c), linux:CONFIG_INET6_AH with CONFIG_INET6_ESP; internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex) and buildIPsecSA the manual-key state, so Linux XFRM processes and verifies the AH or ESP header before the datagram reaches ze's socket; the VPP IPsec backend refuses the direction-less multicast SA (vppUnsupportedSA, internal/component/ike/dataplane/vpp.go), so no VPP path carries OSPFv3 IPsec. Owed tag: a test that reads the installed policy and state back. Ze only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go) |
| `RFC5340-4.2.2-5` | The version number field MUST specify protocol version 3 (§4.2.2) | MUST | 4.2.2 | **positive:** `unit/verify` [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L31). **negative:** `unit/verify` [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L80) |
| `RFC5340-4.9-1` | o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-4.9-2` | If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9) | MUST | 4.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| `RFC5340-A.3.1-2` | These fields are reserved. They SHOULD be set to 0 when sending protocol packets and MUST be ignored when receiving protocol packets. (§A.3.1) | MUST | A.3.1 | **positive:** `unit/verify` [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L373). **positive:** `unit/verify` [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L25). **negative:** `unit/verify` [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L29). **negative:** `unit/verify` [`TestRFC5340ReservedOctetNeverReachesDecodedHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L173) |
| `RFC5340-A.4.7-1` | Forwarding address A fully qualified IPv6 address (128 bits). Included in the LSA if and only if bit F has been set. If included, data traffic for the advertised destination will be forwarded to this address. It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10). (§A.4.7) | MUST NOT | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L298). **positive:** `unit/verify` [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L45). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300). **negative:** `unit/verify` [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L56) |
| `RFC5340-A.4.7-2` | While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address. (§A.4.7) | MUST | A.4.7 | **positive:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L306). **positive:** `unit/verify` [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L67). **negative:** `unit/verify` [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L310). **negative:** `unit/verify` [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L74) |
| `RFC5340-A.4.8-1` | A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers. (§A.4.8) | MUST | A.4.8 | **positive:** `unit/verify` [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L78). **positive:** `unit/verify` [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L62). **positive:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L342). **negative:** `unit/verify` [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L68). **negative:** `unit/verify` [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L50). **negative:** `unit/verify` [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L345) |
| `RFC5340-C.3-1` | The interface output cost MUST always be greater than 0 (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L430). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L433) |
| `RFC5340-C.3-2` | InfTransDelay The estimated number of seconds it takes to transmit a Link State Update packet over this interface. LSAs contained in the update packet must have their age incremented by this amount before transmission. This value should take into account the transmission and propagation delays of the interface. It MUST be greater than 0. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L66). **positive:** `unit/verify` [`TestRFC5340IPv6InterfaceTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L483). **negative:** `unit/verify` [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L73). **negative:** `unit/verify` [`TestRFC5340IPv6InterfaceTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L487) |
| `RFC5340-C.3-3` | HelloInterval The length of time, in seconds, between Hello packets that the router sends on the interface. This value is advertised in the router's Hello packets. It MUST be the same for all routers attached to a common link. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L493). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L496) |
| `RFC5340-C.3-4` | RouterDeadInterval After ceasing to hear a router's Hello packets, the number of seconds before its neighbors declare the router down. This is also advertised in the router's Hello packets in their RouterDeadInterval field. This should be some multiple of the HelloInterval (e.g., 4). This value again MUST be the same for all routers attached to a common link. (§C.3) | MUST | C.3 | **positive:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L499). **negative:** `unit/verify` [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L501) |
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
| [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3) o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM AH and ESP input (net/ipv6/ah6.c, net/ipv6/esp6.c), linux:CONFIG_INET6_AH with CONFIG_INET6_ESP; internal/plugins/ospf/ipsec_install.go::buildIPsecPolicies installs the inbound require-policy (SADirIn, transport mode, upper-protocol 89, scoped to the arrival ifindex) and buildIPsecSA the manual-key state, so Linux XFRM processes and verifies the AH or ESP header before the datagram reaches ze's socket; the VPP IPsec backend refuses the direction-less multicast SA (vppUnsupportedSA, internal/component/ike/dataplane/vpp.go), so no VPP path carries OSPFv3 IPsec. Owed tag: a test that reads the installed policy and state back. Ze only samples the resulting drop counters (readXfrmDropsPlatform, ipsec_drops_linux.go) |
| [`RFC5340-4.9-1`](#rfc5340-4.9-1) o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9) | {gap}, no test | ze has no notion of several interfaces sharing one link. Each configured interface is enrolled independently, keyed by its own name and OS ifindex (openInterface, instance.go:679-691; the Interface ID is interfaceIndex, interface_addr.go:107-123), and two interfaces on the same physical link with the same Instance ID form two separate adjacencies rather than one Active/Standby pair. Disclosed in docs/features/rfc-status.md RFC 5340 row |
| [`RFC5340-4.9-2`](#rfc5340-4.9-2) If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9) | {gap}, no test | there is no Active/Standby interface model to flush from -- grep for "Standby" over internal/plugins/ospf finds no producer. A link-local scope Link-LSA is flushed only with its own interface's link store (v6OriginateLinkLSA / OriginateLinkSelf, origination_v6_link.go:58), never re-flushed onto a sibling interface. Disclosed in docs/features/rfc-status.md RFC 5340 row |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5340-2.5-1`](#rfc5340-2.5-1)

On virtual links, a global scope IPv6 address MUST be used as the source address for OSPF protocol packets (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestV6VirtualEndpointRequiresGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L557) | unit/verify | not verified |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L521) | unit/verify | not verified |

### [`RFC5340-2.5-2`](#rfc5340-2.5-2)

In particular, link-local addresses MUST NOT be advertised in inter-area-prefix-LSAs (Section 4.4.3.4), AS-external-LSAs (Section 4.4.3.6), NSSA-LSAs (Section 4.4.3.7), or intra-area-prefix- LSAs (Section 4.4.3.9). (§2.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-2.5-2, so there is no linked test.

### [`RFC5340-2.8-2`](#rfc5340-2.8-2)

Receivers MUST concatenate all the router-LSAs originated by a given router when running the SPF calculation. (§2.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-2.8-2, so there is no linked test.

### [`RFC5340-2.8-3`](#rfc5340-2.8-3)

a network-LSA MUST list all routers connected to the link (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Network-LSA that omits a router attached to the link. TestRFC5340NetworkLSAListsAttachedRouters drives v6OriginateNetwork, decodes the installed Network-LSA and asserts got[self] (the DR) and got[full] (a Full neighbor), each red on omission; got[twoWay] false and Len 2 bound the list to the routers RFC 2328 12.4.2 counts as attached (fully adjacent), which RFC 5340 leaves unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L192) | unit/verify | not verified |
| positive | [`TestRFC5340NetworkLSAListsAttachedRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L189) | unit/verify | not verified |

### [`RFC5340-2.8-4`](#rfc5340-2.8-4)

a link-LSA MUST list all of a router's addresses on the link (§2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Link-LSA that omits one of the router's addresses on the link. TestRFC5340LinkLSAListsLinkAddresses decodes the originated Link-LSA and asserts the link-local fe80::1 and both global prefixes 2001:db8:1::/64 and 2001:db8:2::/64, each red on omission; the eth1 half asserts each link's LSA carries its own link-local and prefix and not the other link's.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L231) | unit/verify | not verified |
| positive | [`TestRFC5340LinkLSAListsLinkAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L227) | unit/verify | not verified |

### [`RFC5340-4.1.2-2`](#rfc5340-4.1.2-2)

For IPv6, the IPv6 address appearing in the source of OSPF packets sent on the interface is almost always a link-local address. The one exception is for virtual links that MUST use one of the router's own global IPv6 addresses as IP interface address. (§4.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a virtual link using anything but one of this router's own global IPv6 addresses as its IP interface address. TestV6VirtualEndpointResolvesGlobalAddress asserts v6ResolveVirtualEndpointLocked returns this router's global 2001:db8:1::1 as source (red on another address); TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress asserts that with only fe80::1 advertised the endpoint does not resolve (red if a link-local is adopted). The 'almost always a link-local' sentence is descriptive and states no obligation.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_vlink_test.go#L14) | unit/verify | not verified |
| positive | [`TestV6VirtualEndpointResolvesGlobalAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/virtual_link_test.go#L526) | unit/verify | not verified |

### [`RFC5340-4.2.1.1-1`](#rfc5340-4.2.1.1-1)

Before the Hello packet is sent on an interface, the interface's Interface ID MUST be copied into the Hello packet. (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Hello sent without this interface's own Interface ID. TestRFC5340HelloCarriesInterfaceID sends real Hellos through SendHello and the v6 encoder and asserts Interface ID 42 on eth0 and 43 on eth1 (red on a missing or shared value), and after a neighbor Hello carrying 99 asserts the next sent Hello still carries 42 (red if the neighbor's value is copied in).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L80) | unit/verify | not verified |
| positive | [`TestRFC5340HelloCarriesInterfaceID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L74) | unit/verify | not verified |

### [`RFC5340-4.2.1.1-2`](#rfc5340-4.2.1.1-2)

Those that MUST be set correctly in Hello packets are as follows. The E-bit is set if and only if the interface attaches to a regular area, i.e., not a stub or NSSA area. Similarly, the N-bit is set if and only if the interface attaches to an NSSA area (see [NSSA]). Finally, the DC-bit is set if and only if the router wishes to suppress the sending of future Hellos over the interface (see [DEMAND]). (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Hello whose E, N or DC bit does not follow the area type or the wish to suppress Hellos. TestRFC5340HelloOptionsBits sends Hellos through the real send path and asserts E set and N clear for a regular area, N set and E clear for NSSA, both clear for stub (both directions of each 'if and only if'), and DC zero in all three; ze has no demand-circuit support and never wishes to suppress Hellos, so DC clear is the only correct value.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L123) | unit/verify | not verified |
| positive | [`TestRFC5340HelloOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L119) | unit/verify | not verified |

### [`RFC5340-4.2.1.2-1`](#rfc5340-4.2.1.2-1)

Those that MUST be set correctly in Database Description packets are as follows. The DC-bit is set if and only if the router wishes to suppress the sending of Hellos over the interface (see [DEMAND]). (§4.2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (OSPF judge, continuations 3+4): TestRFC5340DBDescOptionsBits tags now claim only the DC-bit: DDs of a regular and a stub area carry DC clear (Ze suppresses no Hellos, so 'if and only if' requires clear), and neutral Options carrying DC handed to the encoder still yield DC clear. Revert records on encoder_v6.go::neutralToV6Options observed red. The 'set when suppressing' direction has no Ze path (no demand circuits).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC5340DBDescOptionsBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L154) | unit/verify | revert, verified |

### [`RFC5340-4.2.2-1`](#rfc5340-4.2.2-1)

In order for the packet to be passed to OSPF for processing, the following tests must be performed on the encapsulating IPv6 headers: o The packet's IP destination address MUST be one of the IPv6 unicast addresses associated with the receiving interface (this includes link-local addresses), one of the IPv6 multicast addresses AllSPFRouters or AllDRouters, or an IPv6 global address (for virtual links). (§4.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-4.2.2-1, so there is no linked test.

### [`RFC5340-4.2.2-2`](#rfc5340-4.2.2-2)

The Next Header field of the immediately encapsulating IPv6 header MUST specify the OSPF protocol (89) (§4.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5340TransportUsesOSPFProtocolNumber`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/transport/rfc5340_linux_test.go#L13) | unit/verify | not verified |

### [`RFC5340-4.2.2-3`](#rfc5340-4.2.2-3)

o Any encapsulating IP Authentication Headers (see [IPAUTH]) and the IP Encapsulating Security Payloads (see [IPESP]) MUST be processed and/or verified to ensure integrity and authentication/ confidentiality of OSPF routing exchanges. (§4.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-4.2.2-3, so there is no linked test.

### [`RFC5340-4.2.2-5`](#rfc5340-4.2.2-5)

The version number field MUST specify protocol version 3 (§4.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOSPFv3DecodeHeaderBounds`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L80) | unit/verify | not verified |
| positive | [`TestOSPFv3HeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L31) | unit/verify | not verified |

### [`RFC5340-4.9-1`](#rfc5340-4.9-1)

o Each of the multiple interfaces MUST be configured with the same Interface Instance ID to be considered on the same link. (§4.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-4.9-1, so there is no linked test.

### [`RFC5340-4.9-2`](#rfc5340-4.9-2)

If a Standby Interface goes down, then the link-local scope LSAs originated for the Standby Interfaces MUST be flushed on the Active Interface. (§4.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5340-4.9-2, so there is no linked test.

### [`RFC5340-A.3.1-2`](#rfc5340-a.3.1-2)

These fields are reserved. They SHOULD be set to 0 when sending protocol packets and MUST be ignored when receiving protocol packets. (§A.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 after a comment-only spelling change (honoured to honored) in TestRFC5340ReservedHeaderOctetIgnoredOnReceive; its assertions are byte-identical. Receive: a Hello with Reserved 0xFF forms the neighbor (assert.Len 1), record on v3 DecodeHeader. Negative TestRFC5340ReservedOctetNeverReachesDecodedHeader: headers differing only in octet 15 decode identically. Send: TestRFC5340ReservedHeaderOctetZeroOnSend, record on EncodeHello. Earlier enforced judgement stands.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L29) | unit/verify | revert, verified |
| negative | [`TestRFC5340ReservedOctetNeverReachesDecodedHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/v3/packet/rfc5340_header_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC5340ReservedHeaderOctetZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_reserved_send_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC5340ReservedHeaderOctetIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L373) | unit/verify | revert, verified |

### [`RFC5340-A.4.7-1`](#rfc5340-a.4.7-1)

Forwarding address A fully qualified IPv6 address (128 bits). Included in the LSA if and only if bit F has been set. If included, data traffic for the advertised destination will be forwarded to this address. It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10). (§A.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-19 (independent auditor, after 6af25a7a79). Appendix A.4.7: "It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10)." and "While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address." 6af25a7a79 touched only rfc/discrimination/rfc5340.json: the translator negative's record moves from a panic of v6UsableForwardingAddress (reach only) to the mutant at interface_addr.go:73 dropping '!addr.IsLinkLocalUnicast() &&'. Line 73 at HEAD is that return statement, and with the link-local term gone fe80::2/fe80::3 pass every remaining term (Is6, not 4in6, not loopback, unspecified or multicast), so translatableForwardingAddressV6 admits it and the negative's 'no Type-5' assertion is the one that fails, as the author reports (red at :60/:77, not re-observed here). This closes the prior judge's note that the record proved reach, not the branch. Producers, tests and claims are byte-identical to the last judgement; tagged units pass at HEAD (go test -count=1). For -1: TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding asserts a global address is carried unchanged and fe80::2 yields no Type-5; the Unspecified half is pinned by TestRFC5340ForwardingAddressIsGlobal (:: refused, all-zero origination clears HasForwardingAddr), and translateNSSAV6 skips an all-zero forwarding address before the gate.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L56) | unit/verify | mutant, verified |
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L300) | unit/verify | not verified |
| positive | [`TestRFC5340TranslatorNeverAdvertisesLinkLocalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L298) | unit/verify | not verified |

### [`RFC5340-A.4.7-2`](#rfc5340-a.4.7-2)

While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address. (§A.4.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-19 (independent auditor, after 6af25a7a79). Appendix A.4.7: "It MUST NOT be set to the IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address (Prefix FE80/10)." and "While OSPFv3 routes are normally installed with link-local addresses, an OSPFv3 implementation advertising a forwarding address MUST advertise a global IPv6 address." 6af25a7a79 touched only rfc/discrimination/rfc5340.json: the translator negative's record moves from a panic of v6UsableForwardingAddress (reach only) to the mutant at interface_addr.go:73 dropping '!addr.IsLinkLocalUnicast() &&'. Line 73 at HEAD is that return statement, and with the link-local term gone fe80::2/fe80::3 pass every remaining term (Is6, not 4in6, not loopback, unspecified or multicast), so translatableForwardingAddressV6 admits it and the negative's 'no Type-5' assertion is the one that fails, as the author reports (red at :60/:77, not re-observed here). This closes the prior judge's note that the record proved reach, not the branch. Producers, tests and claims are byte-identical to the last judgement; tagged units pass at HEAD (go test -count=1). For -2: TestRFC5340TranslatorAdvertisesGlobalForwarding asserts the translated forwarding address IsGlobalUnicast and that fe80::3 yields no Type-5; claims state no more than these assertions.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L74) | unit/verify | mutant, verified |
| negative | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L310) | unit/verify | not verified |
| positive | [`TestRFC5340TranslatorAdvertisesGlobalForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_translate_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC5340ForwardingAddressIsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L306) | unit/verify | not verified |

### [`RFC5340-A.4.8-1`](#rfc5340-a.4.8-1)

A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers. (§A.4.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, ospf c13). Appendix A.4.8: 'A global IPv6 address MUST be selected as forwarding address for NSSA-LSAs that are to be propagated by NSSA area border routers.' The c12 gap is closed: NEW TestRFC5340NSSAForwardingAddressFromKernelInterface (linux, own user+net namespace) calls the production interfaceIPv6ForwardingAddress over a real dummy link: - a link with only fe80::1 and lo with only ::1 yield no address; + after adding 2001:db8:9::1 the lookup returns exactly that address. Judge overlay removing the v6UsableForwardingAddress filter from interfaceIPv6ForwardingAddress reds the negative ('ospffa0 with only fe80::1 yielded forwarding address fe80::1'); base green. The unit also caught a defect the author fixed: the lookup never loaded the iface backend (siblings at interface_addr.go:29/46 do), so it answered no address everywhere and a negative would have passed vacuously; EnsureBackend added. Selection across interfaces stays proven by the seam unit TestRFC5340NSSAForwardingAddressSelectsGlobal; propagation gating by TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress (mutant records on v6OriginateNSSALSA). Residual: the kernel unit Skips where a dummy link cannot be created.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L345) | unit/verify | mutant, verified |
| positive | [`TestRFC5340NSSAForwardingAddressFromKernelInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_iface_linux_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC5340NSSAForwardingAddressSelectsGlobal`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_nssa_fa_select_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC5340NSSAPropagationNeedsGlobalForwardingAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L342) | unit/verify | mutant, verified |

### [`RFC5340-C.3-1`](#rfc5340-c.3-1)

The interface output cost MUST always be greater than 0 (§C.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L433) | unit/verify | not verified |
| positive | [`TestRFC5340IPv6InterfaceCostAndTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L430) | unit/verify | not verified |

### [`RFC5340-C.3-2`](#rfc5340-c.3-2)

InfTransDelay The estimated number of seconds it takes to transmit a Link State Update packet over this interface. LSAs contained in the update packet must have their age incremented by this amount before transmission. This value should take into account the transmission and propagation delays of the interface. It MUST be greater than 0. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. The InfTransDelay check is split from the cost check: TestRFC5340IPv6InterfaceTransmitDelay accepts 1 and rejects 0 with ErrTransmitDelayZero, and its first validateConfig call is now the transmit-delay case, so the validateConfigAF revert record hits this clause. Age increment by InfTransDelay before transmission: TestRFC5340FloodIncrementsAgeByInfTransDelay (records floodCopy).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestRFC5340IPv6InterfaceTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L487) | unit/verify | revert, verified |
| positive | [`TestRFC5340FloodIncrementsAgeByInfTransDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_flood_age_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5340IPv6InterfaceTransmitDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L483) | unit/verify | revert, verified |

### [`RFC5340-C.3-3`](#rfc5340-c.3-3)

HelloInterval The length of time, in seconds, between Hello packets that the router sends on the interface. This value is advertised in the router's Hello packets. It MUST be the same for all routers attached to a common link. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a neighbor whose HelloInterval differs on the common link. TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink asserts ReceiveDecodedHello accepts a matching HelloInterval (empty reason) and drops DefaultHelloInterval+1 with reason hello-interval; accepting the mismatch turns it red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L496) | unit/verify | not verified |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L493) | unit/verify | not verified |

### [`RFC5340-C.3-4`](#rfc5340-c.3-4)

RouterDeadInterval After ceasing to hear a router's Hello packets, the number of seconds before its neighbors declare the router down. This is also advertised in the router's Hello packets in their RouterDeadInterval field. This should be some multiple of the HelloInterval (e.g., 4). This value again MUST be the same for all routers attached to a common link. (§C.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a neighbor whose RouterDeadInterval differs on the common link. TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink asserts a matching RouterDeadInterval is accepted (empty reason) and DefaultDeadInterval+1 is dropped with reason dead-interval; accepting the mismatch turns it red. The 'multiple of the HelloInterval' clause is a lower-case should.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L501) | unit/verify | not verified |
| positive | [`TestRFC5340HelloAndDeadIntervalMustMatchOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5340_test.go#L499) | unit/verify | not verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `2.4` | not stated | 0 | reviewed | not stated |
| `2.5` | not stated | 2 | reviewed | not stated |
| `2.6` | not stated | 0 | reviewed | not stated |
| `2.7` | not stated | 0 | reviewed | not stated |
| `2.8` | not stated | 2 | reviewed | not stated |
| `2.9` | not stated | 0 | reviewed | not stated |
| `2.10` | not stated | 0 | reviewed | not stated |
| `2.11` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 0 | reviewed | not stated |
| `3.7` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.1.1` | not stated | 0 | reviewed | not stated |
| `4.1.2` | not stated | 1 | reviewed | not stated |
| `4.1.3` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.2.1` | not stated | 0 | reviewed | not stated |
| `4.2.1.1` | not stated | 2 | reviewed | not stated |
| `4.2.1.2` | not stated | 1 | reviewed | not stated |
| `4.2.2` | not stated | 4 | reviewed | not stated |
| `4.2.2.1` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `4.3.1` | not stated | 0 | reviewed | not stated |
| `4.4` | not stated | 0 | reviewed | not stated |
| `4.4.1` | not stated | 0 | reviewed | not stated |
| `4.4.2` | not stated | 0 | reviewed | not stated |
| `4.4.3` | not stated | 0 | reviewed | not stated |
| `4.4.3.1` | not stated | 0 | reviewed | not stated |
| `4.4.3.2` | not stated | 0 | reviewed | not stated |
| `4.4.3.3` | not stated | 0 | reviewed | not stated |
| `4.4.3.4` | not stated | 1 | reviewed | not stated |
| `4.4.3.5` | not stated | 0 | reviewed | not stated |
| `4.4.3.6` | not stated | 0 | reviewed | not stated |
| `4.4.3.7` | not stated | 0 | reviewed | not stated |
| `4.4.3.8` | not stated | 0 | reviewed | not stated |
| `4.4.3.9` | not stated | 0 | reviewed | not stated |
| `4.4.4` | not stated | 0 | reviewed | not stated |
| `4.5` | not stated | 0 | reviewed | not stated |
| `4.5.1` | not stated | 0 | reviewed | not stated |
| `4.5.2` | not stated | 0 | reviewed | not stated |
| `4.5.3` | not stated | 0 | reviewed | not stated |
| `4.6` | not stated | 0 | reviewed | not stated |
| `4.7` | not stated | 1 | reviewed | not stated |
| `4.8` | not stated | 1 | reviewed | not stated |
| `4.8.1` | not stated | 2 | reviewed | not stated |
| `4.8.2` | not stated | 0 | reviewed | not stated |
| `4.8.3` | not stated | 0 | reviewed | not stated |
| `4.8.4` | not stated | 0 | reviewed | not stated |
| `4.8.5` | not stated | 0 | reviewed | not stated |
| `4.9` | not stated | 2 | reviewed | not stated |
| `4.9.1` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |
| `A.1` | not stated | 0 | reviewed | not stated |
| `A.2` | not stated | 0 | reviewed | not stated |
| `A.3` | not stated | 0 | reviewed | not stated |
| `A.3.1` | not stated | 1 | reviewed | not stated |
| `A.3.2` | not stated | 0 | reviewed | not stated |
| `A.3.3` | not stated | 0 | reviewed | not stated |
| `A.3.4` | not stated | 0 | reviewed | not stated |
| `A.3.5` | not stated | 0 | reviewed | not stated |
| `A.3.6` | not stated | 0 | reviewed | not stated |
| `A.4` | not stated | 0 | reviewed | not stated |
| `A.4.1` | not stated | 0 | reviewed | not stated |
| `A.4.1.1` | not stated | 0 | reviewed | not stated |
| `A.4.2` | not stated | 0 | reviewed | not stated |
| `A.4.2.1` | not stated | 0 | reviewed | not stated |
| `A.4.3` | not stated | 0 | reviewed | not stated |
| `A.4.4` | not stated | 0 | reviewed | not stated |
| `A.4.5` | not stated | 0 | reviewed | not stated |
| `A.4.6` | not stated | 0 | reviewed | not stated |
| `A.4.7` | not stated | 2 | reviewed | not stated |
| `A.4.8` | not stated | 1 | reviewed | not stated |
| `A.4.9` | not stated | 0 | reviewed | not stated |
| `A.4.10` | not stated | 0 | reviewed | not stated |
| `B` | not stated | 0 | reviewed | not stated |
| `C` | not stated | 0 | reviewed | not stated |
| `C.1` | not stated | 0 | reviewed | not stated |
| `C.2` | not stated | 0 | reviewed | not stated |
| `C.3` | not stated | 4 | reviewed | not stated |
| `C.4` | not stated | 0 | reviewed | not stated |
| `C.5` | not stated | 0 | reviewed | not stated |
| `C.6` | not stated | 0 | reviewed | not stated |
| `C.7` | not stated | 0 | reviewed | not stated |

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
