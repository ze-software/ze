# RFC 9568 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

Partial. Every requirement this repository extracted from RFC 9568, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 79.2% | 38 of 48 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 8 of 48 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 48 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 91 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 85 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 4.2% | 2 of 48 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 48 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 85 |
| Gated MUST-level | 59 |
| Obligations that bind Ze | 48 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 91 |
| Tagged units | 91 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Positive and negative tests | 38 | one part of the gated population |
| Annotated instead of tested | 21 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (38):** [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2), [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2), [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1), [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1), [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2), [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1), [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1), [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1), [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2), [`RFC9568-6.1-1`](#rfc9568-6.1-1), [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1), [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2), [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4), [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5), [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6), [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7), [`RFC9568-6.4.2-8`](#rfc9568-6.4.2-8), [`RFC9568-6.4.2-9`](#rfc9568-6.4.2-9), [`RFC9568-6.4.2-10`](#rfc9568-6.4.2-10), [`RFC9568-6.4.3-1`](#rfc9568-6.4.3-1), [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5), [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6), [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7), [`RFC9568-6.4.3-8`](#rfc9568-6.4.3-8), [`RFC9568-6.4.3-9`](#rfc9568-6.4.3-9), [`RFC9568-6.4.3-10`](#rfc9568-6.4.3-10), [`RFC9568-6.4.3-11`](#rfc9568-6.4.3-11), [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12), [`RFC9568-7.1-1`](#rfc9568-7.1-1), [`RFC9568-7.1-2`](#rfc9568-7.1-2), [`RFC9568-7.1-3`](#rfc9568-7.1-3), [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1), [`RFC9568-7.1-4`](#rfc9568-7.1-4), [`RFC9568-7.1-5`](#rfc9568-7.1-5), [`RFC9568-7.1-6`](#rfc9568-7.1-6), [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1), [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2), [`RFC9568-8.2.2-4`](#rfc9568-8.2.2-4)

**Annotated instead of tested (21):** [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1), [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1), [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1), [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1), [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3), [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2), [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3), [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4), [`RFC9568-7.2-1`](#rfc9568-7.2-1), [`RFC9568-7.2-2`](#rfc9568-7.2-2), [`RFC9568-7.2-3`](#rfc9568-7.2-3), [`RFC9568-7.2-4`](#rfc9568-7.2-4), [`RFC9568-7.4-1`](#rfc9568-7.4-1), [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1), [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1), [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2), [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3), [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1), [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1), [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2), [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9568-5.1.1.2-1` | Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2) | MUST NOT | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin forwards no IP datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine and re-emits nothing, and the tx socket scopes adverts to the link-local group with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| `RFC9568-5.1.1.3-1` | Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L299). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally writes TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the receive-side TTL!=255 discard is the separate RFC9568-5.1.1.3-2 |
| `RFC9568-5.1.1.3-2` | Discard received IPv4 VRRP packets whose TTL is not equal to 255 (§5.1.1.3, §7.1) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L527) |
| `RFC9568-5.1.2.2-1` | Never forward a datagram destined to ff02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2) | MUST NOT | 5.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the VRRP plugin forwards no IPv6 datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine, and the v6 tx socket sets IPV6_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:193 |
| `RFC9568-5.1.2.3-1` | Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L241). **negative:** no negative test. **{single-polarity}:** the hop limit is a socket option, IPV6_MULTICAST_HOPS 255 set once at openV6 internal/plugins/vrrp/transport/backend_linux.go:185, so every advertisement on that socket carries 255 and no input produces another value -- the receive-side hop-limit discard is the separate RFC9568-5.1.2.3-2. EVIDENCE DISCLOSURE: the sole positive test is internal/plugins/vrrp/transport/transport_integration_linux_test.go:239, which does NOT run in the ordinary unit suite -- the file is gated //go:build integration && linux and its setupLab (internal/plugins/vrrp/transport/transport_integration_linux_test.go:86) skips without CAP_NET_RAW (skipNoRaw, internal/plugins/vrrp/transport/transport_integration_linux_test.go:76-80) and without CAP_NET_ADMIN (internal/plugins/vrrp/transport/transport_integration_linux_test.go:96, :102, :120). It runs under the privileged QEMU integration suite; on an unprivileged host it skips and this requirement is proven by nothing. The evidence itself is genuine: the test getsockopt reads IPV6_MULTICAST_HOPS back from both the tx and the NA socket and requires 255, so the kernel accepted the value |
| `RFC9568-5.1.2.3-2` | Discard received IPv6 VRRP packets whose Hop Limit is not equal to 255 (§5.1.2.3, §7.1) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L99). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L653) |
| `RFC9568-5.2.2-1` | Discard packets with unknown Type; only 1 = ADVERTISEMENT is defined (§5.2.2, §7.1) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L146) |
| `RFC9568-5.2.4-1` | Use Priority 255 for the VRRP Router that owns the Virtual Router's IPvX address(es) (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L766). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L767) |
| `RFC9568-5.2.4-2` | Use Priority values 1-254 for VRRP Routers backing up a Virtual Router (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L318) |
| `RFC9568-5.2.5-1` | Ignore a received VRRP advertisement whose IPvX Addr Count is 0 (§5.2.5, erratum 8299) | MUST | 5.2.5 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L76). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L536) |
| `RFC9568-5.2.6-1` | Set the Reserve field to zero on transmission and ignore it on reception (§5.2.6) | MUST | 5.2.6 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L106). **negative:** `unit/verify` [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L629) |
| `RFC9568-5.2.9-1` | Send the IPv6 link-local address associated with the Virtual Router as the first address in the list (§5.2.9, §6.1, erratum 8300) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L101). **positive:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L520). **negative:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L521). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L254) |
| `RFC9568-5.2.9-2` | Advertise addresses whose family is the same as the VRRP packet's IPvX header address family (§5.2.9) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L542). **negative:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L543) |
| `RFC9568-6.1-1` | Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109). **negative:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L110) |
| `RFC9568-6.4.2-1` | Backup: never respond to ARP requests for the IPv4 address(es) associated with the Virtual Router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L321). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L361) |
| `RFC9568-6.4.2-2` | Backup: never respond to ND Neighbor Solicitations for the IPv6 address(es) associated with the Virtual Router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L635). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L636) |
| `RFC9568-6.4.2-3` | Backup: never send ND Router Advertisements for the Virtual Router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no ND Router Advertisement in any state -- the only ICMPv6 message the plugin builds is the unsolicited Neighbor Advertisement, type 136 at internal/plugins/vrrp/transport/na.go:25,58, and a grep for a type-134 or router-advertisement builder over internal/plugins/vrrp returns nothing, so a Backup has no path that could send one. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| `RFC9568-6.4.2-4` | Backup: discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L322). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L362) |
| `RFC9568-6.4.2-5` | Backup: never accept packets addressed to the IPvX address(es) associated with the Virtual Router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L323). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L363) |
| `RFC9568-6.4.2-6` | Backup: on Shutdown, cancel the Active_Down_Timer and transition to Initialize (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110) |
| `RFC9568-6.4.2-7` | Backup: when the Active_Down_Timer fires, send an ADVERTISEMENT, send gratuitous ARP per IPv4 address (erratum 7949: containing the IPv4 address, target link-layer = Virtual Router MAC) or join the Solicited-Node group and send unsolicited NA (R set, S clear, O set) per IPv6 address, set Adver_Timer to Advertisement_Interval, transition to Active (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L430). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L620) |
| `RFC9568-6.4.2-8` | Backup: on an advertisement with Priority 0, set the Active_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) |
| `RFC9568-6.4.2-9` | Backup: on a non-zero-priority advertisement, if Preempt_Mode is False or the advertised Priority >= local Priority, adopt the Max Advertise Interval as Active_Adver_Interval, recompute Skew_Time and Active_Down_Interval, and reset the Active_Down_Timer (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L113). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114) |
| `RFC9568-6.4.2-10` | Backup: on a non-zero-priority advertisement with Preempt_Mode True and advertised Priority < local Priority, discard the advertisement (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L115). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) |
| `RFC9568-6.4.3-1` | Active: respond to ARP requests for the Virtual Router IPv4 address(es), answering with the Virtual Router MAC address (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L63). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L122) |
| `RFC9568-6.4.3-2` | Active (IPv6): be a member of the Solicited-Node multicast address for the Virtual Router IPv6 address(es) (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L637). **negative:** no negative test. **{single-polarity}:** the membership is the kernel's automatic consequence of the address install ze performs on promotion (doInstallVIPs internal/plugins/vrrp/instance.go:369 registers the VIP on the virtual-MAC macvlan), so there is no ze input that installs the address yet skips the join, and the not-a-member case is the Backup requirement RFC9568-6.4.2-2 |
| `RFC9568-6.4.3-3` | Active (IPv6): respond to ND Neighbor Solicitations, with the Router Flag set, for the Virtual Router IPv6 address(es), answering with the Virtual Router MAC address (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze answers the solicitation from the virtual-MAC macvlan that holds the VIP (doInstallVIPs internal/plugins/vrrp/instance.go:369), but nothing makes that reply carry the Router flag: the only R-bit producer in the plugin is the UNSOLICITED announcement builder, naFlags internal/plugins/vrrp/transport/na.go:30 used by BuildNA na.go:64, and the solicited reply is left to the kernel while applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go:125 sets only accept_dad on the IPv6 macvlan and never a forwarding knob, so the R flag depends on host state ze does not manage |
| `RFC9568-6.4.3-4` | Active (IPv6): send ND Router Advertisements for the Virtual Router (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| `RFC9568-6.4.3-5` | Active: forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L359). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L324) |
| `RFC9568-6.4.3-6` | Active: accept packets addressed to the Virtual Router IPvX address(es) if the IPvX address owner or if Accept_Mode is True (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L261). **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L237). **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L360). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L202). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L325) |
| `RFC9568-6.4.3-7` | Active: never accept packets addressed to the Virtual Router IPvX address(es) when neither owner nor Accept_Mode True (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L201). **negative:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L262). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L238) |
| `RFC9568-6.4.3-8` | Active: on Shutdown, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L117). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L430). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118) |
| `RFC9568-6.4.3-9` | Active: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L119). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L621) |
| `RFC9568-6.4.3-10` | Active: on an advertisement with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L121) |
| `RFC9568-6.4.3-11` | Active: on an advertisement with higher Priority, or equal Priority and greater sender primary IPvX address (unsigned integer comparison in network byte order), cancel the Adver_Timer, adopt Active_Adver_Interval, recompute Skew_Time and Active_Down_Interval, set the Active_Down_Timer, and transition to Backup (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L123) |
| `RFC9568-6.4.3-12` | Active: on a losing advertisement (lower Priority, or equal Priority with smaller sender address), discard it and send an ADVERTISEMENT immediately to assert the Active state and refresh learning bridges (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L125) |
| `RFC9568-7.1-1` | Rx: verify the VRRP version is 3 (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L602) |
| `RFC9568-7.1-2` | Rx: verify the VRRP packet type is 1 (ADVERTISEMENT) (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L147) |
| `RFC9568-7.1-3` | Rx: verify the received packet contains the complete VRRP packet, including fixed fields and the IPvX address list (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L79). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L504) |
| `RFC9568-5.2.8-1` | Rx: verify the VRRP checksum (IPv6: including the pseudo-header) (§5.2.8, §7.1) | MUST | 5.2.8 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L100). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L652) |
| `RFC9568-7.1-4` | Rx: verify the VRID is configured on the receiving interface; as published also that the local router is not the IPvX address owner (erratum 8298 downgrades the owner check to SHOULD-verify-and-log) (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L158) |
| `RFC9568-7.1-5` | Rx: verify the Max Advertise Interval is non-zero (§7.1, erratum 8301) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L546) |
| `RFC9568-7.1-6` | Rx: discard the packet if any mandatory receive check fails (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L473). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L454) |
| `RFC9568-7.2-1` | Tx: fill in the VRRP fields from the Virtual Router configuration state and compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107). **negative:** no negative test. **{single-polarity}:** golden encodes pin every field and the checksum for v3 over IPv4 and IPv6 -- WriteTo internal/plugins/vrrp/packet/packet.go:251 plus FillChecksum internal/plugins/vrrp/packet/checksum.go:86 -- while rejecting a corrupted encoding is the separate receive requirement RFC9568-7.1-3 |
| `RFC9568-7.2-2` | Tx: set the source MAC address to the Virtual Router MAC address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L384). **negative:** no negative test. **{single-polarity}:** the source MAC is the derived Virtual Router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97, egressed by binding the tx socket to that vMAC macvlan (internal/plugins/vrrp/transport/backend_linux.go:133 for IPv4, :179 for IPv6), a deterministic derivation with no input that yields another MAC |
| `RFC9568-7.2-3` | Tx: set the source IP to the interface's primary IPv4 address, or its link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L203). **negative:** no negative test. **{single-polarity}:** the source is a deterministic selection re-resolved on address change -- resolveParentPrimaryV4 internal/plugins/vrrp/transport/transport.go:573 for IPv4 and macvlanLinkLocal internal/plugins/vrrp/transport/backend_linux.go:466 for IPv6 -- with no input that produces a wrong-source advert: an unresolved v6 link-local sends nothing and counts no-link-local instead |
| `RFC9568-7.2-4` | Tx: set the IPvX protocol to VRRP (112) and send to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L300). **negative:** no negative test. **{single-polarity}:** buildIPv4Header writes protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 / ff02::12 at internal/plugins/vrrp/transport/backend_linux.go:242,256, all constants with no input that changes them |
| `RFC9568-7.4-1` | Never use the Virtual Router MAC as the Net_Iface parameter in RFC 7217 / RFC 8981 interface identifier derivation (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze derives no interface identifiers -- a grep for 7217, 8981, addr_gen_mode or stable-privacy over internal/plugins/vrrp returns nothing, and the only IPv6 knob the plugin writes on a virtual-MAC device is accept_dad (internal/plugins/vrrp/dataplane_linux.go:135), so no ze code feeds the Virtual Router MAC into an IID derivation |
| `RFC9568-8.1.2-1` | Active: never answer ARP requests for virtual addresses with the physical MAC address (§8.1.2) | MUST NOT | 8.1.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L64). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L123) |
| `RFC9568-8.1.2-2` | Delay gratuitous ARP at system boot until both the IPv4 address and the Virtual Router MAC address are configured (§8.1.2) | MUST | 8.1.2 | **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L678). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L679) |
| `RFC9568-8.1.3-1` | Advertise the Virtual Router MAC address in Proxy ARP messages for VRRP-protected addresses (§8.1.3) | MUST | 8.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no proxy ARP for virtual addresses -- a grep for proxy_arp over internal/plugins/vrrp returns nothing, and the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64,73) |
| `RFC9568-8.2.2-1` | Active: never answer ND Neighbor Solicitations for virtual addresses with the physical MAC address (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L638). **negative:** no negative test. **{single-polarity}:** the virtual address is installed only on the virtual-MAC macvlan (doInstallVIPs internal/plugins/vrrp/instance.go:369 passing in.dev, the macvlan created with the vMAC at internal/plugins/vrrp/register.go:329-341) and never on the parent that carries the physical MAC, so no input yields a physical-MAC answer; answering nothing at all is the Backup requirement RFC9568-6.4.2-2 |
| `RFC9568-8.2.2-2` | Active: include the Virtual Router MAC in the source link-layer address option of Neighbor Solicitations it sends for hosts (when the option is present) (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56, and no type-135 builder or NS send path exists anywhere under internal/plugins/vrrp |
| `RFC9568-8.2.2-3` | Active: never use the physical MAC address in that source link-layer address option (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no Neighbor Solicitation, so no source link-layer address option is authored by the plugin -- its only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56 |
| `RFC9568-8.2.2-4` | Delay all ND Router Advertisements, Neighbor Advertisements, and Neighbor Solicitations at boot until both the IPv6 address and the Virtual Router MAC address are configured (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L680). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L681) |
| `RFC9568-8.2.3-1` | Configure Backup Routers to send the same Router Advertisement options as the address owner (§8.2.3) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this governs the Router Advertisement option set, and ze offers no RA surface for a Virtual Router at all -- the vrrp YANG group has no RA leaf (applyGroupLeaves internal/plugins/vrrp/groups.go:355 accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only) and the plugin builds no RA. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| `RFC9568-8.4.2-1` | Interop mode, Active: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even sub-second (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no VRRPv2/VRRPv3 dual mode -- a group runs exactly one version (parseVersion internal/plugins/vrrp/groups.go:429 admits 2 or 3, doSendAdvert internal/plugins/vrrp/instance.go:350 encodes that single spec.Version), and the receive ladder discards any advert whose wire version differs from the group's at internal/plugins/vrrp/packet/validate.go:149, so no dual-send path exists |
| `RFC9568-8.4.2-2` | Interop mode, Backup: time out based on the rate advertised by the Active Router (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this requirement is scoped to the Section 8.4.2 interop mode, which ze does not implement -- one version per group (parseVersion internal/plugins/vrrp/groups.go:429) and a version-mismatch discard at internal/plugins/vrrp/packet/validate.go:149 mean a VRRPv2 Active Router is never heard by a v3 group at all. Native v3 interval adoption is the separate RFC9568-6.4.2-9 |
| `RFC9568-8.4.2-3` | Interop mode, Backup: translate a VRRPv2 Active Router's advertised interval from seconds to centiseconds (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** with no interop mode, a v3 group never accepts a VRRPv2 advert to translate -- the receive ladder discards a wire version that differs from the configured group version at internal/plugins/vrrp/packet/validate.go:149; the seconds/centiseconds conversion helpers exist per version (v2SecondsToMS and v3CentisecondsToMS internal/plugins/vrrp/packet/packet.go:233,240) and are never combined in one group |
| `RFC9568-2.5-1` | Log (rate-limited) when queueing delays cause the Active Router to observe self-induced flapping at small Advertisement_Intervals (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-7` | Log the event (rate-limited) when a mandatory receive check fails (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-8` | Verify the received Max Advertise Interval matches the configured Advertisement_Interval, without dropping on mismatch (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-9` | Log (rate-limited) on Max Advertise Interval or address-list mismatch (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-3` | After restart/boot, never send ARP messages using the physical MAC for owned IPv4 virtual addresses (§8.1.2) | SHOULD NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-4` | Broadcast a gratuitous ARP with the Virtual Router MAC for each IPv4 address when configuring an interface (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-5` | Use an IPv4 address known to belong to a specific router for direct access (e.g., SSH) (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.1-1` | Set the IPv6 source of ICMPv6 redirects to the address the end-host used for its next-hop decision (§8.2.1) | SHOULD | 8.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-5` | After restart/boot, never send ND messages with the physical MAC for owned IPv6 virtual addresses (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-6` | Send an unsolicited ND Neighbor Advertisement with the Virtual Router MAC when configuring an interface (§8.2.2) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.3-2` | Never send special-service RA options from the owner unless Backup Routers fully assume the service (§8.2.3) | SHOULD NOT | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.4-1` | Accept Unsolicited Neighbor Advertisements and update the neighbor cache in both Active and Backup states (§8.2.4) | SHOULD | 8.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.1-1` | Never forward packets addressed to virtual IPvX addresses when not the address owner (§8.3.1) | SHOULD NOT | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-1` | Configure at most one VRRP Router per link with priority 255 (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-2` | Log (rate-limited) when multiple routers advertising priority 255 are detected (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-3` | Configure Virtual Routers with sufficiently different priorities to avoid simultaneous Active promotion (§8.3.2) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-4` | Interop mode, Backup: ignore VRRPv2 advertisements from the current Active Router if also receiving VRRPv3 from it (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2.1.1-1` | Never give a VRRPv2 implementation higher priority than an interoperating peer advertising at sub-second rate (§8.4.2.1.1) | SHOULD NOT | 8.4.2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-5` | VRRPv2/VRRPv3 mixed operation as a permanent deployment (§8.4.2) | NOT RECOMMENDED | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-9-1` | Follow the IPv6 link-level security guidelines in Section 2.3 of RFC 9099 (§9) | RECOMMENDED | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-10` | Indicate via network management that a receive error or misconfiguration occurred (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-11` | Verify that IPvX Addr Count and the address list match the addresses configured for the VRID (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.2-7` | The Backup Router may answer Duplicate Address Detection for an owned address while the Active restarts; skipping DAD on the owner is one solution (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.3.2-4` | Log a warning (rate-limited) when multiple routers advertise the same priority (§8.3.2) | MAY | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-6` | Implement a configuration flag to listen for and send both VRRPv2 and VRRPv3 advertisements (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.4.2-7` | Report when a VRRPv3 Active Router is not sending VRRPv2 packets while interop mode is configured (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1) Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin forwards no IP datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine and re-emits nothing, and the tx socket scopes adverts to the link-local group with IP_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:143 |
| [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1) Never forward a datagram destined to ff02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: the VRRP plugin forwards no IPv6 datagram -- instance.onPacket internal/plugins/vrrp/instance.go:453 consumes every received advert into the state machine, and the v6 tx socket sets IPV6_MULTICAST_LOOP 0 at internal/plugins/vrrp/transport/backend_linux.go:193 |
| [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3) Backup: never send ND Router Advertisements for the Virtual Router (§6.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no ND Router Advertisement in any state -- the only ICMPv6 message the plugin builds is the unsolicited Neighbor Advertisement, type 136 at internal/plugins/vrrp/transport/na.go:25,58, and a grep for a type-134 or router-advertisement builder over internal/plugins/vrrp returns nothing, so a Backup has no path that could send one. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3) Active (IPv6): respond to ND Neighbor Solicitations, with the Router Flag set, for the Virtual Router IPv6 address(es), answering with the Virtual Router MAC address (§6.4.3, §8.2.2) | {gap}, no test | ze answers the solicitation from the virtual-MAC macvlan that holds the VIP (doInstallVIPs internal/plugins/vrrp/instance.go:369), but nothing makes that reply carry the Router flag: the only R-bit producer in the plugin is the UNSOLICITED announcement builder, naFlags internal/plugins/vrrp/transport/na.go:30 used by BuildNA na.go:64, and the solicited reply is left to the kernel while applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go:125 sets only accept_dad on the IPv6 macvlan and never a forwarding knob, so the R flag depends on host state ze does not manage |
| [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4) Active (IPv6): send ND Router Advertisements for the Virtual Router (§6.4.3) | {gap}, no test | ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| [`RFC9568-7.4-1`](#rfc9568-7.4-1) Never use the Virtual Router MAC as the Net_Iface parameter in RFC 7217 / RFC 8981 interface identifier derivation (§7.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze derives no interface identifiers -- a grep for 7217, 8981, addr_gen_mode or stable-privacy over internal/plugins/vrrp returns nothing, and the only IPv6 knob the plugin writes on a virtual-MAC device is accept_dad (internal/plugins/vrrp/dataplane_linux.go:135), so no ze code feeds the Virtual Router MAC into an IID derivation |
| [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1) Advertise the Virtual Router MAC address in Proxy ARP messages for VRRP-protected addresses (§8.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no proxy ARP for virtual addresses -- a grep for proxy_arp over internal/plugins/vrrp returns nothing, and the per-group virtual-MAC macvlan answers ARP for the VIP directly (createMacvlan internal/plugins/vrrp/register.go:329 plus the sole-responder sysctl recipe internal/plugins/vrrp/dataplane_linux.go:64,73) |
| [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2) Active: include the Virtual Router MAC in the source link-layer address option of Neighbor Solicitations it sends for hosts (when the option is present) (§8.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56, and no type-135 builder or NS send path exists anywhere under internal/plugins/vrrp |
| [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3) Active: never use the physical MAC address in that source link-layer address option (§8.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no Neighbor Solicitation, so no source link-layer address option is authored by the plugin -- its only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go:25,56 |
| [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1) Configure Backup Routers to send the same Router Advertisement options as the address owner (§8.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: this governs the Router Advertisement option set, and ze offers no RA surface for a Virtual Router at all -- the vrrp YANG group has no RA leaf (applyGroupLeaves internal/plugins/vrrp/groups.go:355 accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only) and the plugin builds no RA. The absent Active-side emission is the gap tracked at RFC9568-6.4.3-4 |
| [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1) Interop mode, Active: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even sub-second (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no VRRPv2/VRRPv3 dual mode -- a group runs exactly one version (parseVersion internal/plugins/vrrp/groups.go:429 admits 2 or 3, doSendAdvert internal/plugins/vrrp/instance.go:350 encodes that single spec.Version), and the receive ladder discards any advert whose wire version differs from the group's at internal/plugins/vrrp/packet/validate.go:149, so no dual-send path exists |
| [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2) Interop mode, Backup: time out based on the rate advertised by the Active Router (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: this requirement is scoped to the Section 8.4.2 interop mode, which ze does not implement -- one version per group (parseVersion internal/plugins/vrrp/groups.go:429) and a version-mismatch discard at internal/plugins/vrrp/packet/validate.go:149 mean a VRRPv2 Active Router is never heard by a v3 group at all. Native v3 interval adoption is the separate RFC9568-6.4.2-9 |
| [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3) Interop mode, Backup: translate a VRRPv2 Active Router's advertised interval from seconds to centiseconds (§8.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: with no interop mode, a v3 group never accepts a VRRPv2 advert to translate -- the receive ladder discards a wire version that differs from the configured group version at internal/plugins/vrrp/packet/validate.go:149; the seconds/centiseconds conversion helpers exist per version (v2SecondsToMS and v3CentisecondsToMS internal/plugins/vrrp/packet/packet.go:233,240) and are never combined in one group |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1)

Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.1.2-1, so no unit is bound to it.

### [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1)

Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L299) | unit/verify | unproven |

### [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2)

Discard received IPv4 VRRP packets whose TTL is not equal to 255 (§5.1.1.3, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L527) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74) | unit/verify | unproven |

### [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1)

Never forward a datagram destined to ff02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.2.2-1, so no unit is bound to it.

### [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1)

Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L241) | unit/verify | unproven |

### [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2)

Discard received IPv6 VRRP packets whose Hop Limit is not equal to 255 (§5.1.2.3, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L653) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L99) | unit/verify | unproven |

### [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1)

Discard packets with unknown Type; only 1 = ADVERTISEMENT is defined (§5.2.2, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L146) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75) | unit/verify | unproven |

### [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1)

Use Priority 255 for the VRRP Router that owns the Virtual Router's IPvX address(es) (§5.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L767) | unit/verify | unproven |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L766) | unit/verify | unproven |

### [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2)

Use Priority values 1-254 for VRRP Routers backing up a Virtual Router (§5.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L318) | unit/verify | unproven |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L317) | unit/verify | unproven |

### [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1)

Ignore a received VRRP advertisement whose IPvX Addr Count is 0 (§5.2.5, erratum 8299)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L536) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L76) | unit/verify | unproven |

### [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1)

Set the Reserve field to zero on transmission and ignore it on reception (§5.2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L629) | unit/verify | unproven |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L106) | unit/verify | unproven |

### [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1)

Send the IPv6 link-local address associated with the Virtual Router as the first address in the list (§5.2.9, §6.1, erratum 8300)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L521) | unit/verify | unproven |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L254) | unit/verify | unproven |
| positive | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L520) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L101) | unit/verify | unproven |

### [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2)

Advertise addresses whose family is the same as the VRRP packet's IPvX header address family (§5.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L543) | unit/verify | unproven |
| positive | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L542) | unit/verify | unproven |

### [`RFC9568-6.1-1`](#rfc9568-6.1-1)

Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L110) | unit/verify | unproven |
| positive | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1)

Backup: never respond to ARP requests for the IPv4 address(es) associated with the Virtual Router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L361) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L321) | unit/verify | unproven |

### [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2)

Backup: never respond to ND Neighbor Solicitations for the IPv6 address(es) associated with the Virtual Router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L636) | unit/verify | unproven |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L635) | unit/verify | unproven |

### [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3)

Backup: never send ND Router Advertisements for the Virtual Router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.2-3, so no unit is bound to it.

### [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4)

Backup: discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L362) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L322) | unit/verify | unproven |

### [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5)

Backup: never accept packets addressed to the IPvX address(es) associated with the Virtual Router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L363) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L323) | unit/verify | unproven |

### [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6)

Backup: on Shutdown, cancel the Active_Down_Timer and transition to Initialize (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7)

Backup: when the Active_Down_Timer fires, send an ADVERTISEMENT, send gratuitous ARP per IPv4 address (erratum 7949: containing the IPv4 address, target link-layer = Virtual Router MAC) or join the Solicited-Node group and send unsolicited NA (R set, S clear, O set) per IPv6 address, set Adver_Timer to Advertisement_Interval, transition to Active (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L620) | unit/verify | unproven |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L430) | unit/verify | unproven |

### [`RFC9568-6.4.2-8`](#rfc9568-6.4.2-8)

Backup: on an advertisement with Priority 0, set the Active_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111) | unit/verify | unproven |

### [`RFC9568-6.4.2-9`](#rfc9568-6.4.2-9)

Backup: on a non-zero-priority advertisement, if Preempt_Mode is False or the advertised Priority >= local Priority, adopt the Max Advertise Interval as Active_Adver_Interval, recompute Skew_Time and Active_Down_Interval, and reset the Active_Down_Timer (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L113) | unit/verify | unproven |

### [`RFC9568-6.4.2-10`](#rfc9568-6.4.2-10)

Backup: on a non-zero-priority advertisement with Preempt_Mode True and advertised Priority < local Priority, discard the advertisement (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L115) | unit/verify | unproven |

### [`RFC9568-6.4.3-1`](#rfc9568-6.4.3-1)

Active: respond to ARP requests for the Virtual Router IPv4 address(es), answering with the Virtual Router MAC address (§6.4.3, §8.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L122) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L63) | unit/verify | unproven |

### [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2)

Active (IPv6): be a member of the Solicited-Node multicast address for the Virtual Router IPv6 address(es) (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L637) | unit/verify | unproven |

### [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3)

Active (IPv6): respond to ND Neighbor Solicitations, with the Router Flag set, for the Virtual Router IPv6 address(es), answering with the Virtual Router MAC address (§6.4.3, §8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.3-3, so no unit is bound to it.

### [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4)

Active (IPv6): send ND Router Advertisements for the Virtual Router (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.3-4, so no unit is bound to it.

### [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5)

Active: forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L324) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L359) | unit/verify | unproven |

### [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6)

Active: accept packets addressed to the Virtual Router IPvX address(es) if the IPvX address owner or if Accept_Mode is True (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L202) | unit/verify | unproven |
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L325) | unit/verify | unproven |
| positive | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L261) | unit/verify | unproven |
| positive | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L237) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L360) | unit/verify | unproven |

### [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7)

Active: never accept packets addressed to the Virtual Router IPvX address(es) when neither owner nor Accept_Mode True (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L262) | unit/verify | unproven |
| negative | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L238) | unit/verify | unproven |
| positive | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L201) | unit/verify | unproven |

### [`RFC9568-6.4.3-8`](#rfc9568-6.4.3-8)

Active: on Shutdown, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L117) | unit/verify | unproven |
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L430) | unit/verify | unproven |

### [`RFC9568-6.4.3-9`](#rfc9568-6.4.3-9)

Active: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L621) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L119) | unit/verify | unproven |

### [`RFC9568-6.4.3-10`](#rfc9568-6.4.3-10)

Active: on an advertisement with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L121) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120) | unit/verify | unproven |

### [`RFC9568-6.4.3-11`](#rfc9568-6.4.3-11)

Active: on an advertisement with higher Priority, or equal Priority and greater sender primary IPvX address (unsigned integer comparison in network byte order), cancel the Adver_Timer, adopt Active_Adver_Interval, recompute Skew_Time and Active_Down_Interval, set the Active_Down_Timer, and transition to Backup (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L123) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122) | unit/verify | unproven |

### [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12)

Active: on a losing advertisement (lower Priority, or equal Priority with smaller sender address), discard it and send an ADVERTISEMENT immediately to assert the Active state and refresh learning bridges (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L125) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124) | unit/verify | unproven |

### [`RFC9568-7.1-1`](#rfc9568-7.1-1)

Rx: verify the VRRP version is 3 (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L602) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77) | unit/verify | unproven |

### [`RFC9568-7.1-2`](#rfc9568-7.1-2)

Rx: verify the VRRP packet type is 1 (ADVERTISEMENT) (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L147) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78) | unit/verify | unproven |

### [`RFC9568-7.1-3`](#rfc9568-7.1-3)

Rx: verify the received packet contains the complete VRRP packet, including fixed fields and the IPvX address list (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L504) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L79) | unit/verify | unproven |

### [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1)

Rx: verify the VRRP checksum (IPv6: including the pseudo-header) (§5.2.8, §7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L652) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L100) | unit/verify | unproven |

### [`RFC9568-7.1-4`](#rfc9568-7.1-4)

Rx: verify the VRID is configured on the receiving interface; as published also that the local router is not the IPvX address owner (erratum 8298 downgrades the owner check to SHOULD-verify-and-log) (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L158) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80) | unit/verify | unproven |

### [`RFC9568-7.1-5`](#rfc9568-7.1-5)

Rx: verify the Max Advertise Interval is non-zero (§7.1, erratum 8301)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L546) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81) | unit/verify | unproven |

### [`RFC9568-7.1-6`](#rfc9568-7.1-6)

Rx: discard the packet if any mandatory receive check fails (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L454) | unit/verify | unproven |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L473) | unit/verify | unproven |

### [`RFC9568-7.2-1`](#rfc9568-7.2-1)

Tx: fill in the VRRP fields from the Virtual Router configuration state and compute the VRRP checksum (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107) | unit/verify | unproven |

### [`RFC9568-7.2-2`](#rfc9568-7.2-2)

Tx: set the source MAC address to the Virtual Router MAC address (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L384) | unit/verify | unproven |

### [`RFC9568-7.2-3`](#rfc9568-7.2-3)

Tx: set the source IP to the interface's primary IPv4 address, or its link-local IPv6 address (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L203) | unit/verify | unproven |

### [`RFC9568-7.2-4`](#rfc9568-7.2-4)

Tx: set the IPvX protocol to VRRP (112) and send to the VRRP IPvX multicast group (§7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L300) | unit/verify | unproven |

### [`RFC9568-7.4-1`](#rfc9568-7.4-1)

Never use the Virtual Router MAC as the Net_Iface parameter in RFC 7217 / RFC 8981 interface identifier derivation (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-7.4-1, so no unit is bound to it.

### [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1)

Active: never answer ARP requests for virtual addresses with the physical MAC address (§8.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L123) | unit/verify | unproven |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L64) | unit/verify | unproven |

### [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2)

Delay gratuitous ARP at system boot until both the IPv4 address and the Virtual Router MAC address are configured (§8.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L679) | unit/verify | unproven |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L678) | unit/verify | unproven |

### [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1)

Advertise the Virtual Router MAC address in Proxy ARP messages for VRRP-protected addresses (§8.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.1.3-1, so no unit is bound to it.

### [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1)

Active: never answer ND Neighbor Solicitations for virtual addresses with the physical MAC address (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L638) | unit/verify | unproven |

### [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2)

Active: include the Virtual Router MAC in the source link-layer address option of Neighbor Solicitations it sends for hosts (when the option is present) (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.2-2, so no unit is bound to it.

### [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3)

Active: never use the physical MAC address in that source link-layer address option (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.2-3, so no unit is bound to it.

### [`RFC9568-8.2.2-4`](#rfc9568-8.2.2-4)

Delay all ND Router Advertisements, Neighbor Advertisements, and Neighbor Solicitations at boot until both the IPv6 address and the Virtual Router MAC address are configured (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L681) | unit/verify | unproven |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L680) | unit/verify | unproven |

### [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1)

Configure Backup Routers to send the same Router Advertisement options as the address owner (§8.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.2.3-1, so no unit is bound to it.

### [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1)

Interop mode, Active: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even sub-second (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-1, so no unit is bound to it.

### [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2)

Interop mode, Backup: time out based on the rate advertised by the Active Router (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-2, so no unit is bound to it.

### [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3)

Interop mode, Backup: translate a VRRPv2 Active Router's advertised interval from seconds to centiseconds (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-8.4.2-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9568, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9568, so its obligations are stated where they were written.
