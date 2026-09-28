# RFC 5798 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

Partial. Every requirement this repository extracted from RFC 5798, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.9% | 39 of 55 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.3% | 4 of 55 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 55 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 98.9% | 87 of 88 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 55 | of 80 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 55 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 55 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 5.5% | 3 of 55 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 3.6% | 2 of 55 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.7% | 7 of 55 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 43 | of 55 gated MUSTs judged | 21 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 55 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 80 |
| Gated MUST-level | 55 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 7 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 88 |
| Tagged units | 88 |
| Recorded audit verdicts | 43 |
| Discrimination records | 87 |
| Summary | `rfc/short/rfc5798.md` |
| Requirement shard | `rfc/requirements/rfc5798.md` |
| RFC text | `rfc/full/rfc5798.txt` |

## Enrolment

Enrolled: VRRP Version 3 for IPv4 and IPv6 (RFC 5798, obsoleted by RFC 9568): the VRRPv3 document ze actually speaks on the IPv4 wire. Every one of its 55 gated requirements carries a {superseded} marker naming where the obligation lives in RFC 9568, and 51 of them are restated there unchanged, so ze implements them through the same producers rfc/short/rfc9568.md gates: internal/plugins/vrrp/packet (advert encode/decode, checksum), internal/plugins/vrrp/fsm (Initialize/Backup/Master), internal/plugins/vrrp/transport (proto 112, GTSM, GARP/NA). The row that is NOT a restatement is RFC5798-5.2.8-1: Section 5.2.8 puts a pseudo-header under the checksum for both families, RFC 9568 Section 5.2.8 removes it for IPv4, and ze transmits this document form because keepalived and the pre-RFC-9568 base require it (pseudoSumV4Legacy and FillChecksum, internal/plugins/vrrp/packet/checksum.go). Tagged under RFC5798 ids: 39 of the 55 gated MUSTs carry a positive and a negative test, 4 carry a positive test and a `{single-polarity}` annotation because the producer writes a constant no input changes, 3 are met below ze by the Linux forwarding and SLAAC paths, 2 are conditional on an optional feature ze declined, and 7 are `{gap}` rows ze owes.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- For IPv4, ze transmits the RFC 5798 pseudo-header checksum form, because that is what keepalived (proven on the wire: its own adverts use it) and the rest of the deployed base compute and require
- a message-only advert is rejected by them as "Invalid VRRPv3 checksum". On receive, ze dual-accepts both this form and the RFC 9568 message-only form.


**What the ledger says remains**

RFC 9568 Section 5.2.8 clarifies the IPv4 checksum as message-only (no pseudo-header); ze diverges from that clarification on transmit for interoperability, and counts message-only senders (`checksum-rfc9568-message-only`) so the strict-RFC-9568 population is visible. When that population dominates, the transmit form can be revisited.

- **Enrolled 2026-09-01:** [`rfc/short/rfc5798.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5798.md) declares 80 requirements, 55 of them MUST-level, and every one carries a `{superseded}` marker naming where RFC 9568 states it. 39 of those 55 are proven under RFC5798 ids in both polarities. 4 carry a positive test and a `{single-polarity}` annotation, because the producer writes a constant no input changes: [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1), [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1), [`RFC5798-7.2-2`](#rfc5798-7.2-2) and [`RFC5798-7.2-4`](#rfc5798-7.2-4). [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1) is the weakest of those four, because its only evidence is an integration-gated test the privileged QEMU suite runs and `./le rfc discriminate-record` cannot reach. 3 are met below ze and carry a `{lower-layer}` annotation: [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) and [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1), where Linux consumes a link-local scope multicast datagram rather than forwarding it, and [`RFC5798-7.4-1`](#rfc5798-7.4-1), where Linux derives each device's Interface Identifier. 2 are conditional on an optional feature ze declined and carry a `{feature-declined}` annotation: [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1), the VRRPv2/VRRPv3 dual-send flag RFC 5798 Section 8.4.2 makes a MAY, and RFC5798-A.2-1, the Token Ring functional-address mode. 7 are `{gap}` rows ze owes: [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) and [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) (no ND Router Advertisement is sent for a virtual router, so there is no option set to configure either), [`RFC5798-7.4-2`](#rfc5798-7.4-2) (the virtual-MAC macvlan carries no `addr_gen_mode`, so Linux derives its link-local Interface Identifier from the virtual router MAC), [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) (a proxy ARP reply on a VRRP router does not carry the virtual router MAC), [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) and [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) (ze authors no Neighbor Solicitation for a host and installs nothing that sources one from the virtual MAC), and [`RFC5798-7.1-3`](#rfc5798-7.1-3), where Section 7.1 makes the receiver discard an advertisement when the local router is the IPvX address owner and `Decode` ([`internal/plugins/vrrp/packet/validate.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate.go)) verifies only the VRID half, because RFC 9568 erratum 8298 lowers the address-owner half to a log. One RFC 5798 obligation is NOT a restatement: [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) puts a pseudo-header under the checksum for both address families, which is the divergence this row describes. RFC 5798 Section 5.2.8 cites the pseudo-header "as defined in Section 8.1 of [RFC2460]", an IPv6-only shape, so what ze and the deployed base compute for IPv4 is the classic IPv4 pseudo-header (`pseudoSumV4Legacy`, [`internal/plugins/vrrp/packet/checksum.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum.go)) rather than the shape that sentence names.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 39 | one part of the gated population |
| Annotated instead of tested | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **55** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (39):** [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2), [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2), [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1), [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1), [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2), [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1), [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1), [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1), [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2), [`RFC5798-6.1-1`](#rfc5798-6.1-1), [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1), [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2), [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3), [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4), [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5), [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6), [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7), [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8), [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9), [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10), [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1), [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2), [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3), [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5), [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6), [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7), [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8), [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9), [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10), [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11), [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12), [`RFC5798-7.1-1`](#rfc5798-7.1-1), [`RFC5798-7.1-2`](#rfc5798-7.1-2), [`RFC5798-7.1-4`](#rfc5798-7.1-4), [`RFC5798-7.2-1`](#rfc5798-7.2-1), [`RFC5798-7.2-3`](#rfc5798-7.2-3), [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1), [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1), [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4)

**Annotated instead of tested (16):** [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1), [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1), [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1), [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1), [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4), [`RFC5798-7.1-3`](#rfc5798-7.1-3), [`RFC5798-7.2-2`](#rfc5798-7.2-2), [`RFC5798-7.2-4`](#rfc5798-7.2-4), [`RFC5798-7.4-1`](#rfc5798-7.4-1), [`RFC5798-7.4-2`](#rfc5798-7.4-2), [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1), [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2), [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3), [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1), [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1), [`RFC5798-A.2-1`](#rfc5798-a.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5798-5.1.1.2-1` | The IPv4 multicast address as assigned by the IANA for VRRP is: 224.0.0.18 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | MUST NOT | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux forwarding plane; internal/plugins/fib/kernel/backend_linux.go::addRoute writes every route ze puts into the kernel FIB and it writes unicast next hops only, ze enables no multicast routing anywhere, and Linux consumes a datagram addressed to the 224.0.0.0/24 link-local scope group on its own input path instead of forwarding it, so no state ze installs decides this |
| `RFC5798-5.1.1.3-1` | The TTL MUST be set to 255. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L315). **negative:** no negative test. **{single-polarity}:** buildIPv4Header internal/plugins/vrrp/transport/transport.go writes the literal 255 into the TTL octet of every advertisement header it builds, so no configuration reaches another value; discarding a received TTL that is not 255 is the separate RFC5798-5.1.1.3-2 |
| `RFC5798-5.1.1.3-2` | A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.1.1.3, §7.1) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L535) |
| `RFC5798-5.1.2.2-1` | The IPv6 multicast address assigned by the IANA for VRRP is: FF02:0:0:0:0:0:0:12 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | MUST NOT | 5.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux forwarding plane; internal/plugins/fib/kernel/backend_linux.go::addRoute writes every route ze puts into the kernel FIB, ze enables no IPv6 multicast routing anywhere, and Linux consumes a datagram addressed to the ff02::/16 link-local scope group on its own input path instead of forwarding it, so no state ze installs decides this |
| `RFC5798-5.1.2.3-1` | The Hop Limit MUST be set to 255. (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L242). **negative:** no negative test. **{single-polarity}:** the hop limit is a socket option, IPV6_MULTICAST_HOPS 255 set once in openV6 internal/plugins/vrrp/transport/backend_linux.go, so every advertisement leaving that socket carries 255 and no configuration reaches another value; discarding a received hop limit that is not 255 is the separate RFC5798-5.1.2.3-2. The one positive test is internal/plugins/vrrp/transport/transport_integration_linux_test.go, gated //go:build integration && linux and skipped without CAP_NET_RAW and CAP_NET_ADMIN, so it runs under the privileged QEMU suite and on an unprivileged host this requirement is proven by nothing |
| `RFC5798-5.1.2.3-2` | A VRRP router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet. (§5.1.2.3, §7.1) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L104). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L665) |
| `RFC5798-5.2.2-1` | The only packet type defined in this version of the protocol is: 1 ADVERTISEMENT A packet with unknown type MUST be discarded. (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L152) |
| `RFC5798-5.2.4-1` | The priority value for the VRRP router that owns the IPvX address associated with the virtual router MUST be 255 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L774). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L776) |
| `RFC5798-5.2.4-2` | VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) |
| `RFC5798-5.2.6-1` | This field MUST be set to zero on transmission and ignored on reception. (§5.2.6) | MUST | 5.2.6 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107). **negative:** `unit/verify` [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L639) |
| `RFC5798-5.2.8-1` | The checksum is the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]. The next header field in the "pseudo-header" should be set to 112 (decimal) for VRRP. For computing the checksum, the checksum field is set to zero. (§5.2.8, §7.1) | MUST | 5.2.8 | **positive:** `unit/verify` [`TestFillChecksumFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L68). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L663) |
| `RFC5798-5.2.9-1` | For IPv6, the first address must be the IPv6 link-local address associated with the virtual router. (§5.2.9, §6.1; lowercase "must" in the RFC) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L524). **negative:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L526) |
| `RFC5798-5.2.9-2` | This field contains either one or more IPv4 addresses, or one or more IPv6 addresses, that is, IPv4 and IPv6 MUST NOT both be carried in one IPvX Address field. (§5.2.9) | MUST NOT | 5.2.9 | **positive:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L548). **negative:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L550) |
| `RFC5798-6.1-1` | Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L110). **negative:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L112) |
| `RFC5798-6.4.2-1` | (310) + MUST NOT respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L386). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L431) |
| `RFC5798-6.4.2-2` | (320) + MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L712). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L714) |
| `RFC5798-6.4.2-3` | (325) + MUST NOT send ND Router Advertisement messages for the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1107). **negative:** `unit/verify` [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1108) |
| `RFC5798-6.4.2-4` | (335) - MUST discard packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L388). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L433) |
| `RFC5798-6.4.2-5` | (340) - MUST NOT accept packets addressed to the IPvX address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L390). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L435) |
| `RFC5798-6.4.2-6` | If a Shutdown event is received, then: (350) + Cancel the Master_Down_Timer (355) + Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) |
| `RFC5798-6.4.2-7` | If the Master_Down_Timer fires, then: (370) + Send an ADVERTISEMENT (375) + If the protected IPvX address is an IPv4 address, then: (380) * Broadcast a gratuitous ARP request on that interface containing the virtual router MAC address for each IPv4 address associated with the virtual router. (385) + else // ipv6 (390) * Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the virtual router. (395) * For each IPv6 address associated with the virtual router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) unset, the Override flag (O) set, the target address set to the IPv6 address of the virtual router, and the target link-layer address set to the virtual router MAC address. (400) +endif // was protected addr ipv4? (405) + Set the Adver_Timer to Advertisement_Interval (410) + Transition to the {Master} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L448). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L639) |
| `RFC5798-6.4.2-8` | If the Priority in the ADVERTISEMENT is zero, then: (430) * Set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) |
| `RFC5798-6.4.2-9` | If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: (450) @ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (455) @ Recompute the Master_Down_Interval (460) @ Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120) |
| `RFC5798-6.4.2-10` | else // preempt was true or priority was less (470) @ Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124) |
| `RFC5798-6.4.3-1` | (610) + MUST respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L72). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L139) |
| `RFC5798-6.4.3-2` | (620) + MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L716). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L720) |
| `RFC5798-6.4.3-3` | (625) + MUST respond to ND Neighbor Solicitation message for the IPv6 address(es) associated with the virtual router. (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L721). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L722) |
| `RFC5798-6.4.3-4` | (630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs IPv6 virtual routers and sends no Router Advertisement for them. The announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136, built by BuildNA internal/plugins/vrrp/transport/na.go and selected by frameBuilder internal/plugins/vrrp/transport/transport.go, and no Router Advertisement builder and no RA send path exists under internal/plugins/vrrp |
| `RFC5798-6.4.3-5` | (645) - MUST forward packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L428). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L392) |
| `RFC5798-6.4.3-6` | (650) - MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L242). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L206) |
| `RFC5798-6.4.3-7` | MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, MUST NOT accept these packets. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L204). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L244) |
| `RFC5798-6.4.3-8` | If a Shutdown event is received, then: (660) + Cancel the Adver_Timer (665) + Send an ADVERTISEMENT with Priority = 0 (670) + Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L126). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L128) |
| `RFC5798-6.4.3-9` | If the Adver_Timer fires, then: (685) + Send an ADVERTISEMENT (690) + Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L130). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L641) |
| `RFC5798-6.4.3-10` | If the Priority in the ADVERTISEMENT is zero, then: (710) -* Send an ADVERTISEMENT (715) -* Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L132). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L134) |
| `RFC5798-6.4.3-11` | If the Priority in the ADVERTISEMENT is greater than the local Priority, (730) -* or (735) -* If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX Address of the sender is greater than the local primary IPvX Address, then: (740) -@ Cancel Adver_Timer (745) -@ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (750) -@ Recompute the Skew_Time (755) @ Recompute the Master_Down_Interval (760) @ Set Master_Down_Timer to Master_Down_Interval (765) @ Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L136). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L78). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L138). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L79) |
| `RFC5798-6.4.3-12` | else // new Master logic (775) @ Discard ADVERTISEMENT (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L140). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L142) |
| `RFC5798-7.1-1` | - MUST verify that the VRRP version is 3. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L611) |
| `RFC5798-7.1-2` | - MUST verify that the received packet contains the complete VRRP packet (including fixed fields, and IPvX address). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L83). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L511) |
| `RFC5798-7.1-3` | - MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Decode internal/plugins/vrrp/packet/validate.go verifies the VRID half of this check and ze implements no address-owner half, so an advertisement arriving for a VRID whose address the local router owns is processed rather than discarded. RFC 9568 erratum 8298 lowers that half to a SHOULD that logs and ze follows the successor, so the RFC 5798 MUST as published is unmet |
| `RFC5798-7.1-4` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L547). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L527) |
| `RFC5798-7.2-1` | The following operations MUST be performed when transmitting a VRRP packet: - Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109). **negative:** `unit/verify` [`TestBoundaryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L290) |
| `RFC5798-7.2-2` | If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L388). **negative:** no negative test. **{single-polarity}:** the source link-layer address is the virtual router MAC that packet.VirtualMAC internal/plugins/vrrp/packet/packet.go derives from the family and the VRID, put on the wire by binding the tx socket to the macvlan created with that address (openV4 and openV6 internal/plugins/vrrp/transport/backend_linux.go), a derivation no input can make yield another MAC |
| `RFC5798-7.2-3` | If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address + Set the source IPv6 address to interface link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218). **negative:** `unit/verify` [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L408). **negative:** `unit/verify` [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L360) |
| `RFC5798-7.2-4` | Set the IPvX protocol to VRRP - Send the VRRP packet to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L317). **negative:** no negative test. **{single-polarity}:** buildIPv4Header internal/plugins/vrrp/transport/transport.go writes the constant packet.ProtoNumber into the protocol octet and SendAdvert internal/plugins/vrrp/transport/backend_linux.go targets the constants packet.MulticastV4 and packet.MulticastV6, so no configuration reaches another protocol number or another destination |
| `RFC5798-7.4-1` | IPv6 routers running VRRP MUST create their Interface Identifiers in the normal manner (e.g., "Transmission of IPv6 Packets over Ethernet Networks" [RFC2464]). (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux SLAAC; internal/plugins/vrrp/dataplane_linux.go::applyDataplaneSysctls is the only IPv6 state ze writes on a VRRP device and it writes accept_dad alone, while Linux derives each device's Interface Identifier from that device's MAC in the RFC 2464 manner this sentence names, so no value ze writes decides the derivation |
| `RFC5798-7.4-2` | They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers. (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze creates the per-group macvlan carrying the virtual router MAC and writes no addr_gen_mode on it, because applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go writes accept_dad and nothing else on an IPv6 group, so Linux derives that device's link-local Interface Identifier from the virtual router MAC, which is the derivation this sentence forbids. Suppressing it, by writing addr_gen_mode on the IPv6 macvlan, is unbuilt |
| `RFC5798-8.1.2-1` | The Virtual Router Master MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2) | MUST NOT | 8.1.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L74). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L44). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L141) |
| `RFC5798-8.1.3-1` | If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC) | MUST | 8.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze offers proxy ARP, as the builtin proxy sysctl profile that writes net.ipv4.conf.<iface>.proxy_arp=1 internal/core/sysctl/profiles.go, and nothing makes a proxy ARP reply on a VRRP router carry the virtual router MAC: the kernel answers from the replying device's own address and ze installs no virtual-MAC proxy state, so the obligation is unbuilt rather than out of reach |
| `RFC5798-8.2.2-1` | The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L718). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L45). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L719). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L46) |
| `RFC5798-8.2.2-2` | When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze authors no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go -- and it installs nothing that makes a host solicitation source from the virtual-MAC macvlan, which doInstallVIPs internal/plugins/vrrp/instance.go gives the virtual address alone. The solicitation a Master sends for a host therefore leaves the parent, carrying the parent's own link-layer address |
| `RFC5798-8.2.2-3` | It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same unbuilt path as RFC5798-8.2.2-2. Ze authors no Neighbor Solicitation and installs nothing that keeps the physical MAC out of the source link-layer address option of one its stack sends for a host |
| `RFC5798-8.2.2-4` | o At system boot, when initializing interfaces for VRRP operation, all ND Router and Neighbor Advertisements and Solicitation messages must be delayed until both the IPv6 address and the virtual router MAC address are configured. (§8.2.2; lowercase "must" in the RFC) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L765). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L767) |
| `RFC5798-8.2.3-1` | The Backup routers must be configured to send the same Router Advertisement options as the address owner. (§8.2.3; lowercase "must" in the RFC) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation rests on the Router Advertisement ze does not send, which is the gap at RFC5798-6.4.3-4. The vrrp group carries no Router Advertisement leaf at all -- applyGroupLeaves internal/plugins/vrrp/groups.go accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only -- so there is no option set a Backup could be configured to match |
| `RFC5798-8.4.2-1` | An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. When a virtual router is configured this way and is the Master, it MUST send both types at the configured rate, even if sub-second. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements"; ze offers no such flag, so no virtual router is ever configured that way and the condition this MUST rests on is never true. internal/plugins/vrrp/groups.go::parseVersion admits 2 or 3 and a group runs exactly that one version, which internal/plugins/vrrp/instance.go::doSendAdvert encodes |
| `RFC5798-A.2-1` | The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. (§A.2) | MUST | A.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring"; ze supports VRRP over Ethernet only and is never a router supporting VRRP on Token Ring, so the condition this MUST names is never true. internal/plugins/vrrp/packet/packet.go::VirtualMAC derives the Ethernet 00-00-5E-00-01 and 00-00-5E-00-02 address from the VRID and carries no Token-Ring functional-address mapping |
| `RFC5798-7.1-5` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-8` | MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.1-1` | The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision. (§8.1.1; lowercase "should" in the RFC) | SHOULD | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-2` | When a VRRP router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for the IPv4 address it owns; it should only send ARP messages that include virtual MAC addresses. (§8.1.2) | SHOULD NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-3` | When configuring an interface, Virtual Router Master routers should broadcast a gratuitous ARP request containing the virtual router MAC address for each IPv4 address on that interface. (§8.1.2; lowercase "should" in the RFC) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-4` | At system boot, when initializing interfaces for VRRP operation, delay gratuitous ARP requests and ARP responses until both the IPv4 address and the virtual router MAC address are configured. (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-5` | When, for example, ssh access to a particular VRRP router is required, an IP address known to belong to that router must be used. (§8.1.2; lowercase "must" inside a SHOULD-level bullet list) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.1-1` | The IPv6 source address of an ICMPv6 redirect should be the address that the end-host used when making its next-hop routing decision. (§8.2.1; lowercase "should" in the RFC) | SHOULD | 8.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-5` | When a VRRP router restarts or boots, it SHOULD NOT send any ND messages with its physical MAC address for the IPv6 address it owns; it should only send ND messages that include virtual MAC addresses. (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-6` | When configuring an interface, Virtual Router Master routers should send an unsolicited ND Neighbor Advertisement message containing the virtual router MAC address for the IPv6 address on that interface. (§8.2.2; lowercase "should" in the RFC) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.3-2` | Router Advertisement options that advertise special services (e.g., Home Agent Information Option) that are present in the address owner should not be sent by the address owner unless the Backup routers are prepared to assume these services in full and have a complete and synchronized database for this service. (§8.2.3; lowercase "should not" in the RFC) | SHOULD NOT | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.1-1` | If it is not the address owner, a VRRP router SHOULD NOT forward packets addressed to the IPvX address for which it becomes Master. (§8.3.1) | SHOULD NOT | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.2-1` | A priority value of 255 designates a particular router as the "IPvX address owner". Care must be taken not to configure more than one router on the link in this way for a single VRID. (§8.3.2; lowercase in the RFC) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.2-2` | When there are multiple Backup routers, their priority values should be uniformly distributed. For example, if one Backup router has the default priority of 100 and another Backup Router is added, a priority of 50 would be a better choice for it than 99 or 100, in order to facilitate faster convergence. (§8.3.2; lowercase in the RFC) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-2` | As mentioned above, this support is intended for upgrade scenarios and is NOT recommended for permanent deployments. (§8.4.2) | NOT RECOMMENDED | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-3` | When a virtual router is configured this way and is the Backup, it should time out based on the rate advertised by the Master; in the case of a VRRPv2 Master, this means it must translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-4` | Also, a Backup should ignore VRRPv2 advertisements from the current Master if it is also receiving VRRPv3 packets from it. (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.3.1-1` | A VRRPv2 implementation should not be given a higher priority than a VRRPv2/VRRPv3 implementation it is interacting with if the VRRPv2/ VRRPv3 rate is sub-second. (§8.4.3.1) | SHOULD NOT | 8.4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-A.1-1` | To avoid this, an implementation SHOULD configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device, rather than changing its hardware MAC address. (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-6` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event, and MAY indicate via network management that an error occurred. - MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-7` | MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-7` | Note that on a restarting Master router where the VRRP protected address is the interface address, (that is, priority 255) duplicate address detection (DAD) may fail, as the Backup router may answer that it owns the address. One solution is to not run DAD in this case. (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-5` | An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-6` | It MAY report when a VRRPv3 Master is *not* sending VRRPv2 packets: that suggests they don't agree on whether they're supporting VRRPv2 routers. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-A.2-2` | The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. Additionally, routers MAY support the unicast mode of operation to take advantage of newer Token-Ring adapter implementations that support non-promiscuous reception for multiple unicast MAC addresses and to avoid both the multicast traffic and usage conflicts associated with the use of Token-Ring functional addresses. (§A.2) | MAY | A.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) The IPv4 multicast address as assigned by the IANA for VRRP is: 224.0.0.18 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux forwarding plane; internal/plugins/fib/kernel/backend_linux.go::addRoute writes every route ze puts into the kernel FIB and it writes unicast next hops only, ze enables no multicast routing anywhere, and Linux consumes a datagram addressed to the 224.0.0.0/24 link-local scope group on its own input path instead of forwarding it, so no state ze installs decides this |
| [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1) The IPv6 multicast address assigned by the IANA for VRRP is: FF02:0:0:0:0:0:0:12 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux forwarding plane; internal/plugins/fib/kernel/backend_linux.go::addRoute writes every route ze puts into the kernel FIB, ze enables no IPv6 multicast routing anywhere, and Linux consumes a datagram addressed to the ff02::/16 link-local scope group on its own input path instead of forwarding it, so no state ze installs decides this |
| [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) (630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3) | {gap}, no test | ze runs IPv6 virtual routers and sends no Router Advertisement for them. The announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136, built by BuildNA internal/plugins/vrrp/transport/na.go and selected by frameBuilder internal/plugins/vrrp/transport/transport.go, and no Router Advertisement builder and no RA send path exists under internal/plugins/vrrp |
| [`RFC5798-7.1-3`](#rfc5798-7.1-3) - MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1) | {gap}, no test | Decode internal/plugins/vrrp/packet/validate.go verifies the VRID half of this check and ze implements no address-owner half, so an advertisement arriving for a VRID whose address the local router owns is processed rather than discarded. RFC 9568 erratum 8298 lowers that half to a SHOULD that logs and ze follows the successor, so the RFC 5798 MUST as published is unmet |
| [`RFC5798-7.4-1`](#rfc5798-7.4-1) IPv6 routers running VRRP MUST create their Interface Identifiers in the normal manner (e.g., "Transmission of IPv6 Packets over Ethernet Networks" [RFC2464]). (§7.4) | no test | no test carries this requirement id; annotated {lower-layer}: Linux SLAAC; internal/plugins/vrrp/dataplane_linux.go::applyDataplaneSysctls is the only IPv6 state ze writes on a VRRP device and it writes accept_dad alone, while Linux derives each device's Interface Identifier from that device's MAC in the RFC 2464 manner this sentence names, so no value ze writes decides the derivation |
| [`RFC5798-7.4-2`](#rfc5798-7.4-2) They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers. (§7.4) | {gap}, no test | ze creates the per-group macvlan carrying the virtual router MAC and writes no addr_gen_mode on it, because applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go writes accept_dad and nothing else on an IPv6 group, so Linux derives that device's link-local Interface Identifier from the virtual router MAC, which is the derivation this sentence forbids. Suppressing it, by writing addr_gen_mode on the IPv6 macvlan, is unbuilt |
| [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC) | {gap}, no test | ze offers proxy ARP, as the builtin proxy sysctl profile that writes net.ipv4.conf.<iface>.proxy_arp=1 internal/core/sysctl/profiles.go, and nothing makes a proxy ARP reply on a VRRP router carry the virtual router MAC: the kernel answers from the replying device's own address and ze installs no virtual-MAC proxy state, so the obligation is unbuilt rather than out of reach |
| [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2) | {gap}, no test | ze authors no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go -- and it installs nothing that makes a host solicitation source from the virtual-MAC macvlan, which doInstallVIPs internal/plugins/vrrp/instance.go gives the virtual address alone. The solicitation a Master sends for a host therefore leaves the parent, carrying the parent's own link-layer address |
| [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | {gap}, no test | the same unbuilt path as RFC5798-8.2.2-2. Ze authors no Neighbor Solicitation and installs nothing that keeps the physical MAC out of the source link-layer address option of one its stack sends for a host |
| [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) The Backup routers must be configured to send the same Router Advertisement options as the address owner. (§8.2.3; lowercase "must" in the RFC) | {gap}, no test | the obligation rests on the Router Advertisement ze does not send, which is the gap at RFC5798-6.4.3-4. The vrrp group carries no Router Advertisement leaf at all -- applyGroupLeaves internal/plugins/vrrp/groups.go accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only -- so there is no option set a Backup could be configured to match |
| [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1) An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. When a virtual router is configured this way and is the Master, it MUST send both types at the configured rate, even if sub-second. (§8.4.2) | no test | no test carries this requirement id; annotated {feature-declined}: "An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements"; ze offers no such flag, so no virtual router is ever configured that way and the condition this MUST rests on is never true. internal/plugins/vrrp/groups.go::parseVersion admits 2 or 3 and a group runs exactly that one version, which internal/plugins/vrrp/instance.go::doSendAdvert encodes |
| [`RFC5798-A.2-1`](#rfc5798-a.2-1) The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. (§A.2) | no test | no test carries this requirement id; annotated {feature-declined}: "The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring"; ze supports VRRP over Ethernet only and is never a router supporting VRRP on Token Ring, so the condition this MUST names is never true. internal/plugins/vrrp/packet/packet.go::VirtualMAC derives the Ethernet 00-00-5E-00-01 and 00-00-5E-00-02 address from the VRID and carries no Token-Ring functional-address mapping |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1)

The IPv4 multicast address as assigned by the IANA for VRRP is: 224.0.0.18 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.1.2-1, so no unit is bound to it.

### [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1)

The TTL MUST be set to 255. (§5.1.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity by annotation: TestSendAdvertV3IPv4HeaderTTLProtoDst asserts frame[8]==255 on the IPv4 header buildIPv4Header writes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L315) | unit/verify | revert, verified |

### [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2)

A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.1.1.3, §7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. golden decode with TTL 255 passes; N4 with TTL 64 asserts ErrTTL, isolated from every other ladder row

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L535) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75) | unit/verify | revert, verified |

### [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1)

The IPv6 multicast address assigned by the IANA for VRRP is: FF02:0:0:0:0:0:0:12 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.2.2-1, so no unit is bound to it.

### [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1)

The Hop Limit MUST be set to 255. (§5.1.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity by annotation: integration test reads IPV6_MULTICAST_HOPS 255 back from the tx socket; runs only under the privileged QEMU suite

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L242) | unit/verify | unproven |

### [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2)

A VRRP router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet. (§5.1.2.3, §7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. golden v6 decode passes at hop limit 255; hop limit 64 asserts ErrTTL

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L665) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L104) | unit/verify | revert, verified |

### [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1)

The only packet type defined in this version of the protocol is: 1 ADVERTISEMENT A packet with unknown type MUST be discarded. (§5.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. type 1 golden accepted; type 2 asserts ErrType, which precedes the VRID lookup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77) | unit/verify | revert, verified |

### [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1)

The priority value for the VRRP router that owns the IPvX address associated with the virtual router MUST be 255 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. owner EffectivePriority(0)==255 from configured 120; non-owner keeps 100

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L776) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L774) | unit/verify | revert, verified |

### [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2)

VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestBoundaryPriority proves only the config range 1..254; the run-time priority is EffectivePriority(decrement), whose clamp at 1 (groups.go EffectivePriority) is exercised by the untagged TestEffectivePriorityWithTracking, so removing the clamp leaves the tagged units green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) | unit/verify | revert, verified |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319) | unit/verify | revert, verified |

### [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1)

This field MUST be set to zero on transmission and ignored on reception. (§5.2.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. negative is sound (all-ones nibble ignored on decode); positive encodes into a freshly zeroed buffer with a 100 cs interval, so a WriteTo that left the nibble untouched or OR-ed into it would still pass

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L639) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107) | unit/verify | revert, verified |

### [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1)

The checksum is the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]. The next header field in the "pseudo-header" should be set to 112 (decimal) for VRRP. For computing the checksum, the checksum field is set to zero. (§5.2.8, §7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: (a) ones-complement sum over the whole message, (b) the pseudo-header for both families with (c) next header 112, (d) the checksum field zero during computation. TestFillChecksumFamilies pins (a)(b)(c) through the v3v4 golden 0xDEFB and the v3v6 golden, and TestDecodeV3IPv6ChecksumAndHopLimit asserts ErrChecksum on a corrupted v6 checksum. Clause (d) is not discriminated: the test sets buf[6], buf[7] = 0, 0 before calling FillChecksum, so deleting the zeroing at checksum.go FillChecksum (msg[6] = 0; msg[7] = 0) stays green. No tagged IPv4 negative either (N1b sits in TestNegativeReferenceBugs, untagged for this id).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L663) | unit/verify | revert, verified |
| positive | [`TestFillChecksumFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L68) | unit/verify | revert, verified |

### [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1)

For IPv6, the first address must be the IPv6 link-local address associated with the virtual router. (§5.2.9, §6.1; lowercase "must" in the RFC)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a transmitted IPv6 advertisement whose first IPvX address is not the link-local one. TestValidateIPv6LinkLocal asserts only that validateGroups rejects a config listing a global address first and accepts link-local first. No tagged assertion reads the address order of an encoded or transmitted advert, so a tx path that reordered or sorted the VIPs (netip order puts 2001:db8:: ahead of fe80::) would stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L526) | unit/verify | revert, verified |
| positive | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L524) | unit/verify | revert, verified |

### [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2)

This field contains either one or more IPv4 addresses, or one or more IPv6 addresses, that is, IPv4 and IPv6 MUST NOT both be carried in one IPvX Address field. (§5.2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. same-family addresses accepted; the other family on either group is rejected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L550) | unit/verify | revert, verified |
| positive | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L548) | unit/verify | revert, verified |

### [`RFC5798-6.1-1`](#rfc5798-6.1-1)

Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ICMPv6 135 and 136 accepts must precede the first drop of the accept-filter chain; the test fails if either is missing or placed after a drop

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L110) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1)

(310) + MUST NOT respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup installs no VIP (parent carries arp_ignore=1, so nothing answers ARP for it); Master contrast installs it on the vMAC device

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L431) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L386) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2)

(320) + MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup installs no IPv6 VIP, Master installs it on the vMAC macvlan only; NS answering follows the kernel address

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L714) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L712) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3)

(325) + MUST NOT send ND Router Advertisement messages for the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup emits zero announcements; the Master's one announcement is BuildNA type 136, never 134

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1108) | unit/verify | revert, verified |
| positive | [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1107) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4)

(335) - MUST discard packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. both units assert only that a Backup installs no VIP and a Master does. That decides local delivery, not what happens to a frame whose destination MAC is the virtual router MAC: the vMAC macvlan exists in Backup too, and whether the kernel discards or forwards such a frame is observed by no tagged unit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L433) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L388) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5)

(340) - MUST NOT accept packets addressed to the IPvX address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup installs no VIP so no packet to it is delivered locally; Master contrast installs it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L435) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L390) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6)

If a Shutdown event is received, then: (350) + Cancel the Master_Down_Timer (355) + Transition to the {Initialize} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row backup/shutdown asserts exactly StopTimers + EmitStateChange to Initialize; a non-Shutdown event leaves Backup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7)

If the Master_Down_Timer fires, then: (370) + Send an ADVERTISEMENT (375) + If the protected IPvX address is an IPv4 address, then: (380) * Broadcast a gratuitous ARP request on that interface containing the virtual router MAC address for each IPv4 address associated with the virtual router. (385) + else // ipv6 (390) * Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the virtual router. (395) * For each IPv6 address associated with the virtual router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) unset, the Override flag (O) set, the target address set to the IPv6 address of the virtual router, and the target link-layer address set to the virtual router MAC address. (400) +endif // was protected addr ipv4? (405) + Set the Adver_Timer to Advertisement_Interval (410) + Transition to the {Master} state (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestFSMMasterDownPromotion asserts the action TYPE order and the Master state only: it does not check the StartAdvertTimer interval equals Advertisement_Interval, nor the gratuitous ARP carrying the virtual router MAC, nor the NA flags R set, S unset, O set; negative (stale generation) is sound

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L639) | unit/verify | revert, verified |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L448) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8)

If the Priority in the ADVERTISEMENT is zero, then: (430) * Set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Priority 0 advert arms StartMasterDownTimer(skewDur); non-zero arms mdDur

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9)

If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: (450) @ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (455) @ Recompute the Master_Down_Interval (460) @ Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: adopt when (a) Preempt_Mode is False, or (b) advertised priority >= local, then set Master_Adver_Interval, recompute Master_Down_Interval, reset the timer. TestFSMTransitionMatrix covers (a) with backup/advert/adopt/preempt-false-lower-priority and the equality half of (b) with backup/advert/adopt/equal-priority, each asserting StartMasterDownTimer(mdDur(cfg,4000)); the discard row is the negative. No Backup row receives an advert with priority GREATER than local under Preempt True, so a condition written == instead of >= stays green. mdDur/skewDur call the production masterDownInterval/skewTime, so the expected durations are not independent of the formula.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10)

else // preempt was true or priority was less (470) @ Discard the ADVERTISEMENT (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Preempt True and lower priority emits no actions; same advert with Preempt False adopts

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1)

(610) + MUST respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.3, §8.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive sets the sysctl recipe that makes the vMAC macvlan the ARP responder; the negative asserts the parent knobs are restored on last teardown, a neighbouring property, not a Master that fails to answer ARP. No tagged unit shows the Master holds the VIP it answers for

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L72) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2)

(620) + MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Master installs the IPv6 VIP on the vMAC macvlan (kernel joins its solicited-node group); Backup installs nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L720) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L716) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3)

(625) + MUST respond to ND Neighbor Solicitation message for the IPv6 address(es) associated with the virtual router. (§6.4.3, §8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Master installs the IPv6 VIP on the vMAC macvlan so it answers NS; Backup installs nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L722) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L721) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4)

(630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-4, so no unit is bound to it.

### [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5)

(645) - MUST forward packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. both units assert VIP install state, which decides local delivery; forwarding a frame whose destination MAC is the virtual router MAC does not depend on the VIP being installed, and no tagged unit observes forwarding

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L392) | unit/verify | revert, verified |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L428) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6)

(650) - MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Accept_Mode True positive and the neither-condition negative are sound, but the address-owner half of the sentence is proven only by TestActiveAddressOwnerAcceptsWhateverAcceptModeSays, which carries RFC9568 tags and no RFC5798-6.4.3-6 tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L242) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7)

MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, MUST NOT accept these packets. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a Master that is neither owner nor Accept_Mode True accepting packets to the VIP. TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery asserts filters[0].accept == false; TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery asserts accept == true for Accept_Mode True. The quote bounds the prohibition by two exemptions, and the owner exemption is proven only by TestActiveAddressOwnerAcceptsWhateverAcceptModeSays, which carries RFC9568 tags and no RFC5798-6.4.3-7 tag, so a suppression applied to the owner too leaves every tagged unit green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L204) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8)

If a Shutdown event is received, then: (660) + Cancel the Adver_Timer (665) + Send an ADVERTISEMENT with Priority = 0 (670) + Transition to the {Initialize} state (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. master/shutdown asserts StopTimers, SendAdvertZeroPriority, RemoveVIPs, Initialize; Backup shutdown sends no zero-priority advert

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L126) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9)

If the Adver_Timer fires, then: (685) + Send an ADVERTISEMENT (690) + Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. armed-generation expiry sends an advert and re-arms at 1s; a stale generation does nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L641) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L130) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10)

If the Priority in the ADVERTISEMENT is zero, then: (710) -* Send an ADVERTISEMENT (715) -* Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Priority 0 advert sends an advert and re-arms the Adver_Timer; a losing non-zero advert does not re-arm

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L132) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11)

If the Priority in the ADVERTISEMENT is greater than the local Priority, (730) -* or (735) -* If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX Address of the sender is greater than the local primary IPvX Address, then: (740) -@ Cancel Adver_Timer (745) -@ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (750) -@ Recompute the Skew_Time (755) @ Recompute the Master_Down_Interval (760) @ Set Master_Down_Timer to Master_Down_Interval (765) @ Transition to the {Backup} state (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. higher priority and equal-priority greater-IP rows assert StopTimers, StartMasterDownTimer(mdDur(cfg,4000)) and Backup; losing rows stay Master with no timer. DF-VRRP-5 re-read 2026-09-27, strictly: the matrix rows configure LocalPrimaryIP directly, so they never proved which address the engine uses as 'the local primary IPvX Address'; that was a live defect (the first virtual address). internal/plugins/vrrp/tiebreak_test.go::TestInstanceTieBreakComparesTheAdvertisementSource now proves it for IPv4 VRRPv3: sender above the transport's source demotes, sender below it holds, red under a first-virtual-address operand (recorded). The IPv6 half of the operand (the virtual-MAC device's link-local the transport pins) is proven only by the untagged TestAdvertSourceIPv6IsThePinnedLinkLocal at the transport seam; no tagged unit drives an IPv6 election through the engine. RA-VRRP strict re-read 2026-09-27 (independent): agrees weak. Proven: higher priority and equal-priority-greater rows assert the exact action slice [StopTimers, RemoveVIPs, StartMasterDownTimer{mdDur(cfg,4000)}, EmitStateChange Backup], which pins Cancel Adver_Timer, the adopted Adver Interval and the recomputed Skew_Time/Master_Down_Interval; losing rows (lower priority, smaller 192.0.2.5 vs local 192.0.2.10, which also catches a string compare) stay Master; the IPv4 local operand is the wire source (tiebreak test). Missing: the IPv6 'IPvX' operand through the engine (only the untagged TestAdvertSourceIPv6IsThePinnedLinkLocal).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L138) | unit/verify | revert, verified |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L78) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12)

else // new Master logic (775) @ Discard ADVERTISEMENT (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. losing rows leave state and timers untouched; the extra SendAdvert they assert is RFC 9568 Section 6.4.3's added step, recorded on the row's superseded marker, and does not act on the advert's contents

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L142) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L140) | unit/verify | revert, verified |

### [`RFC5798-7.1-1`](#rfc5798-7.1-1)

- MUST verify that the VRRP version is 3. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. version 3 matching the group accepted; v2 at a v3 group asserts ErrVersion

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L611) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80) | unit/verify | revert, verified |

### [`RFC5798-7.1-2`](#rfc5798-7.1-2)

- MUST verify that the received packet contains the complete VRRP packet (including fixed fields, and IPvX address). (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: accepting a packet that does not contain the complete VRRP packet, i.e. shorter than 8 + count*4. TestDecodeGoldenV3IPv4 accepts the exact length; the tagged negative (TestNegativeReferenceBugs N2) feeds a LONGER packet (8-byte trailer) and asserts ErrLength. No tagged unit feeds a short packet or a count that overstates the addresses present (those cases live in TestValidationOrder, untagged for this id), so a check written len > want instead of len != want stays green while accepting incomplete packets.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L511) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L83) | unit/verify | revert, verified |

### [`RFC5798-7.1-3`](#rfc5798-7.1-3)

- MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-3, so no unit is bound to it.

### [`RFC5798-7.1-4`](#rfc5798-7.1-4)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence forbids a packet that fails any of the five mandatory checks (TTL/Hop Limit, version, completeness, checksum, VRID/owner) reaching further processing. TestInstanceRxValidAdvertReachesFSM proves a passing packet reaches the FSM. TestInstanceRxDecodeErrorMapsReason feeds only a truncated packet (one of five checks) and asserts only that a reason was recorded in rxErrors; it never reads in.events, so an onPacket that recorded the error and still dispatched would pass, and the TTL, version, checksum and VRID/owner failures are not exercised by this unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L527) | unit/verify | revert, verified |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L547) | unit/verify | revert, verified |

### [`RFC5798-7.2-1`](#rfc5798-7.2-1)

The following operations MUST be performed when transmitting a VRRP packet: - Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: (a) fill the packet fields from the virtual router state, (b) compute the checksum. TestEncodeGoldenV3IPv4 bytes.Equal against the golden proves WriteTo and FillChecksum encode a hand-built Advertisement correctly, but no tagged assertion reads a transmitted advert against the instance state (EffectivePriority, VIPs, Advertisement_Interval), so a tx path that filled the configured priority instead of the effective one stays green. The negative TestBoundaryInterval (Validate refuses an out-of-range interval) is a neighbouring property.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109) | unit/verify | revert, verified |

### [`RFC5798-7.2-2`](#rfc5798-7.2-2)

If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a transmitted advertisement whose source MAC is not the virtual router MAC. The tagged unit in packet_test.go asserts only the VirtualMAC derivation (00-00-5E-00-01/02-{VRID}); no tagged assertion reads the source MAC of a transmitted frame, so binding the tx socket to the parent instead of the vMAC macvlan would stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L388) | unit/verify | revert, verified |

### [`RFC5798-7.2-3`](#rfc5798-7.2-3)

If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address + Set the source IPv6 address to interface link-local IPv6 address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. IPv4 half enforced: TestSendAdvertUsesParentPrimaryV4Source reads the source from frame bytes 12-16 and requires the parent primary, re-resolved on update and address change; TestSendAdvertNoPrimaryV4SkipsAndCounts refuses to send without a primary. IPv6 half: TestSendAdvertNoLinkLocalSkipsAndCounts asserts only that nothing is sent without a link-local and that the retry sends; no tagged assertion reads the IPv6 source of a sent advert, so a non-link-local IPv6 source would pass.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L408) | unit/verify | revert, verified |
| negative | [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L360) | unit/verify | revert, verified |
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218) | unit/verify | revert, verified |

### [`RFC5798-7.2-4`](#rfc5798-7.2-4)

Set the IPvX protocol to VRRP - Send the VRRP packet to the VRRP IPvX multicast group (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a protocol other than 112 or a destination other than the VRRP IPvX multicast group. TestSendAdvertV3IPv4HeaderTTLProtoDst asserts frame[9] == packet.ProtoNumber and dst == packet.MulticastV4 for IPv4 only; the IPv6 next header and ff02::12 destination are not asserted by any tagged unit, and the comparisons are against the code constants rather than literals.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L317) | unit/verify | revert, verified |

### [`RFC5798-7.4-1`](#rfc5798-7.4-1)

IPv6 routers running VRRP MUST create their Interface Identifiers in the normal manner (e.g., "Transmission of IPv6 Packets over Ethernet Networks" [RFC2464]). (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.4-1, so no unit is bound to it.

### [`RFC5798-7.4-2`](#rfc5798-7.4-2)

They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers. (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.4-2, so no unit is bound to it.

### [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1)

The Virtual Router Master MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: the Master answering ARP with its physical MAC. The owner case is now enforced end to end by TestVRRPOwnerAnswersWithVirtualMACOnly (QEMU): control captures the parent's physical MAC in ar$sha, the owner filter (ownerARPReplyTerm) leaves only the virtual MAC. The non-owner case rests on TestDataplaneApplyIPv4SetsRecipe asserting the parent recipe, a proxy, and its negative (TestDataplaneRestoreOnLastGroup) is a teardown property. Proven in QEMU on the 7.2 runtime kernel 2026-09-27 (DF-VRRP-3 qemu.log): green, and red in the filtered ARP phase with ownerARPReplyTerm's Drop replaced by Accept. applyDataplaneSysctls now also sets arp_ignore=8 on an IPv6 group's macvlan, which the IPv4 path this row rests on does not reach RA-VRRP strict re-read 2026-09-27 (independent): agrees weak. Also: the tagged integration unit applies setOwnerFilter by hand, so the product wiring in doInstallVIPs that installs the filter for an owner Master is proven only by the untagged TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt. The integration unit carries no RFC5798-8.1.2-1 negative tag; the negative is the teardown test.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L141) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L44) | unit/verify | revert, verified |

### [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1)

If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.1.3-1, so no unit is bound to it.

### [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1)

The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: the Master answering an NS with its physical MAC. Owner case: TestVRRPOwnerAnswersWithVirtualMACOnly (QEMU) reads the Target Link-Layer Address option of every NA for the owner VIP: the no-filter control carries the parent's physical MAC, with ownerNDAdvertTerm applied only the virtual MAC. Non-owner case: TestInstanceIPv6VIPLivesOnVirtualMACDevice asserts only the install device, with IsOwner forced true on a fake parent, so no non-owner NA is observed RA-VRRP strict re-read 2026-09-27 (independent): agrees weak. The tagged integration unit applies setOwnerFilter by hand, so the doInstallVIPs wiring that installs ownerNDAdvertTerm for an owner Master has no tagged assertion (untagged TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt). The tagged negative on TestInstanceIPv6VIPLivesOnVirtualMACDevice is the same install assertion as its positive (one test wearing two hats).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L719) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L718) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L45) | unit/verify | revert, verified |

### [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2)

When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-2, so no unit is bound to it.

### [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3)

It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-3, so no unit is bound to it.

### [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4)

o At system boot, when initializing interfaces for VRRP operation, all ND Router and Neighbor Advertisements and Solicitation messages must be delayed until both the IPv6 address and the virtual router MAC address are configured. (§8.2.2; lowercase "must" in the RFC)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: delay ND messages until (a) the IPv6 address and (b) the virtual router MAC are configured. TestInstanceDelaysAnnounceUntilParentUsable/ipv6 asserts no announce before parentReady and install before announce, covering (a) for the unsolicited NA ze emits. Clause (b) rests on waitDevicePresent (register.go), which the fake deps do not model, so no tagged assertion goes red if the NA leaves before the vMAC device exists; kernel-originated RS/NS/RA are not covered either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L767) | unit/verify | revert, verified |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L765) | unit/verify | revert, verified |

### [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1)

The Backup routers must be configured to send the same Router Advertisement options as the address owner. (§8.2.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.3-1, so no unit is bound to it.

### [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1)

An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. When a virtual router is configured this way and is the Master, it MUST send both types at the configured rate, even if sub-second. (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.4.2-1, so no unit is bound to it.

### [`RFC5798-A.2-1`](#rfc5798-a.2-1)

The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. (§A.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-A.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude-opus-5 (rfcgate-6 phase, rfc5798 walk) |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/full/rfc5798.txt |
| Source fingerprint | e28505fbe523b3a8 |
| Record | rfc/extraction/rfc5798.json |
| Mapped sentences | 49 |
| Declined as scope | 13 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | skipped (front-matter) | Title block, abstract, status of this memo, copyright notice and table of contents. No obligation is stated before Section 1. |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `1.5` | not stated | 0 | walked | not stated |
| `1.6` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.1.1` | not stated | 0 | walked | not stated |
| `5.1.1.2` | not stated | 1 | walked | not stated |
| `5.1.1.3` | not stated | 2 | walked | not stated |
| `5.1.1.4` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.2.1` | not stated | 0 | walked | not stated |
| `5.1.2.2` | not stated | 1 | walked | not stated |
| `5.1.2.3` | not stated | 2 | walked | not stated |
| `5.1.2.4` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 1 | walked | not stated |
| `5.2.3` | not stated | 0 | walked | not stated |
| `5.2.4` | not stated | 2 | walked | not stated |
| `5.2.5` | not stated | 0 | walked | not stated |
| `5.2.6` | not stated | 1 | walked | not stated |
| `5.2.7` | not stated | 0 | walked | not stated |
| `5.2.8` | not stated | 0 | walked | not stated |
| `5.2.9` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 2 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.4.1` | not stated | 0 | walked | not stated |
| `6.4.2` | not stated | 6 | walked | not stated |
| `6.4.3` | not stated | 9 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 7 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 1 | walked | not stated |
| `8.1.2` | not stated | 3 | walked | not stated |
| `8.1.3` | not stated | 1 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 1 | walked | not stated |
| `8.2.2` | not stated | 5 | walked | not stated |
| `8.2.3` | not stated | 1 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `8.3.2` | not stated | 1 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.4.1` | not stated | 0 | walked | not stated |
| `8.4.2` | not stated | 2 | walked | not stated |
| `8.4.3` | not stated | 0 | walked | not stated |
| `8.4.3.1` | not stated | 0 | walked | not stated |
| `8.4.3.2` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | Section 10, Contributors and Acknowledgments | 0 | skipped (acknowledgements) | Section 10, Contributors and Acknowledgments. It names the people who wrote the merged source documents. |
| `11` | Section 11, IANA Considerations | 0 | skipped (iana) | Section 11, IANA Considerations. It records the IPv4 and IPv6 multicast assignments and the protocol number already made for VRRP, and binds IANA rather than a speaker. |
| `12` | Section 12, References, its heading only | 0 | skipped (references) | Section 12, References, its heading only. |
| `12.1` | Normative reference list | 0 | skipped (references) | Normative reference list. |
| `12.2` | Informative reference list | 0 | skipped (references) | Informative reference list. |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 2 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The sentence is the IETF Trust Legal Provisions boilerplate that opens every RFC. It binds a party redistributing the document text, states no protocol behaviour, and the extractor did not strip it. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The site is a Table of Contents line, "Required Features ... 8 2.1.", captured by the prose scan because the heading it names contains the word Required. A contents line states nothing. | Required Features ...............................................8 2.1. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The site is the section heading "Required Features" itself, matched on the word Required. A heading states no obligation; every feature it introduces is stated in Sections 2.1 to 2.5 and, normatively, in Sections 5 to 8. | Required Features |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 is the protocol overview and the sentence states a deployment precondition, that an operator gives every VRRP router on the LAN the same VRID-to-address mapping. It is not a behaviour a speaker performs on a packet: no VRRP message carries or negotiates the mapping, and ze reads its own VRID and virtual addresses from operator configuration (applyGroupLeaves, internal/plugins/vrrp/groups.go, reading the vrid and virtual-address leaves of ze-vrrp-conf.yang). No normative section of RFC 5798 restates it. | The mapping between the VRID and its IPvX address(es) must be coordinated among all VRRP routers on a LAN. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 is a worked sample configuration. The sentence describes what that example needs, "In order to back up IPvX B, a second virtual router must be configured", and states no obligation on an implementation. | In order to back up IPvX B, a second virtual router must be configured. |
| `6.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The IPv6_Addresses parameter description restates the address-ordering rule of Section 5.2.9, which site 5.2.9:1 maps. | The first address must be the Link-Local address associated with the virtual router. |
| `6.4.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence is the head of the Backup state list, "(300) While in this state, a VRRP router MUST do the following:". It states no behaviour of its own: it sets the obligation level for the bullets under it, each of which repeats MUST or MUST NOT and is a site of its own at 6.4.2:2 to 6.4.2:6. The steps it also binds that carry no keyword, (345) to (475), are declared in this section unsourced-ids. | (300) While in this state, a VRRP router MUST do the following: |
| `6.4.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence is the head of the Master state list, "(600) While in this state, a VRRP router MUST do the following:", and states no behaviour of its own, exactly as 6.4.2:1 does for Backup. Its keyworded bullets are sites 6.4.3:2 to 6.4.3:9, and the steps it binds that carry no keyword, (655) to (780), are declared in this section unsourced-ids. | (600) While in this state, a VRRP router MUST do the following: |
| `6.4.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Step (635) restates the Accept_Mode note of Section 6.1, which site 6.1:2 maps. | (635) ++ If Accept_Mode is False: MUST NOT drop IPv6 Neighbor Solicitations and Neighbor Advertisements. |
| `8.1.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8.1.2 restates the Master ARP obligation of Section 6.4.3 step (610), which site 6.4.3:2 maps. The half that is NEW here, the prohibition on answering with the physical MAC, is the separate site 8.1.2:2. | When a host sends an ARP request for one of the virtual router IPv4 addresses, the Virtual Router Master MUST respond to the ARP request with an ARP response that indicates the virtual MAC address for the virtual router. |
| `8.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8.2.2 restates the Master Neighbor Solicitation obligation of Section 6.4.3 step (625), which site 6.4.3:4 maps. The half that is NEW here, the prohibition on answering with the physical MAC, is the separate site 8.2.2:2. | When a host sends an ND Neighbor Solicitation message for the virtual router IPv6 address, the Virtual Router Master MUST respond to the ND Neighbor Solicitation message with the virtual MAC address for the virtual router. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 9 states that VRRP needs no confidentiality: "there is no information in the VRRP messages that must be kept secret from other nodes on the LAN". The word sits inside a negative existential describing the absence of a need, not an obligation on a speaker. | Confidentiality is not necessary for the correct operation of VRRP, and there is no information in the VRRP messages that must be kept secret from other nodes on the LAN. |
| `A.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The bullet explains why VRRP over Token Ring is difficult, that source-route bridges need cached source-route information updated when the Master moves. It describes a property of that medium and states no obligation on a VRRP implementation; the one Token Ring obligation is site A.2:2. | o In order to switch to a new Master located on a different bridge Token-Ring segment from the previous Master when using source- route bridges, a mechanism is required to update cached source- route information. |

## Superseded

RFC 5798 is obsoleted by RFC 9568.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) The IPv4 multicast address as assigned by the IANA for VRRP is: 224.0.0.18 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | restated | RFC9568-5.1.1.2-1 | RFC 9568 Section 5.1.1.2 keeps the rule for IPv4 word for word |
| [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1) The TTL MUST be set to 255. (§5.1.1.3) | restated | RFC9568-5.1.1.3-1 | RFC 9568 Section 5.1.1.3 keeps the IPv4 TTL of 255 on transmit |
| [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2) A VRRP router receiving a packet with the TTL not equal to 255 MUST discard the packet. (§5.1.1.3, §7.1) | restated | RFC9568-5.1.1.3-2 | RFC 9568 Section 5.1.1.3 keeps the discard rule and Section 7.1 keeps the matching receive check |
| [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1) The IPv6 multicast address assigned by the IANA for VRRP is: FF02:0:0:0:0:0:0:12 This is a link-local scope multicast address. Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | restated | RFC9568-5.1.2.2-1 | RFC 9568 Section 5.1.2.2 keeps the rule for the IPv6 group |
| [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1) The Hop Limit MUST be set to 255. (§5.1.2.3) | restated | RFC9568-5.1.2.3-1 | RFC 9568 Section 5.1.2.3 keeps the Hop Limit of 255 on transmit |
| [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2) A VRRP router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet. (§5.1.2.3, §7.1) | restated | RFC9568-5.1.2.3-2 | RFC 9568 Section 5.1.2.3 keeps the discard rule and Section 7.1 keeps the matching receive check |
| [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1) The only packet type defined in this version of the protocol is: 1 ADVERTISEMENT A packet with unknown type MUST be discarded. (§5.2.2) | restated | RFC9568-5.2.2-1 | RFC 9568 Section 5.2.2 keeps ADVERTISEMENT as the only defined type and keeps the discard rule, and Section 7.1 adds an explicit receive check for it |
| [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1) The priority value for the VRRP router that owns the IPvX address associated with the virtual router MUST be 255 (decimal). (§5.2.4) | restated | RFC9568-5.2.4-1 | RFC 9568 Section 5.2.4 keeps Priority 255 for the address owner |
| [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2) VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.2.4) | restated | RFC9568-5.2.4-2 | RFC 9568 Section 5.2.4 keeps the 1 to 254 range for backup routers |
| [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1) This field MUST be set to zero on transmission and ignored on reception. (§5.2.6) | restated | RFC9568-5.2.6-1 | RFC 9568 Section 5.2.6 renames the field to Reserve and keeps both halves of the rule |
| [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) The checksum is the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]. The next header field in the "pseudo-header" should be set to 112 (decimal) for VRRP. For computing the checksum, the checksum field is set to zero. (§5.2.8, §7.1) | restated | RFC9568-5.2.8-1 | RFC 9568 Section 5.2.8 keeps the ones-complement arithmetic and CHANGES the coverage, stating the IPv4 checksum over the VRRP message alone and keeping the RFC 8200 Section 8.1 pseudo-header for IPv6 only. RFC 9568 Section 1.2 lists that as change 4. The two documents therefore disagree about the IPv4 checksum, and the deployed base computes this one |
| [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1) For IPv6, the first address must be the IPv6 link-local address associated with the virtual router. (§5.2.9, §6.1; lowercase "must" in the RFC) | restated | RFC9568-5.2.9-1 | RFC 9568 Section 5.2.9 states the same rule with an uppercase MUST, and erratum 8300 adds the matching receive check |
| [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2) This field contains either one or more IPv4 addresses, or one or more IPv6 addresses, that is, IPv4 and IPv6 MUST NOT both be carried in one IPvX Address field. (§5.2.9) | restated | RFC9568-5.2.9-2 | RFC 9568 Section 5.2.9 states the rule as an address family that MUST be the same as the packet's IPvX header address family |
| [`RFC5798-6.1-1`](#rfc5798-6.1-1) Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3) | restated | RFC9568-6.1-1 | RFC 9568 Section 6.1 keeps the Accept_Mode note verbatim |
| [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1) (310) + MUST NOT respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-1 | RFC 9568 Section 6.4.2 keeps the rule and renames the state to Backup Router |
| [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2) (320) + MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-2 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3) (325) + MUST NOT send ND Router Advertisement messages for the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-3 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4) (335) - MUST discard packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.2) | restated | RFC9568-6.4.2-4 | RFC 9568 Section 6.4.2 keeps the discard unchanged |
| [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5) (340) - MUST NOT accept packets addressed to the IPvX address(es) associated with the virtual router. (§6.4.2) | restated | RFC9568-6.4.2-5 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6) If a Shutdown event is received, then: (350) + Cancel the Master_Down_Timer (355) + Transition to the {Initialize} state (§6.4.2) | restated | RFC9568-6.4.2-6 | RFC 9568 Section 6.4.2 keeps the Shutdown transition and renames the timer to Active_Down_Timer |
| [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7) If the Master_Down_Timer fires, then: (370) + Send an ADVERTISEMENT (375) + If the protected IPvX address is an IPv4 address, then: (380) * Broadcast a gratuitous ARP request on that interface containing the virtual router MAC address for each IPv4 address associated with the virtual router. (385) + else // ipv6 (390) * Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the virtual router. (395) * For each IPv6 address associated with the virtual router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) unset, the Override flag (O) set, the target address set to the IPv6 address of the virtual router, and the target link-layer address set to the virtual router MAC address. (400) +endif // was protected addr ipv4? (405) + Set the Adver_Timer to Advertisement_Interval (410) + Transition to the {Master} state (§6.4.2) | restated | RFC9568-6.4.2-7 | RFC 9568 Section 6.4.2 keeps the whole sequence, renames the timer and the state, and erratum 7949 corrects the gratuitous ARP to carry the virtual router IPv4 address with the virtual router MAC as the target link-layer address |
| [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8) If the Priority in the ADVERTISEMENT is zero, then: (430) * Set the Master_Down_Timer to Skew_Time (§6.4.2) | restated | RFC9568-6.4.2-8 | RFC 9568 Section 6.4.2 keeps the Skew_Time rule for a Priority 0 advertisement |
| [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9) If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: (450) @ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (455) @ Recompute the Master_Down_Interval (460) @ Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | restated | RFC9568-6.4.2-9 | RFC 9568 Section 6.4.2 keeps the rule and ADDS one step, recomputing Skew_Time beside Active_Down_Interval, which this document omits |
| [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10) else // preempt was true or priority was less (470) @ Discard the ADVERTISEMENT (§6.4.2) | restated | RFC9568-6.4.2-10 | RFC 9568 Section 6.4.2 keeps the discard unchanged |
| [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1) (610) + MUST respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.3, §8.1.2) | restated | RFC9568-6.4.3-1 | RFC 9568 Section 6.4.3 keeps the ARP obligation and renames the state to Active Router |
| [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2) (620) + MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router. (§6.4.3) | restated | RFC9568-6.4.3-2 | RFC 9568 Section 6.4.3 keeps the membership obligation unchanged |
| [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3) (625) + MUST respond to ND Neighbor Solicitation message for the IPv6 address(es) associated with the virtual router. (§6.4.3, §8.2.2) | restated | RFC9568-6.4.3-3 | RFC 9568 Section 6.4.3 keeps the obligation and ADDS that the Neighbor Advertisement carries the Router Flag set |
| [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) (630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3) | restated | RFC9568-6.4.3-4 | RFC 9568 Section 6.4.3 keeps the obligation unchanged |
| [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5) (645) - MUST forward packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.3) | restated | RFC9568-6.4.3-5 | RFC 9568 Section 6.4.3 keeps the forwarding obligation unchanged |
| [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6) (650) - MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3) | restated | RFC9568-6.4.3-6 | RFC 9568 Section 6.4.3 keeps both admitting conditions unchanged |
| [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7) MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, MUST NOT accept these packets. (§6.4.3) | restated | RFC9568-6.4.3-7 | RFC 9568 Section 6.4.3 keeps the refusal unchanged |
| [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8) If a Shutdown event is received, then: (660) + Cancel the Adver_Timer (665) + Send an ADVERTISEMENT with Priority = 0 (670) + Transition to the {Initialize} state (§6.4.3) | restated | RFC9568-6.4.3-8 | RFC 9568 Section 6.4.3 keeps the Shutdown sequence unchanged |
| [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9) If the Adver_Timer fires, then: (685) + Send an ADVERTISEMENT (690) + Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-9 | RFC 9568 Section 6.4.3 keeps the timer-expiry behaviour unchanged |
| [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10) If the Priority in the ADVERTISEMENT is zero, then: (710) -* Send an ADVERTISEMENT (715) -* Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-10 | RFC 9568 Section 6.4.3 keeps the response to a Priority 0 advertisement unchanged |
| [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11) If the Priority in the ADVERTISEMENT is greater than the local Priority, (730) -* or (735) -* If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX Address of the sender is greater than the local primary IPvX Address, then: (740) -@ Cancel Adver_Timer (745) -@ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (750) -@ Recompute the Skew_Time (755) @ Recompute the Master_Down_Interval (760) @ Set Master_Down_Timer to Master_Down_Interval (765) @ Transition to the {Backup} state (§6.4.3) | restated | RFC9568-6.4.3-11 | RFC 9568 Section 6.4.3 keeps the whole sequence and states that the sender address comparison is an unsigned integer comparison in network byte order |
| [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12) else // new Master logic (775) @ Discard ADVERTISEMENT (§6.4.3) | restated | RFC9568-6.4.3-12 | RFC 9568 Section 6.4.3 keeps the discard and ADDS a step this document does not have, sending an advertisement immediately so learning bridges relearn the segment. RFC 9568 Section 1.2 lists that as change 5 |
| [`RFC5798-7.1-1`](#rfc5798-7.1-1) - MUST verify that the VRRP version is 3. (§7.1) | restated | RFC9568-7.1-1 | RFC 9568 Section 7.1 keeps the version check |
| [`RFC5798-7.1-2`](#rfc5798-7.1-2) - MUST verify that the received packet contains the complete VRRP packet (including fixed fields, and IPvX address). (§7.1) | restated | RFC9568-7.1-3 | RFC 9568 Section 7.1 keeps the completeness check |
| [`RFC5798-7.1-3`](#rfc5798-7.1-3) - MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1) | restated | RFC9568-7.1-4 | RFC 9568 Section 7.1 keeps both halves as published, and erratum 8298 splits them, lowering the address-owner half to a SHOULD that logs rather than discards |
| [`RFC5798-7.1-4`](#rfc5798-7.1-4) If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | restated | RFC9568-7.1-6 | RFC 9568 Section 7.1 keeps the discard on a failed mandatory check, with the SHOULD log and the MAY network-management indication beside it |
| [`RFC5798-7.2-1`](#rfc5798-7.2-1) The following operations MUST be performed when transmitting a VRRP packet: - Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | restated | RFC9568-7.2-1 | RFC 9568 Section 7.2 keeps the fill-and-checksum step unchanged |
| [`RFC5798-7.2-2`](#rfc5798-7.2-2) If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address (§7.2) | restated | RFC9568-7.2-2 | RFC 9568 Section 7.2 keeps the virtual router MAC as the source link-layer address |
| [`RFC5798-7.2-3`](#rfc5798-7.2-3) If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address + Set the source IPv6 address to interface link-local IPv6 address (§7.2) | restated | RFC9568-7.2-3 | RFC 9568 Section 7.2 keeps both source-address rules unchanged |
| [`RFC5798-7.2-4`](#rfc5798-7.2-4) Set the IPvX protocol to VRRP - Send the VRRP packet to the VRRP IPvX multicast group (§7.2) | restated | RFC9568-7.2-4 | RFC 9568 Section 7.2 keeps protocol 112 and the IPvX multicast destination |
| [`RFC5798-7.4-1`](#rfc5798-7.4-1) IPv6 routers running VRRP MUST create their Interface Identifiers in the normal manner (e.g., "Transmission of IPv6 Packets over Ethernet Networks" [RFC2464]). (§7.4) | dropped | not stated | RFC 9568 Section 7.4 replaces the sentence rather than restating it. It cites RFC 8064 and RFC 7217 as the default scheme for a stable SLAAC address and states no "normal manner" obligation, so no equivalent requirement remains |
| [`RFC5798-7.4-2`](#rfc5798-7.4-2) They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers. (§7.4) | restated | RFC9568-7.4-1 | RFC 9568 Section 7.4 keeps the prohibition and restates it over the Net_Iface parameter of the RFC 7217 and RFC 8981 derivation algorithms |
| [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1) The Virtual Router Master MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2) | restated | RFC9568-8.1.2-1 | RFC 9568 Section 8.1.2 keeps the prohibition unchanged |
| [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC) | restated | RFC9568-8.1.3-1 | RFC 9568 Section 8.1.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1) The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2) | restated | RFC9568-8.2.2-1 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2) | restated | RFC9568-8.2.2-2 | RFC 9568 Section 8.2.2 keeps the obligation unchanged |
| [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | restated | RFC9568-8.2.2-3 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4) o At system boot, when initializing interfaces for VRRP operation, all ND Router and Neighbor Advertisements and Solicitation messages must be delayed until both the IPv6 address and the virtual router MAC address are configured. (§8.2.2; lowercase "must" in the RFC) | restated | RFC9568-8.2.2-4 | RFC 9568 Section 8.2.2 states the same delay with an uppercase MUST |
| [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) The Backup routers must be configured to send the same Router Advertisement options as the address owner. (§8.2.3; lowercase "must" in the RFC) | restated | RFC9568-8.2.3-1 | RFC 9568 Section 8.2.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1) An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. When a virtual router is configured this way and is the Master, it MUST send both types at the configured rate, even if sub-second. (§8.4.2) | restated | RFC9568-8.4.2-1 | RFC 9568 Section 8.4.2 keeps the obligation unchanged |
| [`RFC5798-A.2-1`](#rfc5798-a.2-1) The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no functional-address obligation |
| [`RFC5798-7.1-5`](#rfc5798-7.1-5) If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | restated | RFC9568-7.1-7 | RFC 9568 Section 7.1 keeps the log and adds rate-limiting to it |
| [`RFC5798-7.1-8`](#rfc5798-7.1-8) MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 keeps the log, adds rate-limiting, and states it in one sentence covering the Max Advertise Interval mismatch as well |
| [`RFC5798-8.1.1-1`](#rfc5798-8.1.1-1) The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision. (§8.1.1; lowercase "should" in the RFC) | unextracted | §8.1.1 | RFC 9568 Section 8.1.1 keeps the sentence for IPv4, and rfc/short/rfc9568.md declares a row for the IPv6 counterpart at RFC9568-8.2.1-1 only. The IPv4 half of that pair is an extraction hole in the successor's summary |
| [`RFC5798-8.1.2-2`](#rfc5798-8.1.2-2) When a VRRP router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for the IPv4 address it owns; it should only send ARP messages that include virtual MAC addresses. (§8.1.2) | restated | RFC9568-8.1.2-3 | RFC 9568 Section 8.1.2 keeps the rule unchanged |
| [`RFC5798-8.1.2-3`](#rfc5798-8.1.2-3) When configuring an interface, Virtual Router Master routers should broadcast a gratuitous ARP request containing the virtual router MAC address for each IPv4 address on that interface. (§8.1.2; lowercase "should" in the RFC) | restated | RFC9568-8.1.2-4 | RFC 9568 Section 8.1.2 keeps this half at SHOULD |
| [`RFC5798-8.1.2-4`](#rfc5798-8.1.2-4) At system boot, when initializing interfaces for VRRP operation, delay gratuitous ARP requests and ARP responses until both the IPv4 address and the virtual router MAC address are configured. (§8.1.2) | restated | RFC9568-8.1.2-2 | RFC 9568 Section 8.1.2 splits the boot bullet out and RAISES it to a MUST |
| [`RFC5798-8.1.2-5`](#rfc5798-8.1.2-5) When, for example, ssh access to a particular VRRP router is required, an IP address known to belong to that router must be used. (§8.1.2; lowercase "must" inside a SHOULD-level bullet list) | restated | RFC9568-8.1.2-5 | RFC 9568 Section 8.1.2 keeps the recommendation |
| [`RFC5798-8.2.1-1`](#rfc5798-8.2.1-1) The IPv6 source address of an ICMPv6 redirect should be the address that the end-host used when making its next-hop routing decision. (§8.2.1; lowercase "should" in the RFC) | restated | RFC9568-8.2.1-1 | RFC 9568 Section 8.2.1 keeps the recommendation |
| [`RFC5798-8.2.2-5`](#rfc5798-8.2.2-5) When a VRRP router restarts or boots, it SHOULD NOT send any ND messages with its physical MAC address for the IPv6 address it owns; it should only send ND messages that include virtual MAC addresses. (§8.2.2) | restated | RFC9568-8.2.2-5 | RFC 9568 Section 8.2.2 keeps the rule unchanged |
| [`RFC5798-8.2.2-6`](#rfc5798-8.2.2-6) When configuring an interface, Virtual Router Master routers should send an unsolicited ND Neighbor Advertisement message containing the virtual router MAC address for the IPv6 address on that interface. (§8.2.2; lowercase "should" in the RFC) | restated | RFC9568-8.2.2-6 | RFC 9568 Section 8.2.2 keeps the recommendation |
| [`RFC5798-8.2.3-2`](#rfc5798-8.2.3-2) Router Advertisement options that advertise special services (e.g., Home Agent Information Option) that are present in the address owner should not be sent by the address owner unless the Backup routers are prepared to assume these services in full and have a complete and synchronized database for this service. (§8.2.3; lowercase "should not" in the RFC) | restated | RFC9568-8.2.3-2 | RFC 9568 Section 8.2.3 keeps the recommendation |
| [`RFC5798-8.3.1-1`](#rfc5798-8.3.1-1) If it is not the address owner, a VRRP router SHOULD NOT forward packets addressed to the IPvX address for which it becomes Master. (§8.3.1) | restated | RFC9568-8.3.1-1 | RFC 9568 Section 8.3.1 keeps the rule and states it over both address families |
| [`RFC5798-8.3.2-1`](#rfc5798-8.3.2-1) A priority value of 255 designates a particular router as the "IPvX address owner". Care must be taken not to configure more than one router on the link in this way for a single VRID. (§8.3.2; lowercase in the RFC) | restated | RFC9568-8.3.2-1 | RFC 9568 Section 8.3.2 keeps the recommendation and adds a rate-limited log when several priority-255 advertisers are seen |
| [`RFC5798-8.3.2-2`](#rfc5798-8.3.2-2) When there are multiple Backup routers, their priority values should be uniformly distributed. For example, if one Backup router has the default priority of 100 and another Backup Router is added, a priority of 50 would be a better choice for it than 99 or 100, in order to facilitate faster convergence. (§8.3.2; lowercase in the RFC) | restated | RFC9568-8.3.2-3 | RFC 9568 Section 8.3.2 keeps the recommendation and states it as sufficiently different priorities |
| [`RFC5798-8.4.2-2`](#rfc5798-8.4.2-2) As mentioned above, this support is intended for upgrade scenarios and is NOT recommended for permanent deployments. (§8.4.2) | restated | RFC9568-8.4.2-5 | RFC 9568 Section 8.4.2 keeps the same statement |
| [`RFC5798-8.4.2-3`](#rfc5798-8.4.2-3) When a virtual router is configured this way and is the Backup, it should time out based on the rate advertised by the Master; in the case of a VRRPv2 Master, this means it must translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | restated | RFC9568-8.4.2-2 | RFC 9568 Section 8.4.2 RAISES both halves to MUST and splits them, keeping the timeout at RFC9568-8.4.2-2 and the seconds-to-centiseconds translation at RFC9568-8.4.2-3 |
| [`RFC5798-8.4.2-4`](#rfc5798-8.4.2-4) Also, a Backup should ignore VRRPv2 advertisements from the current Master if it is also receiving VRRPv3 packets from it. (§8.4.2) | restated | RFC9568-8.4.2-4 | RFC 9568 Section 8.4.2 keeps the recommendation |
| [`RFC5798-8.4.3.1-1`](#rfc5798-8.4.3.1-1) A VRRPv2 implementation should not be given a higher priority than a VRRPv2/VRRPv3 implementation it is interacting with if the VRRPv2/ VRRPv3 rate is sub-second. (§8.4.3.1) | restated | RFC9568-8.4.2.1.1-1 | RFC 9568 renumbers the subsection to 8.4.2.1.1 and keeps the recommendation |
| [`RFC5798-A.1-1`](#rfc5798-a.1-1) To avoid this, an implementation SHOULD configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device, rather than changing its hardware MAC address. (§A.1) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no unicast-MAC-filter recommendation |
| [`RFC5798-7.1-6`](#rfc5798-7.1-6) If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event, and MAY indicate via network management that an error occurred. - MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. (§7.1) | restated | RFC9568-7.1-10 | RFC 9568 Section 7.1 keeps the network-management indication as a MAY |
| [`RFC5798-7.1-7`](#rfc5798-7.1-7) MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. (§7.1) | restated | RFC9568-7.1-11 | RFC 9568 Section 7.1 keeps the address-list check as a MAY and renames the field to IPvX Addr Count |
| [`RFC5798-8.2.2-7`](#rfc5798-8.2.2-7) Note that on a restarting Master router where the VRRP protected address is the interface address, (that is, priority 255) duplicate address detection (DAD) may fail, as the Backup router may answer that it owns the address. One solution is to not run DAD in this case. (§8.2.2) | restated | RFC9568-8.2.2-7 | RFC 9568 Section 8.2.2 keeps the note |
| [`RFC5798-8.4.2-5`](#rfc5798-8.4.2-5) An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. (§8.4.2) | restated | RFC9568-8.4.2-6 | RFC 9568 Section 8.4.2 keeps the flag as a MAY |
| [`RFC5798-8.4.2-6`](#rfc5798-8.4.2-6) It MAY report when a VRRPv3 Master is *not* sending VRRPv2 packets: that suggests they don't agree on whether they're supporting VRRPv2 routers. (§8.4.2) | restated | RFC9568-8.4.2-7 | RFC 9568 Section 8.4.2 keeps the report as a MAY |
| [`RFC5798-A.2-2`](#rfc5798-a.2-2) The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. Additionally, routers MAY support the unicast mode of operation to take advantage of newer Token-Ring adapter implementations that support non-promiscuous reception for multiple unicast MAC addresses and to avoid both the multicast traffic and usage conflicts associated with the use of Token-Ring functional addresses. (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no unicast-mode permission |
