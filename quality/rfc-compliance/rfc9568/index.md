# RFC 9568 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

Partial. Every requirement this repository extracted from RFC 9568, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 67.8% | 40 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.2% | 6 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 12.9% | 13 of 101 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 86 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 18.6% | 11 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3.4% | 2 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 47 | of 59 gated MUSTs judged | 22 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 86 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 101 |
| Tagged units | 101 |
| Recorded audit verdicts | 47 |
| Discrimination records | 13 |
| Summary | `rfc/short/rfc9568.md` |
| Requirement shard | `rfc/requirements/rfc9568.md` |
| RFC text | `rfc/full/rfc9568.txt` |

## Enrolment

Enrolled: VRRP Version 3 for IPv4 and IPv6

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Default version. Advert encode/decode, the RFC 9568 Section 6.4 state machine (Backup/Master, Master_Down_Timer, skew time), priority and preemption (including preempt-delay), the address-owner priority 255 rule, centisecond Max_Advert_Int, virtual MAC 00:00:5e:00:01:{vrid} (IPv4) / 00:00:5e:00:02:{vrid} (IPv6) on a per-group macvlan, 224.0.0.18 / ff02::12 multicast, IP protocol 112, GTSM TTL/hop-limit 255 on TX and RX, gratuitous ARP and unsolicited NA on Master transition, Accept_Mode enforced on the dataplane (an Active router that is neither the address owner nor configured Accept_Mode True does not accept packets addressed to the virtual addresses, while still answering ARP and Neighbor Discovery for them and forwarding for the virtual MAC), including the Section 6.1 carve-out that never drops IPv6 Neighbor Solicitations or Advertisements.

**What the ledger says remains**

Two gaps gated in [`rfc/short/rfc9568.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9568.md). [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4): an Active router sends no ND Router Advertisement for an IPv6 Virtual Router; the only ICMPv6 message the plugin builds is the unsolicited Neighbor Advertisement ([`internal/plugins/vrrp/transport/na.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/na.go)). [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3): the solicited Neighbor Advertisement answered for a virtual IPv6 address comes from the kernel and the plugin sets no Router flag or forwarding knob for it, so the Section 6.4.3 R-bit requirement rests on host state ze does not manage.

- **Evidence caveat:** [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1) (IPv6 Hop Limit 255 on transmit) rests entirely on an integration-gated test that needs CAP_NET_RAW and CAP_NET_ADMIN ([`internal/plugins/vrrp/transport/transport_integration_linux_test.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go), build tag `integration && linux`); it runs in the privileged QEMU suite and skips everywhere else.
- **Interoperability IS proven:** ze exchanges adverts with keepalived 2.3.1 under QEMU and passes election, node-death failover, and graceful-stop scenarios, including virtual-MAC ownership of the virtual IP (a foreign host resolves the VIP to 00:00:5e:00:01:{vrid}). Experimental pending deployment hardening.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 40 | one part of the gated population |
| Annotated instead of tested | 19 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (40):** [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2), [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2), [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1), [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1), [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2), [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1), [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1), [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1), [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2), [`RFC9568-6.1-1`](#rfc9568-6.1-1), [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1), [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2), [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4), [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5), [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6), [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7), [`RFC9568-6.4.2-8`](#rfc9568-6.4.2-8), [`RFC9568-6.4.2-9`](#rfc9568-6.4.2-9), [`RFC9568-6.4.2-10`](#rfc9568-6.4.2-10), [`RFC9568-6.4.3-1`](#rfc9568-6.4.3-1), [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5), [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6), [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7), [`RFC9568-6.4.3-8`](#rfc9568-6.4.3-8), [`RFC9568-6.4.3-9`](#rfc9568-6.4.3-9), [`RFC9568-6.4.3-10`](#rfc9568-6.4.3-10), [`RFC9568-6.4.3-11`](#rfc9568-6.4.3-11), [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12), [`RFC9568-7.1-1`](#rfc9568-7.1-1), [`RFC9568-7.1-2`](#rfc9568-7.1-2), [`RFC9568-7.1-3`](#rfc9568-7.1-3), [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1), [`RFC9568-7.1-4`](#rfc9568-7.1-4), [`RFC9568-7.1-5`](#rfc9568-7.1-5), [`RFC9568-7.1-6`](#rfc9568-7.1-6), [`RFC9568-7.2-3`](#rfc9568-7.2-3), [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1), [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2), [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1), [`RFC9568-8.2.2-4`](#rfc9568-8.2.2-4)

**Annotated instead of tested (19):** [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1), [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1), [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1), [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1), [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3), [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2), [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3), [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4), [`RFC9568-7.2-1`](#rfc9568-7.2-1), [`RFC9568-7.2-2`](#rfc9568-7.2-2), [`RFC9568-7.2-4`](#rfc9568-7.2-4), [`RFC9568-7.4-1`](#rfc9568-7.4-1), [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1), [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2), [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3), [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1), [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1), [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2), [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9568-5.1.1.2-1` | Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | MUST NOT | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin forwards no IP datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine and re-emits nothing, and the tx socket scopes adverts to the link-local group with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| `RFC9568-5.1.1.3-1` | The TTL MUST be set to 255. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L314). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally writes TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the receive-side TTL!=255 discard is the separate RFC9568-5.1.1.3-2 |
| `RFC9568-5.1.1.3-2` | A VRRP Router receiving a packet with the TTL not equal to 255 MUST discard the packet [RFC5082]. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L534) |
| `RFC9568-5.1.2.2-1` | Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | MUST NOT | 5.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin forwards no IPv6 datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine, and the v6 tx socket sets IPV6_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:193 |
| `RFC9568-5.1.2.3-1` | The Hop Limit MUST be set to 255. (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L241). **negative:** no negative test. **{single-polarity}:** the hop limit is a socket option, IPV6_MULTICAST_HOPS 255 set once at openV6 internal/plugins/vrrp/transport/backend_linux.go:185, so every advertisement on that socket carries 255 and no input produces another value -- the receive-side hop-limit discard is the separate RFC9568-5.1.2.3-2. EVIDENCE DISCLOSURE: the sole positive test is internal/plugins/vrrp/transport/transport_integration_linux_test.go:239, which does NOT run in the ordinary unit suite -- the file is gated //go:build integration && linux and its setupLab (internal/plugins/vrrp/transport/transport_integration_linux_test.go:86) skips without CAP_NET_RAW (skipNoRaw, internal/plugins/vrrp/transport/transport_integration_linux_test.go:76-80) and without CAP_NET_ADMIN (internal/plugins/vrrp/transport/transport_integration_linux_test.go:96, :102, :120). It runs under the privileged QEMU integration suite; on an unprivileged host it skips and this requirement is proven by nothing. The evidence itself is genuine: the test getsockopt reads IPV6_MULTICAST_HOPS back from both the tx and the NA socket and requires 255, so the kernel accepted the value |
| `RFC9568-5.1.2.3-2` | A VRRP Router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet [RFC5082]. (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L103). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L664) |
| `RFC9568-5.2.2-1` | A packet with unknown type MUST be discarded. (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L76). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L151) |
| `RFC9568-5.2.4-1` | The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1582). **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L773). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L775) |
| `RFC9568-5.2.4-2` | VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L318). **positive:** `unit/verify` [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1583). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L320) |
| `RFC9568-5.2.5-1` | If the received count is 0, the VRRP advertisement MUST be ignored. (§5.2.5, erratum 8299) | MUST | 5.2.5 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L544) |
| `RFC9568-5.2.6-1` | The Reserve field MUST be set to zero on transmission and ignored on reception. (§5.2.6) | MUST | 5.2.6 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L106). **negative:** `unit/verify` [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L638) |
| `RFC9568-5.2.9-1` | For IPv6, the first address MUST be the IPv6 link-local address associated with the Virtual Router. (§5.2.9, §6.1, erratum 8300) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L106). **positive:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L523). **negative:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L525). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L260) |
| `RFC9568-5.2.9-2` | The address family of the addresses, IPv4 or IPv6 but not both, MUST be the same as the VRRP packet's IPvX header address family. (§5.2.9) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L547). **negative:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L549) |
| `RFC9568-6.1-1` | Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109). **negative:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L111) |
| `RFC9568-6.4.2-1` | - It MUST NOT respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L385). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L430) |
| `RFC9568-6.4.2-2` | - It MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L711). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L713) |
| `RFC9568-6.4.2-3` | - It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no ND Router Advertisement in any state -- the only ICMPv6 message the plugin builds is the unsolicited Neighbor Advertisement, type 136 at internal/plugins/vrrp/transport/na.go:25,58, and a grep for a type-134 or router-advertisement builder over internal/plugins/vrrp returns nothing, so a Backup has no path that could send one. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| `RFC9568-6.4.2-4` | * It MUST discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L387). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L432) |
| `RFC9568-6.4.2-5` | * It MUST NOT accept packets addressed to the IPvX address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L389). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L434) |
| `RFC9568-6.4.2-6` | If a Shutdown event is received, then: - Cancel the Active_Down_Timer - Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111) |
| `RFC9568-6.4.2-7` | If the Active_Down_Timer fires, then: - Send an ADVERTISEMENT - If the protected IPvX address is an IPv4 address, then: o For each IPv4 address associated with the Virtual Router, broadcast a gratuitous ARP message containing the Virtual Router IPv4 address and with the target link-layer address set to the Virtual Router MAC address. - else // IPv6 o Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the Virtual Router. o For each IPv6 address associated with the Virtual Router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) clear, the Override flag (O) set, the target address set to the IPv6 address of the Virtual Router, and the target link-layer address set to the Virtual Router MAC address. - endif // was protected address IPv4? - Set the Adver_Timer to Advertisement_Interval - Transition to the {Active} state (§6.4.2, erratum 7949) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L447). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L638) |
| `RFC9568-6.4.2-8` | If the Priority in the ADVERTISEMENT is 0, then: o Set the Active_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L113). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L115) |
| `RFC9568-6.4.2-9` | If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L117). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L119) |
| `RFC9568-6.4.2-10` | else // preempt was true and priority was less than the local priority + Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L121). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L123) |
| `RFC9568-6.4.3-1` | - It MUST respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L71). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L138) |
| `RFC9568-6.4.3-2` | - It MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the Virtual Router. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L715). **negative:** no negative test. **{single-polarity}:** the membership is the kernel's automatic consequence of the address install ze performs on promotion (doInstallVIPs internal/plugins/vrrp/instance.go:369 registers the VIP on the virtual-MAC macvlan), so there is no ze input that installs the address yet skips the join, and the not-a-member case is the Backup requirement RFC9568-6.4.2-2 |
| `RFC9568-6.4.3-3` | - It MUST respond to ND Neighbor Solicitation messages (with the Router Flag (R) set) for the IPv6 address(es) associated with the Virtual Router. (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze answers the solicitation from the virtual-MAC macvlan that holds the VIP (doInstallVIPs internal/plugins/vrrp/instance.go:369), but nothing makes that reply carry the Router flag: the only R-bit producer in the plugin is the UNSOLICITED announcement builder, naFlags internal/plugins/vrrp/transport/na.go:30 used by BuildNA na.go:64, and the solicited reply is left to the kernel while applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go:125 sets only accept_dad on the IPv6 macvlan and never a forwarding knob, so the R flag depends on host state ze does not manage |
| `RFC9568-6.4.3-4` | It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| `RFC9568-6.4.3-5` | * It MUST forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L427). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L391) |
| `RFC9568-6.4.3-6` | * It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L267). **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L241). **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L429). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L205). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L393) |
| `RFC9568-6.4.3-7` | It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, it MUST NOT accept these packets. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L203). **negative:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L268). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L243) |
| `RFC9568-6.4.3-8` | If a Shutdown event is received, then: - Cancel the Adver_Timer - Send an ADVERTISEMENT with Priority = 0 - Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L125). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L502). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L127) |
| `RFC9568-6.4.3-9` | If the Adver_Timer fires, then: - Send an ADVERTISEMENT - Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L129). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L640) |
| `RFC9568-6.4.3-10` | If the Priority in the ADVERTISEMENT is 0, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L131). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L133) |
| `RFC9568-6.4.3-11` | If the Priority in the ADVERTISEMENT is greater than the local Priority or the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX address of the sender is greater than the local primary IPvX address (based on an unsigned integer comparison of the IPvX addresses in network byte order), then: + Cancel Adver_Timer + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval + Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L135). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L76). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L137). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L77) |
| `RFC9568-6.4.3-12` | else // new Active Router logic + Discard the ADVERTISEMENT + Send an ADVERTISEMENT immediately to assert the {Active} state to the sending VRRP Router and to update any learning bridges with the correct Active VRRP Router path. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L139). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L141) |
| `RFC9568-7.1-1` | * It MUST verify that the VRRP version is 3. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L79). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L610) |
| `RFC9568-7.1-2` | * It MUST verify that the VRRP packet type is 1 (ADVERTISEMENT). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L153) |
| `RFC9568-7.1-3` | * It MUST verify that the received packet contains the complete VRRP packet (including fixed fields and the IPvX address). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L82). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L510) |
| `RFC9568-5.2.8-1` | It MUST verify the VRRP checksum. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L105). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L662) |
| `RFC9568-7.1-4` | It MUST verify that the VRID is configured on the receiving interface. (§7.1, erratum 8298) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L84). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L164) |
| `RFC9568-7.1-5` | It MUST verify that the Max Advertise Interval is non zero. (§7.1, erratum 8301) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L85). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L554) |
| `RFC9568-7.1-6` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L546). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L526) |
| `RFC9568-7.2-1` | The following operations MUST be performed when transmitting a VRRP packet: * Fill in the VRRP packet fields with the appropriate Virtual Router configuration state * Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L108). **negative:** no negative test. **{single-polarity}:** golden encodes pin every field and the checksum for v3 over IPv4 and IPv6 -- WriteTo internal/plugins/vrrp/packet/packet.go:251 plus FillChecksum internal/plugins/vrrp/packet/checksum.go:86 -- while rejecting a corrupted encoding is the separate receive requirement RFC9568-7.1-3 |
| `RFC9568-7.2-2` | * Set the source MAC address to the Virtual Router MAC address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L387). **negative:** no negative test. **{single-polarity}:** the source MAC is the derived Virtual Router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97, egressed by binding the tx socket to that vMAC macvlan (internal/plugins/vrrp/transport/backend_linux.go:133 for IPv4, :179 for IPv6), a deterministic derivation with no input that yields another MAC |
| `RFC9568-7.2-3` | * If the protected address is an IPv4 address, then: - Set the source IPv4 address to the interface's primary IPv4 address * else // IPv6 - Set the source IPv6 address to the interface's link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L217). **negative:** `unit/verify` [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L409). **negative:** `unit/verify` [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L361) |
| `RFC9568-7.2-4` | * Set the IPvX protocol to VRRP * Send the VRRP packet to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L316). **negative:** no negative test. **{single-polarity}:** buildIPv4Header writes protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 / ff02::12 at internal/plugins/vrrp/transport/backend_linux.go:242,256, all constants with no input that changes them |
| `RFC9568-7.4-1` | The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]. (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no interface identifiers -- a grep for 7217, 8981, addr_gen_mode or stable-privacy over internal/plugins/vrrp returns nothing, and the only IPv6 knob the plugin writes on a virtual-MAC device is accept_dad (internal/plugins/vrrp/dataplane_linux.go:135), so no ze code feeds the Virtual Router MAC into an IID derivation |
| `RFC9568-8.1.2-1` | The Active Router MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2) | MUST NOT | 8.1.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L73). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L140) |
| `RFC9568-8.1.2-2` | * At system boot, when initializing interfaces for VRRP operation, gratuitous ARP messages MUST be delayed until both the IPv4 address and the Virtual Router MAC address are configured. (§8.1.2) | MUST | 8.1.2 | **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L762). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L763) |
| `RFC9568-8.1.3-1` | If Proxy ARP is to be used on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC address in the Proxy ARP message. (§8.1.3) | MUST | 8.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no proxy ARP for virtual addresses -- a grep for proxy_arp over internal/plugins/vrrp returns nothing, and the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64,73) |
| `RFC9568-8.2.2-1` | The Active Router MUST NOT respond with its physical MAC address. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L717). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L47). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L48) |
| `RFC9568-8.2.2-2` | When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link- layer address option in the Neighbor Solicitation message. (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56, and no type-135 builder or NS send path exists anywhere under internal/plugins/vrrp |
| `RFC9568-8.2.2-3` | It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no Neighbor Solicitation, so no source link-layer address option is authored by the plugin -- its only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56 |
| `RFC9568-8.2.2-4` | * At system boot, when initializing interfaces for VRRP operation, all ND Router Advertisements, ND Neighbor Advertisements, and ND Neighbor Solicitation messages MUST be delayed until both the IPv6 address and the Virtual Router MAC address are configured. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L764). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L766) |
| `RFC9568-8.2.3-1` | The Backup Routers MUST be configured to send the same Router Advertisement options as the address owner. (§8.2.3) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the Router Advertisement option set, and ze offers no RA surface for a Virtual Router at all -- the vrrp YANG group has no RA leaf (applyGroupLeaves internal/plugins/vrrp/groups.go:355 accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only) and the plugin builds no RA. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| `RFC9568-8.4.2-1` | When a Virtual Router is configured this way and is the Active Router, it MUST send both types at the configured rate, even if it is sub-second. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no VRRPv2/VRRPv3 dual mode -- a group runs exactly one version (parseVersion internal/plugins/vrrp/groups.go:429 admits 2 or 3, doSendAdvert internal/plugins/vrrp/instance.go:350 encodes that single spec.Version), and the receive ladder discards any advert whose wire version differs from the group's at internal/plugins/vrrp/packet/validate.go:149, so no dual-send path exists |
| `RFC9568-8.4.2-2` | When a Virtual Router is configured this way and is the Backup Router, it MUST time out based on the rate advertised by the Active Router. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this requirement is scoped to the Section 8.4.2 interop mode, which ze does not implement -- one version per group (parseVersion internal/plugins/vrrp/groups.go:429) and a version-mismatch discard at internal/plugins/vrrp/packet/validate.go:149 mean a VRRPv2 Active Router is never heard by a v3 group at all. Native v3 interval adoption is the separate RFC9568-6.4.2-9 |
| `RFC9568-8.4.2-3` | In the case of a VRRPv2 Active Router, this means it MUST translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** with no interop mode, a v3 group never accepts a VRRPv2 advert to translate -- the receive ladder discards a wire version that differs from the configured group version at internal/plugins/vrrp/packet/validate.go:149; the seconds/centiseconds conversion helpers exist per version (v2SecondsToMS and v3CentisecondsToMS internal/plugins/vrrp/packet/packet.go:233,240) and are never combined in one group |
| `RFC9568-2.5-1` | If the Active Router observes that this is occurring, it SHOULD log the problem (subject to rate-limiting). (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-7` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (subject to rate-limiting) (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-8` | A receiver SHOULD also verify that the Max Advertise Interval in the received VRRP packet matches the Advertisement_Interval configured for the VRID. Instability can occur with differing intervals (refer to Section 5.2.7). If this check fails, the receiver SHOULD log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-9` | If this check fails, the receiver SHOULD log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. A receiver MAY also verify that "IPvX Addr Count" and the list of IPvX address(es) match the IPvX address(es) configured for the VRID. If this check fails, the receiver SHOULD log (subject to rate- limiting) the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-12` | It SHOULD verify that the local router is not the IPvX address owner (Priority = 255 (decimal)) and log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1, erratum 8298) | SHOULD | 7.1 | **positive:** `unit/verify` [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_conflict_test.go#L65). **negative:** `unit/verify` [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_conflict_test.go#L66) |
| `RFC9568-8.1.2-3` | When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner (as defined in Section 1.7), and it should only send ARP messages that include Virtual Router MAC addresses. (§8.1.2) | SHOULD NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-4` | * When configuring an interface, Active Routers SHOULD broadcast a gratuitous ARP message containing the Virtual Router MAC address for each IPv4 address on that interface. (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-5` | * When, for example, Secure Shell (SSH) access to a particular VRRP Router is required, an IPv4 address known to belong to that router SHOULD be used. (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.1-1` | The IPv6 source address of an ICMPv6 redirect SHOULD be the address that the end-host used when making its next-hop routing decision. (§8.2.1) | SHOULD | 8.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-5` | When a VRRP Router restarts or boots, it SHOULD NOT send any ND messages with its physical MAC address for the IPv6 address it owns and it should only send ND messages that include Virtual Router MAC addresses. (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-6` | * When configuring an interface, Active Routers SHOULD send an unsolicited ND Neighbor Advertisement message containing the Virtual Router MAC address for the IPv6 address on that interface. (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.3-2` | Router Advertisement options that advertise special services, e.g., Home Agent Information Option, that are present in the address owner SHOULD NOT be sent by the address owner unless the Backup Routers are prepared to assume these services in full and have a complete and synchronized database for this service. (§8.2.3) | SHOULD NOT | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.4-1` | A VRRP Router acting as either an IPv6 Active Router or Backup Router SHOULD accept Unsolicited Neighbor Advertisements and update the corresponding neighbor cache [RFC4861]. (§8.2.4) | SHOULD | 8.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.1-1` | If it is not the address owner, a VRRP Router SHOULD NOT forward packets addressed to the IPvX address for which it becomes the Active Router. (§8.3.1) | SHOULD NOT | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-1` | For a VRID, only a single VRRP Router on the link SHOULD be configured with priority 255. (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-2` | If multiple VRRP Routers advertising priority 255 are detected, the condition SHOULD be logged (subject to rate-limiting). (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-3` | In order to avoid two or more Backup Routers simultaneously becoming Active Routers after the previous Active Router fails or is shut down, all Virtual Routers SHOULD be configured with different priorities and with sufficient differences in the priorities so that lower priority Backup Routers do not transition to the Active state before receiving an advertisement from the highest priority Backup Router when it transitions to the Active Router. (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-4` | Also, a Backup Router SHOULD ignore VRRPv2 advertisements from the current Active Router if it is also receiving VRRPv3 packets from it. (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2.1.1-1` | A VRRPv2 implementation SHOULD NOT be given a higher priority than a VRRPv2 or VRRPv3 implementation with which it is interoperating if the VRRPv2 or VRRPv3 router's advertisement rate is sub-second. (§8.4.2.1.1) | SHOULD NOT | 8.4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-5` | As mentioned above, this support is intended for upgrade scenarios and is NOT RECOMMENDED for permanent deployments. (§8.4.2) | NOT RECOMMENDED | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-9-1` | Also in the context of IPv6 operation, it is RECOMMENDED that the link-level security guidelines in Section 2.3 of [RFC9099] be followed. (§9) | RECOMMENDED | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-10` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (subject to rate-limiting), and MAY indicate via network management that an error occurred. A receiver SHOULD also verify that the Max Advertise Interval in the received VRRP packet matches the Advertisement_Interval configured for the VRID. Instability can occur with differing intervals (refer to Section 5.2.7). If this check fails, the receiver SHOULD log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-11` | A receiver MAY also verify that "IPvX Addr Count" and the list of IPvX address(es) match the IPvX address(es) configured for the VRID. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-7` | Note that on a restarting Active Router where the VRRP protected address is an interface address, i.e., the address owner, Duplicate Address Detection may fail, as the Backup Router MAY answer that it owns the address. One solution is to not run Duplicate Address Detection in this case. (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-4` | If multiple VRRP Routers advertising the same priority are detected, this condition MAY be logged as a warning (subject to rate-limiting). (§8.3.2) | MAY | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-6` | An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-7` | It MAY report when a VRRPv3 Active Router is not sending VRRPv2 packets, as this suggests they don't agree on whether they're supporting VRRPv2 interoperation. (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1) Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin forwards no IP datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine and re-emits nothing, and the tx socket scopes adverts to the link-local group with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1) Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin forwards no IPv6 datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine, and the v6 tx socket sets IPV6_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:193 |
| [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3) - It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no ND Router Advertisement in any state -- the only ICMPv6 message the plugin builds is the unsolicited Neighbor Advertisement, type 136 at internal/plugins/vrrp/transport/na.go:25,58, and a grep for a type-134 or router-advertisement builder over internal/plugins/vrrp returns nothing, so a Backup has no path that could send one. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3) - It MUST respond to ND Neighbor Solicitation messages (with the Router Flag (R) set) for the IPv6 address(es) associated with the Virtual Router. (§6.4.3, §8.2.2) | {gap}, no test | ze answers the solicitation from the virtual-MAC macvlan that holds the VIP (doInstallVIPs internal/plugins/vrrp/instance.go:369), but nothing makes that reply carry the Router flag: the only R-bit producer in the plugin is the UNSOLICITED announcement builder, naFlags internal/plugins/vrrp/transport/na.go:30 used by BuildNA na.go:64, and the solicited reply is left to the kernel while applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go:125 sets only accept_dad on the IPv6 macvlan and never a forwarding knob, so the R flag depends on host state ze does not manage |
| [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4) It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3) | {gap}, no test | ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| [`RFC9568-7.4-1`](#rfc9568-7.4-1) The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]. (§7.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no interface identifiers -- a grep for 7217, 8981, addr_gen_mode or stable-privacy over internal/plugins/vrrp returns nothing, and the only IPv6 knob the plugin writes on a virtual-MAC device is accept_dad (internal/plugins/vrrp/dataplane_linux.go:135), so no ze code feeds the Virtual Router MAC into an IID derivation |
| [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1) If Proxy ARP is to be used on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC address in the Proxy ARP message. (§8.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no proxy ARP for virtual addresses -- a grep for proxy_arp over internal/plugins/vrrp returns nothing, and the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64,73) |
| [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2) When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link- layer address option in the Neighbor Solicitation message. (§8.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56, and no type-135 builder or NS send path exists anywhere under internal/plugins/vrrp |
| [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3) It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no Neighbor Solicitation, so no source link-layer address option is authored by the plugin -- its only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56 |
| [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1) The Backup Routers MUST be configured to send the same Router Advertisement options as the address owner. (§8.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: this governs the Router Advertisement option set, and ze offers no RA surface for a Virtual Router at all -- the vrrp YANG group has no RA leaf (applyGroupLeaves internal/plugins/vrrp/groups.go:355 accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only) and the plugin builds no RA. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1) When a Virtual Router is configured this way and is the Active Router, it MUST send both types at the configured rate, even if it is sub-second. (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no VRRPv2/VRRPv3 dual mode -- a group runs exactly one version (parseVersion internal/plugins/vrrp/groups.go:429 admits 2 or 3, doSendAdvert internal/plugins/vrrp/instance.go:350 encodes that single spec.Version), and the receive ladder discards any advert whose wire version differs from the group's at internal/plugins/vrrp/packet/validate.go:149, so no dual-send path exists |
| [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2) When a Virtual Router is configured this way and is the Backup Router, it MUST time out based on the rate advertised by the Active Router. (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: this requirement is scoped to the Section 8.4.2 interop mode, which ze does not implement -- one version per group (parseVersion internal/plugins/vrrp/groups.go:429) and a version-mismatch discard at internal/plugins/vrrp/packet/validate.go:149 mean a VRRPv2 Active Router is never heard by a v3 group at all. Native v3 interval adoption is the separate RFC9568-6.4.2-9 |
| [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3) In the case of a VRRPv2 Active Router, this means it MUST translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: with no interop mode, a v3 group never accepts a VRRPv2 advert to translate -- the receive ladder discards a wire version that differs from the configured group version at internal/plugins/vrrp/packet/validate.go:149; the seconds/centiseconds conversion helpers exist per version (v2SecondsToMS and v3CentisecondsToMS internal/plugins/vrrp/packet/packet.go:233,240) and are never combined in one group |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1)

Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.1.2-1, so no unit is bound to it.

### [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1)

The TTL MUST be set to 255. (§5.1.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestSendAdvertV3IPv4HeaderTTLProtoDst reads byte 8 of the frame SendAdvert emits and requires 255; single-polarity, buildIPv4Header writes a constant

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L314) | unit/verify | unproven |

### [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2)

A VRRP Router receiving a packet with the TTL not equal to 255 MUST discard the packet [RFC5082]. (§5.1.1.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: the check clause is enforced (golden accepts TTL 255; N4 feeds TTL 64 and requires ErrTTL). The 'MUST discard the packet' clause is not: the discard is the return after recordRxError in instance.onPacket (instance.go:669-671), and no tagged unit asserts the FSM sees nothing; removing that return leaves every tagged assertion green (same hole as RFC9568-7.1-6)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L534) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74) | unit/verify | unproven |

### [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1)

Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.2.2-1, so no unit is bound to it.

### [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1)

The Hop Limit MUST be set to 255. (§5.1.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestIntegrationOpenInstanceSocketOptions/v6 reads IPV6_MULTICAST_HOPS back from the tx and NA sockets and requires 255; runs only under integration && linux with CAP_NET_RAW/ADMIN, as the row's marker discloses

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L241) | unit/verify | unproven |

### [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2)

A VRRP Router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet [RFC5082]. (§5.1.2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: the check clause is enforced (v6 golden accepts hop limit 255; TestDecodeV3IPv6ChecksumAndHopLimit feeds 64 and requires ErrTTL). The 'MUST discard the packet' clause is not: no tagged unit asserts that a Decode error keeps the advert from the FSM (the return at instance.go:671), so removing it stays green (same hole as RFC9568-7.1-6)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L664) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L103) | unit/verify | unproven |

### [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1)

A packet with unknown type MUST be discarded. (§5.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: TestValidationOrder 3<4 requires ErrType for type 2 and the golden accepts type 1, so the type is checked. The 'MUST be discarded' clause has no assertion: no tagged unit shows a type-2 packet never reaches the FSM (the return at instance.go:671 is untested; same hole as RFC9568-7.1-6)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L151) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L76) | unit/verify | unproven |

### [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1)

The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestOwnerAutoDetection requires EffectivePriority 255 for the owner configured at 120 and the configured 100 for a non-owner; TestEffectivePriorityWithTracking keeps the owner at 255 under any decrement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L775) | unit/verify | revert, verified |
| positive | [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1582) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L773) | unit/verify | revert, verified |

### [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2)

VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBoundaryPriority accepts 1 and 254 and rejects 0 and 255 through extractGroupSpecs/validateGroups; TestEffectivePriorityWithTracking floors a tracked decrement at 1

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L320) | unit/verify | unproven |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L318) | unit/verify | unproven |
| positive | [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1583) | unit/verify | revert, verified |

### [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1)

If the received count is 0, the VRRP advertisement MUST be ignored. (§5.2.5, erratum 8299)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: N5 count-zero requires ErrCountZero on an exact-length count-0 advert and the golden accepts count 2, so the count is checked. The 'MUST be ignored' clause has no assertion: no tagged unit shows the count-0 advert never reaches the FSM (the return at instance.go:671 is untested; same hole as RFC9568-7.1-6)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L544) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78) | unit/verify | unproven |

### [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1)

The Reserve field MUST be set to zero on transmission and ignored on reception. (§5.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestEncodeGoldenV3IPv4 requires the Reserve nibble 0 on transmission and exact golden bytes; TestDecodeV3ReserveIgnoredOnReceive sets all Reserve bits, re-checksums, and requires acceptance with the interval unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L638) | unit/verify | unproven |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L106) | unit/verify | unproven |

### [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1)

For IPv6, the first address MUST be the IPv6 link-local address associated with the Virtual Router. (§5.2.9, §6.1, erratum 8300)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestValidateIPv6LinkLocal refuses a global-first IPv6 group and accepts a link-local-first one (transmit side); TestValidationOrder row 13 rejects a received global-first advert with ErrFirstNotLinkLocal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L525) | unit/verify | unproven |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L260) | unit/verify | unproven |
| positive | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L523) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L106) | unit/verify | unproven |

### [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2)

The address family of the addresses, IPv4 or IPv6 but not both, MUST be the same as the VRRP packet's IPvX header address family. (§5.2.9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: the ipv4-group-ipv6-vip case is isolated (only the family check can refuse it). The inverse, ipv6-group-ipv4-vip, puts the IPv4 address first, so the IPv6 first-address-link-local rule also refuses it, and the test asserts only err != nil: deleting the family check keeps that case green. The IPv6-packet half of 'IPv4 or IPv6 but not both' is cascade-confounded

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L549) | unit/verify | unproven |
| positive | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L547) | unit/verify | unproven |

### [`RFC9568-6.1-1`](#rfc9568-6.1-1)

Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop requires ICMPv6 135 and 136 accept terms ahead of the first drop term in the Accept_Mode False table, and fails if the table has no drop

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L111) | unit/verify | unproven |
| positive | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1)

- It MUST NOT respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestInstanceStartupNonOwnerGoesBackup requires a Backup to install no virtual address, the state Ze hands the kernel that answers ARP; TestInstanceOwnerStartupGoesMaster is the Active contrast

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L430) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L385) | unit/verify | unproven |

### [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2)

- It MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestInstanceIPv6VIPLivesOnVirtualMACDevice requires a Backup to install no IPv6 virtual address and the Active router to install it on the vMAC macvlan only

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L713) | unit/verify | unproven |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L711) | unit/verify | unproven |

### [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3)

- It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.2-3, so no unit is bound to it.

### [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4)

* It MUST discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units assert only that a Backup installs no virtual address. That governs local delivery to the VIP, not the discard of frames whose destination MAC is the Virtual Router MAC (transit traffic sent to the gateway MAC); no tagged unit shows the vMAC macvlan drops or is absent in Backup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L432) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L387) | unit/verify | unproven |

### [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5)

* It MUST NOT accept packets addressed to the IPvX address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestInstanceStartupNonOwnerGoesBackup requires no virtual address installed in Backup, so the kernel accepts nothing addressed to it; Active contrast installs it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L434) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L389) | unit/verify | unproven |

### [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6)

If a Shutdown event is received, then: - Cancel the Active_Down_Timer - Transition to the {Initialize} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestFSMTransitionMatrix backup/shutdown requires exactly StopTimers and a transition to Initialize; the negative is a contrast row (stale timer event keeps Backup)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7)

If the Active_Down_Timer fires, then: - Send an ADVERTISEMENT - If the protected IPvX address is an IPv4 address, then: o For each IPv4 address associated with the Virtual Router, broadcast a gratuitous ARP message containing the Virtual Router IPv4 address and with the target link-layer address set to the Virtual Router MAC address. - else // IPv6 o Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the Virtual Router. o For each IPv6 address associated with the Virtual Router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) clear, the Override flag (O) set, the target address set to the IPv6 address of the Virtual Router, and the target link-layer address set to the Virtual Router MAC address. - endif // was protected address IPv4? - Set the Adver_Timer to Advertisement_Interval - Transition to the {Active} state (§6.4.2, erratum 7949)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27 against the quote re-taken from erratum 7949. TestFSMMasterDownPromotion asserts the promotion emits SendAdvert, InstallVIPs, AnnounceFailover, StartAdvertTimer, EmitStateChange in that order and ends in Master; TestFSMStaleTimerGenerationIgnored binds promotion to the live timer. No tagged unit asserts what the sentence requires of each step: the gratuitous ARP carrying the virtual IPv4 address with target link-layer address the Virtual Router MAC (erratum 7949), the join of the Solicited-Node multicast address, the unsolicited NA with R set, S clear, O set, target the virtual IPv6 address and TLL the Virtual Router MAC, one announcement per associated address, or the Adver_Timer set to Advertisement_Interval (StartAdvertTimer's interval is not read). The frame builders (transport/garp.go buildGARP, transport/na.go BuildNA) carry those fields, but their tests are not tagged on this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L638) | unit/verify | unproven |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L447) | unit/verify | unproven |

### [`RFC9568-6.4.2-8`](#rfc9568-6.4.2-8)

If the Priority in the ADVERTISEMENT is 0, then: o Set the Active_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row backup/advert/priority-zero requires exactly StartMasterDownTimer with skewDur; equal-priority row arms mdDur instead

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L115) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L113) | unit/verify | unproven |

### [`RFC9568-6.4.2-9`](#rfc9568-6.4.2-9)

If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row backup/advert/adopt/equal-priority requires the down timer at mdDur(cfg, 4000), the recomputed interval from the advertised 4000 ms; the discard row emits no action

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L119) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L117) | unit/verify | unproven |

### [`RFC9568-6.4.2-10`](#rfc9568-6.4.2-10)

else // preempt was true and priority was less than the local priority + Discard the ADVERTISEMENT (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row preempt-lower-priority-no-delay requires no actions (discard); preempt-false-lower-priority re-arms instead, so the discard is bound to Preempt_Mode True

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L123) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L121) | unit/verify | unproven |

### [`RFC9568-6.4.3-1`](#rfc9568-6.4.3-1)

- It MUST respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.3, §8.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: TestDataplaneApplyIPv4SetsRecipe pins the sole-responder sysctl recipe (positive), but the row has no {single-polarity} marker and the tagged negative TestDataplaneRestoreOnLastGroup proves teardown restoration, a neighbouring lifecycle rule, not an Active router failing to answer ARP for the virtual address. Negative polarity missing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L138) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L71) | unit/verify | unproven |

### [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2)

- It MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the Virtual Router. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestInstanceIPv6VIPLivesOnVirtualMACDevice requires the Active router to install the IPv6 address on the vMAC macvlan, which makes the kernel join the Solicited-Node group

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L715) | unit/verify | unproven |

### [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3)

- It MUST respond to ND Neighbor Solicitation messages (with the Router Flag (R) set) for the IPv6 address(es) associated with the Virtual Router. (§6.4.3, §8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.3-3, so no unit is bound to it.

### [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4)

It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.3-4, so no unit is bound to it.

### [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5)

* It MUST forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units assert only that the Active router installs the virtual address on the vMAC macvlan; forwarding of transit packets whose destination MAC is the Virtual Router MAC depends on the macvlan being up and forwarding, which no tagged unit asserts

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L391) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L427) | unit/verify | unproven |

### [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6)

* It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. owner installs and accepts (instance and acceptfilter tests), Accept_Mode True hands no suppression, and a non-owner with Accept_Mode False gets a suppression: both conditions and the contrast are asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L205) | unit/verify | unproven |
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L393) | unit/verify | unproven |
| positive | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L267) | unit/verify | unproven |
| positive | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L241) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L429) | unit/verify | unproven |

### [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7)

It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, it MUST NOT accept these packets. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery requires accept=false for exactly the VIP; Accept_Mode True and owner contrasts require accept=true

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L268) | unit/verify | unproven |
| negative | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L243) | unit/verify | unproven |
| positive | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L203) | unit/verify | unproven |

### [`RFC9568-6.4.3-8`](#rfc9568-6.4.3-8)

If a Shutdown event is received, then: - Cancel the Adver_Timer - Send an ADVERTISEMENT with Priority = 0 - Transition to the {Initialize} state (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row master/shutdown requires StopTimers, SendAdvertZeroPriority, RemoveVIPs, Initialize; TestInstanceShutdownAsMasterSendsPriorityZero proves the executor sends priority 0; backup/shutdown sends none

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L127) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L125) | unit/verify | unproven |
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L502) | unit/verify | unproven |

### [`RFC9568-6.4.3-9`](#rfc9568-6.4.3-9)

If the Adver_Timer fires, then: - Send an ADVERTISEMENT - Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row master/advert-timer-expired/matching-gen requires SendAdvert and StartAdvertTimer at 1s; a stale generation sends nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L640) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L129) | unit/verify | unproven |

### [`RFC9568-6.4.3-10`](#rfc9568-6.4.3-10)

If the Priority in the ADVERTISEMENT is 0, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row master/advert/priority-zero requires SendAdvert and StartAdvertTimer; a losing non-zero advert sends without re-arming

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L133) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L131) | unit/verify | unproven |

### [`RFC9568-6.4.3-11`](#rfc9568-6.4.3-11)

If the Priority in the ADVERTISEMENT is greater than the local Priority or the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX address of the sender is greater than the local primary IPvX address (based on an unsigned integer comparison of the IPvX addresses in network byte order), then: + Cancel Adver_Timer + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval + Transition to the {Backup} state (§6.4.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. higher-priority and equal-priority-greater-ip rows require StopTimers, RemoveVIPs, StartMasterDownTimer at mdDur(cfg, 4000) and Backup; losing rows stay Active with no timer change. DF-VRRP-5 re-read 2026-09-27, strictly: the matrix rows configure LocalPrimaryIP directly, so they never proved which address the engine compares as 'the local primary IPvX address'; that was a live defect (the first virtual address). internal/plugins/vrrp/tiebreak_test.go::TestInstanceTieBreakComparesTheAdvertisementSource now proves it for IPv4 VRRPv3: sender above the transport's source demotes, sender below it holds, red under a first-virtual-address operand (recorded). The IPv6 operand (the link-local the transport pins, RFC 9568 Section 7.2) is proven only by the untagged TestAdvertSourceIPv6IsThePinnedLinkLocal; no tagged unit drives an IPv6 election through the engine. RA-VRRP strict re-read 2026-09-27 (independent): agrees weak. Proven: the exact action slice of the demote rows (Cancel Adver_Timer, Active_Adver_Interval and Active_Down_Interval recomputed from 4000 ms via mdDur(cfg,4000), Backup) and losing rows that stay Active; the IPv4 local operand is the wire source. Missing: the IPv6 operand through the engine, and the 'unsigned integer comparison in network byte order' clause: every compared pair differs only in an octet below 128, so a signed-byte compare would stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L137) | unit/verify | unproven |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L135) | unit/verify | unproven |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L76) | unit/verify | revert, verified |

### [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12)

else // new Active Router logic + Discard the ADVERTISEMENT + Send an ADVERTISEMENT immediately to assert the {Active} state to the sending VRRP Router and to update any learning bridges with the correct Active VRRP Router path. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. losing lower-priority, smaller-ip and equal-ip rows require exactly one SendAdvert and state Active; the winning row demotes instead

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L141) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L139) | unit/verify | unproven |

### [`RFC9568-7.1-1`](#rfc9568-7.1-1)

* It MUST verify that the VRRP version is 3. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. golden decode accepts version 3; N9 feeds a v2 advert to a v3 group and requires ErrVersion

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L610) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L79) | unit/verify | unproven |

### [`RFC9568-7.1-2`](#rfc9568-7.1-2)

* It MUST verify that the VRRP packet type is 1 (ADVERTISEMENT). (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. golden decode accepts type 1; TestValidationOrder 3<4 requires ErrType for type 2 before the VRID lookup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L153) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81) | unit/verify | unproven |

### [`RFC9568-7.1-3`](#rfc9568-7.1-3)

* It MUST verify that the received packet contains the complete VRRP packet (including fixed fields and the IPvX address). (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged negative (N2) appends an 8-byte trailer, a packet that still CONTAINS the complete VRRP packet, and is rejected by ze's exact-length rule rather than by this one. A packet missing part of its IPvX address list is the violating input; the truncated case in TestValidationOrder 1<2 is untagged and cuts the fixed header, not the address list

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L510) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L82) | unit/verify | unproven |

### [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1)

It MUST verify the VRRP checksum. (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. row quotes Section 7.1 'It MUST verify the VRRP checksum.', which covers both families; the tagged units prove only IPv6 (golden accepts, corrupted v6 checksum -> ErrChecksum). The IPv4 negative exists (TestNegativeReferenceBugs N1b both-sums-fail-rejected) but is not tagged to this row. RA10: for IPv4, Decode dual-accepts the RFC 5798 pseudo-header sum as well as the RFC 9568 Section 5.2.8 message-only sum, so an advert whose RFC 9568 checksum is wrong is accepted when the legacy sum matches (documented interop deviation, docs/features/rfc-status.md RFC 5798 row)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L662) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L105) | unit/verify | unproven |

### [`RFC9568-7.1-4`](#rfc9568-7.1-4)

It MUST verify that the VRID is configured on the receiving interface. (§7.1, erratum 8298)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27 against the quote re-taken from erratum 8298, which leaves only the VRID check at MUST. Decode (packet/validate.go) returns ErrUnknownVRID when the lookup does not resolve the VRID: TestValidationOrder 4<5 is the negative, TestDecodeGoldenV3IPv4 resolves a configured VRID. The clause 'on the receiving interface' is not asserted: instance.lookup compares the VRID with its own instance, and interface scoping rests on the per-instance socket the transport binds, which no tagged unit exercises. An advert for a VRID configured on another interface is never fed to a tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L164) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L84) | unit/verify | unproven |

### [`RFC9568-7.1-5`](#rfc9568-7.1-5)

It MUST verify that the Max Advertise Interval is non zero. (§7.1, erratum 8301)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting an advertisement whose Max Advertise Interval is zero. N6 v3-interval-zero zeroes the 12-bit interval, recomputes the checksum so no other check fires, and fails unless Decode returns ErrIntervalZero. Positive: the golden advert with 100 cs decodes with interval 1000 ms.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L554) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L85) | unit/verify | unproven |

### [`RFC9568-7.1-6`](#rfc9568-7.1-6)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestInstanceRxValidAdvertReachesFSM proves a passing packet reaches the FSM; TestInstanceRxDecodeErrorMapsReason feeds a truncated packet and asserts only that a reason was recorded, not that the FSM saw nothing, so an onPacket that recorded the error and still dispatched would pass

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L526) | unit/verify | unproven |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L546) | unit/verify | unproven |

### [`RFC9568-7.2-1`](#rfc9568-7.2-1)

The following operations MUST be performed when transmitting a VRRP packet: * Fill in the VRRP packet fields with the appropriate Virtual Router configuration state * Compute the VRRP checksum (§7.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. RA10 re-audit: TestEncodeGoldenV3IPv4 pins the v3/IPv4 checksum to the RFC 5798 pseudo-header form (golden 0xDEFB over src 192.0.2.251/dst 224.0.0.18). RFC 9568 Section 5.2.8: 'For the IPv4 address family, the checksum calculation only includes the VRRP message starting with the Version field and ending after the last IPv4 address'. The test asserts the interop deviation FillChecksum makes (checksum.go), not the RFC 9568 checksum, so the 'Compute the VRRP checksum' clause is asserted contrary to this RFC; the field-fill half is proven only for IPv4 (the IPv6 golden encode is untagged)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L108) | unit/verify | unproven |

### [`RFC9568-7.2-2`](#rfc9568-7.2-2)

* Set the source MAC address to the Virtual Router MAC address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged unit in packet_test.go asserts only VirtualMAC's derivation (00-00-5E-00-01/02-{VRID}), not that a transmitted frame carries it as source MAC; the wire proof exists in the untagged TestIntegrationAdvertOnPeerVeth (transport_integration_linux_test.go, L2 src == vmac)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L387) | unit/verify | unproven |

### [`RFC9568-7.2-3`](#rfc9568-7.2-3)

* If the protected address is an IPv4 address, then: - Set the source IPv4 address to the interface's primary IPv4 address * else // IPv6 - Set the source IPv6 address to the interface's link-local IPv6 address (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. IPv4 half enforced: TestSendAdvertUsesParentPrimaryV4Source reads the source from the frame and TestSendAdvertNoPrimaryV4SkipsAndCounts refuses to send with no primary. IPv6 half has only the negative TestSendAdvertNoLinkLocalSkipsAndCounts; no tagged unit asserts the IPv6 source is the link-local (the untagged TestIntegrationAdvertV6OnWire does)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L409) | unit/verify | revert, verified |
| negative | [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L361) | unit/verify | revert, verified |
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L217) | unit/verify | revert, verified |

### [`RFC9568-7.2-4`](#rfc9568-7.2-4)

* Set the IPvX protocol to VRRP * Send the VRRP packet to the VRRP IPvX multicast group (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestSendAdvertV3IPv4HeaderTTLProtoDst asserts protocol 112 and dst 224.0.0.18 for IPv4 only; the IPv6 next header 112 and ff02::12 destination are asserted only by the untagged TestIntegrationAdvertV6OnWire

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L316) | unit/verify | unproven |

### [`RFC9568-7.4-1`](#rfc9568-7.4-1)

The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]. (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-7.4-1, so no unit is bound to it.

### [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1)

The Active Router MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA10 re-audit: TestDataplaneApplyIPv4SetsRecipe asserts the parent's arp_ignore=1/arp_filter=1 (positive), but the row carries no {single-polarity} marker and no tagged unit feeds a violating state: TestDataplaneRestoreOnLastGroup proves the recipe is reverted on last teardown (a lifecycle rule), not that an Active router with the recipe absent or wrong answers ARP from the physical MAC. Negative polarity missing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L140) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L73) | unit/verify | unproven |

### [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2)

* At system boot, when initializing interfaces for VRRP operation, gratuitous ARP messages MUST be delayed until both the IPv4 address and the Virtual Router MAC address are configured. (§8.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestInstanceDelaysAnnounceUntilParentUsable/ipv4 proves nothing is announced before the parent has a usable address and the virtual address is installed before the gratuitous ARP; the Virtual Router MAC half rests on waitDevicePresent (register.go), which no tagged unit exercises, so removing that wait keeps the test green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L763) | unit/verify | unproven |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L762) | unit/verify | unproven |

### [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1)

If Proxy ARP is to be used on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC address in the Proxy ARP message. (§8.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.1.3-1, so no unit is bound to it.

### [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1)

The Active Router MUST NOT respond with its physical MAC address. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestVRRPOwnerAnswersWithVirtualMACOnly (integration && linux, QEMU) observes the NAs on the wire for the address owner, the one case where the parent also holds the address: the no-filter control carries the physical MAC in the Target Link-Layer Address option, and with ownerNDAdvertTerm applied every NA carries the Virtual Router MAC. For a non-owner the parent never holds the address, and TestInstanceIPv6VIPLivesOnVirtualMACDevice asserts it is installed on the virtual-MAC macvlan only RA-VRRP strict re-read 2026-09-27 (independent): DOWNGRADED enforced -> weak. The forbidden behaviour for the owner is the owner's Active router letting the parent answer an NS with its physical MAC. TestVRRPOwnerAnswersWithVirtualMACOnly proves ownerNDAdvertTerm drops that NA (QEMU DF-VRRP-3 qemu.log: green, and rednd FAIL 'filtered ND: physical MAC ... answered = true' with Drop->Accept), but it installs the filter by calling setOwnerFilter directly; nothing tagged runs doInstallVIPs -> deps.setOwnerFilter, so removing that call (the product's only route to the filter) leaves every tagged unit green. That wiring is proven only by the untagged TestOwnerFilterInstalledBeforeTheAddressAndWithdrawnAfterIt. TestInstanceIPv6VIPLivesOnVirtualMACDevice asserts the install device for an owner (IsOwner forced) only; no non-owner Active install is asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L717) | unit/verify | unproven |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L47) | unit/verify | revert, verified |

### [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2)

When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link- layer address option in the Neighbor Solicitation message. (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.2-2, so no unit is bound to it.

### [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3)

It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.2-3, so no unit is bound to it.

### [`RFC9568-8.2.2-4`](#rfc9568-8.2.2-4)

* At system boot, when initializing interfaces for VRRP operation, all ND Router Advertisements, ND Neighbor Advertisements, and ND Neighbor Solicitation messages MUST be delayed until both the IPv6 address and the Virtual Router MAC address are configured. (§8.2.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestInstanceDelaysAnnounceUntilParentUsable/ipv6 proves no NA before the parent has a usable address and install-before-announce; the Virtual Router MAC half rests on waitDevicePresent (register.go), untested by the tagged unit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L766) | unit/verify | unproven |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L764) | unit/verify | unproven |

### [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1)

The Backup Routers MUST be configured to send the same Router Advertisement options as the address owner. (§8.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.3-1, so no unit is bound to it.

### [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1)

When a Virtual Router is configured this way and is the Active Router, it MUST send both types at the configured rate, even if it is sub-second. (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-1, so no unit is bound to it.

### [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2)

When a Virtual Router is configured this way and is the Backup Router, it MUST time out based on the rate advertised by the Active Router. (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-2, so no unit is bound to it.

### [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3)

In the case of a VRRPv2 Active Router, this means it MUST translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-3, so no unit is bound to it.

### [`RFC9568-7.1-12`](#rfc9568-7.1-12)

It SHOULD verify that the local router is not the IPvX address owner (Priority = 255 (decimal)) and log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1, erratum 8298)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause by clause, TestInstanceV3OwnerConflictLoggedAndProcessed (instance.go noteOwnerConflict, called from onPacket). SHOULD verify the local router is not the owner: a v3 owner (IsOwner, Priority 255) receiving a Priority-255 advert records packet.ReasonOwnerConflict, while a v3 non-owner receiving the same advert records nothing, so an owner check that never fires or always fires goes red. Log the event: the owner's first advert writes the owner-conflict log line and the non-owner writes none. Subject to rate-limiting: a second advert 30 s later adds no line and a third 61 s after the first adds one, so an unlimited or never-repeating log goes red. The erratum's correction, keep processing: every owner advert still reaches the FSM as AdvertReceived. MAY indicate via network management: permissive; the counter is asserted at the recordRxError seam only, not on the show/telemetry surface. Discrimination: revert of noteOwnerConflict (positive) and of onPacket (negative) both red. DF-VRRP-5 re-read 2026-09-27: onPacket's only later edit is the syncSourceLocked call after the owner check, which neither counts, logs nor drops, so each clause above keeps its red. RA-VRRP strict re-read 2026-09-27 (independent): agrees enforced. SHOULD verify: owner counts ReasonOwnerConflict on each of three adverts, non-owner counts nothing. Log subject to rate-limiting: log lines 1, 1, 2 at t=0, 30 s, 61 s, non-owner none. Erratum's keep-processing: drainAdvertReceived true for every owner advert. MAY indicate via network management is permissive and binds nothing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_conflict_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_conflict_test.go#L65) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9568.txt |
| Source fingerprint | 98495f2abdfc1a49 |
| Record | rfc/extraction/rfc9568.json |
| Mapped sentences | 46 |
| Declined as scope | 17 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `1.5` | not stated | 0 | walked | not stated |
| `1.6` | not stated | 0 | walked | not stated |
| `1.7` | not stated | 0 | walked | not stated |
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
| `5.2.5` | not stated | 1 | walked | not stated |
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
| `7.1` | not stated | 9 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 1 | walked | not stated |
| `8.1.2` | not stated | 4 | walked | not stated |
| `8.1.3` | not stated | 1 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.2.1` | not stated | 0 | walked | not stated |
| `8.2.2` | not stated | 5 | walked | not stated |
| `8.2.3` | not stated | 1 | walked | not stated |
| `8.2.4` | not stated | 1 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.3.1` | not stated | 0 | walked | not stated |
| `8.3.2` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.4.1` | not stated | 0 | walked | not stated |
| `8.4.2` | not stated | 3 | walked | not stated |
| `8.4.2.1` | not stated | 0 | walked | not stated |
| `8.4.2.1.1` | not stated | 0 | walked | not stated |
| `8.4.2.1.2` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Revised BSD License for extracted Code Components; it states no protocol obligation. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Table of contents entry for Section 2.1 ("Required Features"); a heading, not a sentence. | Required Features 2.1. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section heading ("Required Features"); a heading, not a sentence. | Required Features |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Deployment coordination statement with a lowercase "must", not the RFC 2119 keyword: it describes what operators arrange across a LAN, and no VRRP Router behavior follows from it. | The mapping between the VRID and its IPvX address(es) must be coordinated among all VRRP Routers on a LAN. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Worked example prose explaining Figure 3 with a lowercase "must"; it states what the example's configuration contains, not an obligation on an implementation. | In order to back up IPvX B, a second Virtual Router must be configured. |
| `6.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 5.2.9 obligation that the first advertised address is the Virtual Router's IPv6 link-local address; RFC9568-5.2.9-1 already carries it and cites both sections. | The first address MUST be the Link-Local address associated with the Virtual Router. |
| `6.4.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Lead-in to the bulleted {Backup} state list; the obligations it introduces are sites 6.4.2:2 to 6.4.2:6 and rows RFC9568-6.4.2-1 through RFC9568-6.4.2-10. | While in the {Backup} state, a VRRP Router MUST do the following: |
| `6.4.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Lead-in to the bulleted {Active} state list; the obligations it introduces are sites 6.4.3:2 to 6.4.3:9 and rows RFC9568-6.4.3-1 through RFC9568-6.4.3-12. | While in the {Active} state, a VRRP Router MUST do the following: |
| `6.4.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 6.1 obligation not to drop IPv6 Neighbor Solicitations and Advertisements; RFC9568-6.1-1 already carries it and cites both sections. | o It MUST NOT drop IPv6 Neighbor Solicitations and Neighbor Advertisements. |
| `7.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Lead-in to the receive-check list (lowercase "must"); the checks are sites 7.1:2 to 7.1:9. | The following functions must be performed when a VRRP packet is received: |
| `7.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 5.1.1.3 TTL-255 receive check; RFC9568-5.1.1.3-2 already carries it and cites both sections. | - It MUST verify that the IPv4 TTL is 255. |
| `7.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 5.1.2.3 Hop-Limit-255 receive check; RFC9568-5.1.2.3-2 already carries it and cites both sections. | - It MUST verify that the IPv6 Hop Limit is 255. |
| `8.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose with a lowercase "must" about how an Active Router picks an ICMP redirect source address; the normative redirect guidance is the SHOULD at Section 8.2.1 (RFC9568-8.2.1-1). | If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it must determine to which Virtual Router the packet was sent when selecting the redirect source address. |
| `8.1.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 6.4.3 Active-state obligation to answer ARP for a virtual IPv4 address with the Virtual Router MAC; RFC9568-6.4.3-1 already carries it and cites both sections. | When a host sends an ARP request for one of the Virtual Router IPv4 addresses, the Active Router MUST respond to the ARP request with an ARP response that indicates the Virtual Router MAC address for the Virtual Router. |
| `8.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 6.4.3 Active-state obligation to answer a Neighbor Solicitation for a virtual IPv6 address with the Virtual Router MAC; RFC9568-6.4.3-3 already carries it and cites both sections. | When a host sends an ND Neighbor Solicitation message for a Virtual Router IPv6 address, the Active Router MUST respond to the ND Neighbor Solicitation message with the Virtual Router MAC address for the Virtual Router. |
| `8.2.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose with a lowercase "may" noting that extra configuration can be needed; the normative statement is the SHOULD at RFC9568-8.2.4-1. | Additional configuration may be required in order for Unsolicited Neighbor Advertisements to update the corresponding neighbor cache. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Statement that confidentiality is NOT needed, with a lowercase "must"; it removes an obligation rather than stating one. | Confidentiality is not necessary for the correct operation of VRRP, and there is no information in the VRRP messages that must be kept secret from other nodes on the LAN. |

## Superseded

No document obsoletes RFC 9568, so its obligations are stated where they were written.
