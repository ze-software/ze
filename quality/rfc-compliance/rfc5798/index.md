# RFC 5798 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

Supported. Every requirement this repository extracted from RFC 5798, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 55 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 55 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 55 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 55 | of 80 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 55 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 55 of 55 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 55 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 80 |
| Gated MUST-level | 55 |
| Obligations that bind Ze | 55 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 55 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5798.md` |
| Requirement shard | `rfc/requirements/rfc5798.md` |
| RFC text | `rfc/full/rfc5798.txt` |

## Enrolment

Enrolled: VRRP Version 3 for IPv4 and IPv6 (RFC 5798, obsoleted by RFC 9568): the VRRPv3 document ze actually speaks on the IPv4 wire. Every one of its 55 gated requirements carries a {superseded} marker naming where the obligation lives in RFC 9568, and 51 of them are restated there unchanged, so ze implements them through the same producers rfc/short/rfc9568.md gates: internal/plugins/vrrp/packet (advert encode/decode, checksum), internal/plugins/vrrp/fsm (Initialize/Backup/Master), internal/plugins/vrrp/transport (proto 112, GTSM, GARP/NA). The row that is NOT a restatement is RFC5798-5.2.8-1: Section 5.2.8 puts a pseudo-header under the checksum for both families, RFC 9568 Section 5.2.8 removes it for IPv4, and ze transmits this document form because keepalived and the pre-RFC-9568 base require it (pseudoSumV4Legacy and FillChecksum, internal/plugins/vrrp/packet/checksum.go). Enrolled 2026-08-31 with no requirement yet tagged under an RFC5798 id: the behaviour is proven under the RFC 9568 ids and the RFC5798 tags are owed.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- For IPv4, ze transmits the RFC 5798 pseudo-header checksum form, because that is what keepalived (proven on the wire: its own adverts use it) and the rest of the deployed base compute and require
- a message-only advert is rejected by them as "Invalid VRRPv3 checksum". On receive, ze dual-accepts both this form and the RFC 9568 message-only form.


**What the ledger says remains**

RFC 9568 Section 5.2.8 clarifies the IPv4 checksum as message-only (no pseudo-header); ze diverges from that clarification on transmit for interoperability, and counts message-only senders (`checksum-rfc9568-message-only`) so the strict-RFC-9568 population is visible. When that population dominates, the transmit form can be revisited.

- **Enrolled 2026-09-01:** [`rfc/short/rfc5798.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5798.md) declares 80 requirements, 55 of them MUST-level, and every one carries a `{superseded}` marker naming where RFC 9568 states it. No test carries an `RFC5798-` tag yet, so the behavior is proven under the RFC 9568 ids and the RFC 5798 ids are an open proof backlog `./le rfc check` names row by row. One RFC 5798 obligation is NOT a restatement: [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) puts a pseudo-header under the checksum for both address families, which is the divergence this row describes. RFC 5798 Section 5.2.8 cites the pseudo-header "as defined in Section 8.1 of [RFC2460]", an IPv6-only shape, so what ze and the deployed base compute for IPv4 is the classic IPv4 pseudo-header (`pseudoSumV4Legacy`, [`internal/plugins/vrrp/packet/checksum.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum.go)) rather than the shape that sentence names.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 55 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **55** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (55):** [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1), [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1), [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2), [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1), [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1), [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2), [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1), [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1), [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2), [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1), [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1), [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1), [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2), [`RFC5798-6.1-1`](#rfc5798-6.1-1), [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1), [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2), [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3), [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4), [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5), [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6), [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7), [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8), [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9), [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10), [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1), [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2), [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3), [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4), [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5), [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6), [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7), [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8), [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9), [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10), [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11), [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12), [`RFC5798-7.1-1`](#rfc5798-7.1-1), [`RFC5798-7.1-2`](#rfc5798-7.1-2), [`RFC5798-7.1-3`](#rfc5798-7.1-3), [`RFC5798-7.1-4`](#rfc5798-7.1-4), [`RFC5798-7.2-1`](#rfc5798-7.2-1), [`RFC5798-7.2-2`](#rfc5798-7.2-2), [`RFC5798-7.2-3`](#rfc5798-7.2-3), [`RFC5798-7.2-4`](#rfc5798-7.2-4), [`RFC5798-7.4-1`](#rfc5798-7.4-1), [`RFC5798-7.4-2`](#rfc5798-7.4-2), [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1), [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1), [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1), [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2), [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3), [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4), [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1), [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1), [`RFC5798-A.2-1`](#rfc5798-a.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5798-5.1.1.2-1` | Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2) | MUST NOT | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.1.1.3-1` | Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.1.1.3-2` | Discard a received IPv4 VRRP packet whose TTL is not 255 (§5.1.1.3, §7.1) | MUST | 5.1.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.1.2.2-1` | Never forward a datagram destined to FF02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2) | MUST NOT | 5.1.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.1.2.3-1` | Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.1.2.3-2` | Discard a received IPv6 VRRP packet whose Hop Limit is not 255 (§5.1.2.3, §7.1) | MUST | 5.1.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.2-1` | Discard a packet with unknown Type; 1 = ADVERTISEMENT is the only type defined (§5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.4-1` | Use Priority 255 for the VRRP router that owns the IPvX address associated with the virtual router (§5.2.4) | MUST | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.4-2` | Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.2.4) | MUST | 5.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.6-1` | Set the rsvd field to zero on transmission and ignore it on reception (§5.2.6) | MUST | 5.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.8-1` | Compute and verify the checksum as the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a pseudo-header defined by RFC 2460, with next header 112 and the checksum field zeroed, for both address families (§5.2.8, §7.1) | MUST | 5.2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.9-1` | Send the IPv6 link-local address associated with the virtual router as the first address in the list (§5.2.9, §6.1; lowercase "must" in the RFC) | MUST | 5.2.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-5.2.9-2` | Never carry IPv4 and IPv6 addresses together in one IPvX Address field (§5.2.9) | MUST NOT | 5.2.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.1-1` | Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-1` | Backup: never respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-2` | Backup: never respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-3` | Backup: never send ND Router Advertisement messages for the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-4` | Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-5` | Backup: never accept packets addressed to the IPvX address(es) associated with the virtual router (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-6` | Backup: on a Shutdown event, cancel the Master_Down_Timer and transition to Initialize (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-7` | Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP carrying the virtual router MAC for each IPv4 address or, for IPv6, join the Solicited-Node multicast address and send an unsolicited Neighbor Advertisement for each address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-8` | Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-9` | Backup: on a non-zero-priority ADVERTISEMENT, when Preempt_Mode is False or the advertised Priority is greater than or equal to the local Priority, set Master_Adver_Interval to the Adver Interval in the advertisement, recompute Master_Down_Interval, and reset the Master_Down_Timer to it (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.2-10` | Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and an advertised Priority lower than the local Priority, discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-1` | Master: respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-2` | Master: be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-3` | Master: respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-4` | Master: send ND Router Advertisements for the virtual router (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-5` | Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-6` | Master: accept packets addressed to the IPvX address(es) associated with the virtual router when it is the address owner or when Accept_Mode is True (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-7` | Master: never accept those packets when it is neither the address owner nor configured with Accept_Mode True (§6.4.3) | MUST NOT | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-8` | Master: on a Shutdown event, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-9` | Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-10` | Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-11` | Master: on an ADVERTISEMENT with a higher Priority, or an equal Priority with a greater sender primary IPvX address, cancel the Adver_Timer, set Master_Adver_Interval to the advertised Adver Interval, recompute Skew_Time and Master_Down_Interval, set the Master_Down_Timer, and transition to Backup (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-6.4.3-12` | Master: on a losing ADVERTISEMENT, one with a lower Priority or an equal Priority with a smaller sender address, discard it (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-1` | Rx: verify that the VRRP version is 3 (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-2` | Rx: verify that the received packet contains the complete VRRP packet, the fixed fields and the IPvX address list (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-3` | Rx: verify that the VRID is configured on the receiving interface and that the local router is not the IPvX address owner (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-4` | Rx: discard the packet when any mandatory receive check fails (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.2-1` | Tx: fill in the VRRP packet fields from the virtual router configuration state and compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.2-2` | Tx: set the source MAC address to the virtual router MAC address (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.2-3` | Tx: set the source IPv4 address to the interface primary IPv4 address, or the source IPv6 address to the interface link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.2-4` | Tx: set the IPvX protocol to VRRP and send the packet to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.4-1` | Create the Interface Identifiers of an IPv6 router running VRRP in the normal manner, as in Transmission of IPv6 Packets over Ethernet Networks (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.4-2` | Never use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-1` | Master: never respond to a host ARP request for a virtual router IPv4 address with its physical MAC address (§8.1.2) | MUST NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.3-1` | Advertise the virtual router MAC address in the Proxy ARP message when Proxy ARP is used on a VRRP router (§8.1.3; lowercase "must" in the RFC) | MUST | 8.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-1` | Master: never respond to an ND Neighbor Solicitation for a virtual router IPv6 address with its physical MAC address (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-2` | Master: include the virtual router MAC address in the source link-layer address option of a Neighbor Solicitation it sends for a host's IPv6 address, when it sends that option (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-3` | Master: never use its physical MAC address in that source link-layer address option (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-4` | At system boot, delay every ND Router Advertisement, Neighbor Advertisement and Neighbor Solicitation until both the IPv6 address and the virtual router MAC address are configured (§8.2.2; lowercase "must" in the RFC) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.3-1` | Configure Backup routers to send the same Router Advertisement options as the address owner (§8.2.3; lowercase "must" in the RFC) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-1` | Interop mode, Master: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even when it is sub-second (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-A.2-1` | Token Ring: implement the functional-address mode of operation when supporting VRRP on Token Ring (§A.2) | MUST | A.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-5` | Log the event when a mandatory receive check fails (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-8` | Log the event when the optional address-list check fails (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.1-1` | Set the IPv4 source address of an ICMP redirect to the address the end-host used when making its next-hop routing decision (§8.1.1; lowercase "should" in the RFC) | SHOULD | 8.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-2` | After a restart or boot, send no ARP message using the physical MAC address for an owned virtual IPv4 address (§8.1.2) | SHOULD NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-3` | When configuring an interface, broadcast a gratuitous ARP request carrying the virtual router MAC address for each IPv4 address on that interface (§8.1.2; lowercase "should" in the RFC) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-4` | At system boot, delay gratuitous ARP requests and ARP responses until both the IPv4 address and the virtual router MAC address are configured (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.1.2-5` | Use an IP address known to belong to a particular router when direct access to that router is required, for example ssh (§8.1.2; lowercase "must" inside a SHOULD-level bullet list) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.1-1` | Set the IPv6 source address of an ICMPv6 redirect to the address the end-host used when making its next-hop routing decision (§8.2.1; lowercase "should" in the RFC) | SHOULD | 8.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-5` | After a restart or boot, send no ND message using the physical MAC address for an owned virtual IPv6 address (§8.2.2) | SHOULD NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-6` | When configuring an interface, send an unsolicited ND Neighbor Advertisement carrying the virtual router MAC address for the IPv6 address on that interface (§8.2.2; lowercase "should" in the RFC) | SHOULD | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.3-2` | Send Router Advertisement options that advertise special services from the address owner unless the Backup routers can assume those services in full with a complete and synchronized database (§8.2.3; lowercase "should not" in the RFC) | SHOULD NOT | 8.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.1-1` | Never forward packets addressed to the IPvX address a router becomes Master for when it is not the address owner (§8.3.1) | SHOULD NOT | 8.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.2-1` | Configure no more than one VRRP router on the link with priority 255 for a single VRID (§8.3.2; lowercase in the RFC) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.3.2-2` | Distribute the priority values of multiple Backup routers uniformly to speed convergence (§8.3.2; lowercase in the RFC) | SHOULD | 8.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-2` | Do not run VRRPv2 and VRRPv3 mixed operation as a permanent deployment; it is an upgrade path (§8.4.2) | NOT RECOMMENDED | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-3` | Interop mode, Backup: time out from the rate the Master advertises, translating a VRRPv2 Master's seconds into centiseconds (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-4` | Interop mode, Backup: ignore VRRPv2 advertisements from the current Master when VRRPv3 packets are also being received from it (§8.4.2) | SHOULD | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.3.1-1` | Never give a VRRPv2 implementation a higher priority than the VRRPv2/VRRPv3 implementation it interacts with when that peer advertises at a sub-second rate (§8.4.3.1) | SHOULD NOT | 8.4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-A.1-1` | FDDI: configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device rather than changing its hardware MAC address (§A.1) | SHOULD | A.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-6` | Indicate through network management that a receive error, or a detected misconfiguration, occurred (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-7.1-7` | Verify that "Count IPvX Addrs" and the list of IPvX addresses match the addresses configured for the VRID (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.2.2-7` | Answer Duplicate Address Detection for an owned address from the Backup router while the Master restarts; one solution is not to run DAD in that case (§8.2.2) | MAY | 8.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-5` | Implement a configuration flag that tells the router to listen for and send both VRRPv2 and VRRPv3 advertisements (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-8.4.2-6` | Report when a VRRPv3 Master is not sending VRRPv2 packets while interop mode is configured (§8.4.2) | MAY | 8.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5798-A.2-2` | Token Ring: support the unicast mode of operation beside the functional-address mode (§A.2) | MAY | A.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2) | no test | no test carries this requirement id |
| [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1) Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3) | no test | no test carries this requirement id |
| [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2) Discard a received IPv4 VRRP packet whose TTL is not 255 (§5.1.1.3, §7.1) | no test | no test carries this requirement id |
| [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1) Never forward a datagram destined to FF02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2) | no test | no test carries this requirement id |
| [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1) Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3) | no test | no test carries this requirement id |
| [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2) Discard a received IPv6 VRRP packet whose Hop Limit is not 255 (§5.1.2.3, §7.1) | no test | no test carries this requirement id |
| [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1) Discard a packet with unknown Type; 1 = ADVERTISEMENT is the only type defined (§5.2.2) | no test | no test carries this requirement id |
| [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1) Use Priority 255 for the VRRP router that owns the IPvX address associated with the virtual router (§5.2.4) | no test | no test carries this requirement id |
| [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2) Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.2.4) | no test | no test carries this requirement id |
| [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1) Set the rsvd field to zero on transmission and ignore it on reception (§5.2.6) | no test | no test carries this requirement id |
| [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) Compute and verify the checksum as the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a pseudo-header defined by RFC 2460, with next header 112 and the checksum field zeroed, for both address families (§5.2.8, §7.1) | no test | no test carries this requirement id |
| [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1) Send the IPv6 link-local address associated with the virtual router as the first address in the list (§5.2.9, §6.1; lowercase "must" in the RFC) | no test | no test carries this requirement id |
| [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2) Never carry IPv4 and IPv6 addresses together in one IPvX Address field (§5.2.9) | no test | no test carries this requirement id |
| [`RFC5798-6.1-1`](#rfc5798-6.1-1) Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1) Backup: never respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2) Backup: never respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3) Backup: never send ND Router Advertisement messages for the virtual router (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4) Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5) Backup: never accept packets addressed to the IPvX address(es) associated with the virtual router (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6) Backup: on a Shutdown event, cancel the Master_Down_Timer and transition to Initialize (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7) Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP carrying the virtual router MAC for each IPv4 address or, for IPv6, join the Solicited-Node multicast address and send an unsolicited Neighbor Advertisement for each address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8) Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9) Backup: on a non-zero-priority ADVERTISEMENT, when Preempt_Mode is False or the advertised Priority is greater than or equal to the local Priority, set Master_Adver_Interval to the Adver Interval in the advertisement, recompute Master_Down_Interval, and reset the Master_Down_Timer to it (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10) Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and an advertised Priority lower than the local Priority, discard the ADVERTISEMENT (§6.4.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1) Master: respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.3, §8.1.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2) Master: be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3) Master: respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.3, §8.2.2) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) Master: send ND Router Advertisements for the virtual router (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5) Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6) Master: accept packets addressed to the IPvX address(es) associated with the virtual router when it is the address owner or when Accept_Mode is True (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7) Master: never accept those packets when it is neither the address owner nor configured with Accept_Mode True (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8) Master: on a Shutdown event, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9) Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10) Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11) Master: on an ADVERTISEMENT with a higher Priority, or an equal Priority with a greater sender primary IPvX address, cancel the Adver_Timer, set Master_Adver_Interval to the advertised Adver Interval, recompute Skew_Time and Master_Down_Interval, set the Master_Down_Timer, and transition to Backup (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12) Master: on a losing ADVERTISEMENT, one with a lower Priority or an equal Priority with a smaller sender address, discard it (§6.4.3) | no test | no test carries this requirement id |
| [`RFC5798-7.1-1`](#rfc5798-7.1-1) Rx: verify that the VRRP version is 3 (§7.1) | no test | no test carries this requirement id |
| [`RFC5798-7.1-2`](#rfc5798-7.1-2) Rx: verify that the received packet contains the complete VRRP packet, the fixed fields and the IPvX address list (§7.1) | no test | no test carries this requirement id |
| [`RFC5798-7.1-3`](#rfc5798-7.1-3) Rx: verify that the VRID is configured on the receiving interface and that the local router is not the IPvX address owner (§7.1) | no test | no test carries this requirement id |
| [`RFC5798-7.1-4`](#rfc5798-7.1-4) Rx: discard the packet when any mandatory receive check fails (§7.1) | no test | no test carries this requirement id |
| [`RFC5798-7.2-1`](#rfc5798-7.2-1) Tx: fill in the VRRP packet fields from the virtual router configuration state and compute the VRRP checksum (§7.2) | no test | no test carries this requirement id |
| [`RFC5798-7.2-2`](#rfc5798-7.2-2) Tx: set the source MAC address to the virtual router MAC address (§7.2) | no test | no test carries this requirement id |
| [`RFC5798-7.2-3`](#rfc5798-7.2-3) Tx: set the source IPv4 address to the interface primary IPv4 address, or the source IPv6 address to the interface link-local IPv6 address (§7.2) | no test | no test carries this requirement id |
| [`RFC5798-7.2-4`](#rfc5798-7.2-4) Tx: set the IPvX protocol to VRRP and send the packet to the VRRP IPvX multicast group (§7.2) | no test | no test carries this requirement id |
| [`RFC5798-7.4-1`](#rfc5798-7.4-1) Create the Interface Identifiers of an IPv6 router running VRRP in the normal manner, as in Transmission of IPv6 Packets over Ethernet Networks (§7.4) | no test | no test carries this requirement id |
| [`RFC5798-7.4-2`](#rfc5798-7.4-2) Never use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers (§7.4) | no test | no test carries this requirement id |
| [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1) Master: never respond to a host ARP request for a virtual router IPv4 address with its physical MAC address (§8.1.2) | no test | no test carries this requirement id |
| [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) Advertise the virtual router MAC address in the Proxy ARP message when Proxy ARP is used on a VRRP router (§8.1.3; lowercase "must" in the RFC) | no test | no test carries this requirement id |
| [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1) Master: never respond to an ND Neighbor Solicitation for a virtual router IPv6 address with its physical MAC address (§8.2.2) | no test | no test carries this requirement id |
| [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) Master: include the virtual router MAC address in the source link-layer address option of a Neighbor Solicitation it sends for a host's IPv6 address, when it sends that option (§8.2.2) | no test | no test carries this requirement id |
| [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) Master: never use its physical MAC address in that source link-layer address option (§8.2.2) | no test | no test carries this requirement id |
| [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4) At system boot, delay every ND Router Advertisement, Neighbor Advertisement and Neighbor Solicitation until both the IPv6 address and the virtual router MAC address are configured (§8.2.2; lowercase "must" in the RFC) | no test | no test carries this requirement id |
| [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) Configure Backup routers to send the same Router Advertisement options as the address owner (§8.2.3; lowercase "must" in the RFC) | no test | no test carries this requirement id |
| [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1) Interop mode, Master: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even when it is sub-second (§8.4.2) | no test | no test carries this requirement id |
| [`RFC5798-A.2-1`](#rfc5798-a.2-1) Token Ring: implement the functional-address mode of operation when supporting VRRP on Token Ring (§A.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1)

Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.1.2-1, so no unit is bound to it.

### [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1)

Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.1.3-1, so no unit is bound to it.

### [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2)

Discard a received IPv4 VRRP packet whose TTL is not 255 (§5.1.1.3, §7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.1.3-2, so no unit is bound to it.

### [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1)

Never forward a datagram destined to FF02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.2.2-1, so no unit is bound to it.

### [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1)

Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.2.3-1, so no unit is bound to it.

### [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2)

Discard a received IPv6 VRRP packet whose Hop Limit is not 255 (§5.1.2.3, §7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.1.2.3-2, so no unit is bound to it.

### [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1)

Discard a packet with unknown Type; 1 = ADVERTISEMENT is the only type defined (§5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.2-1, so no unit is bound to it.

### [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1)

Use Priority 255 for the VRRP router that owns the IPvX address associated with the virtual router (§5.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.4-1, so no unit is bound to it.

### [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2)

Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.4-2, so no unit is bound to it.

### [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1)

Set the rsvd field to zero on transmission and ignore it on reception (§5.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.6-1, so no unit is bound to it.

### [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1)

Compute and verify the checksum as the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a pseudo-header defined by RFC 2460, with next header 112 and the checksum field zeroed, for both address families (§5.2.8, §7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.8-1, so no unit is bound to it.

### [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1)

Send the IPv6 link-local address associated with the virtual router as the first address in the list (§5.2.9, §6.1; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.9-1, so no unit is bound to it.

### [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2)

Never carry IPv4 and IPv6 addresses together in one IPvX Address field (§5.2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-5.2.9-2, so no unit is bound to it.

### [`RFC5798-6.1-1`](#rfc5798-6.1-1)

Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.1-1, so no unit is bound to it.

### [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1)

Backup: never respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-1, so no unit is bound to it.

### [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2)

Backup: never respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-2, so no unit is bound to it.

### [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3)

Backup: never send ND Router Advertisement messages for the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-3, so no unit is bound to it.

### [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4)

Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-4, so no unit is bound to it.

### [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5)

Backup: never accept packets addressed to the IPvX address(es) associated with the virtual router (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-5, so no unit is bound to it.

### [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6)

Backup: on a Shutdown event, cancel the Master_Down_Timer and transition to Initialize (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-6, so no unit is bound to it.

### [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7)

Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP carrying the virtual router MAC for each IPv4 address or, for IPv6, join the Solicited-Node multicast address and send an unsolicited Neighbor Advertisement for each address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-7, so no unit is bound to it.

### [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8)

Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-8, so no unit is bound to it.

### [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9)

Backup: on a non-zero-priority ADVERTISEMENT, when Preempt_Mode is False or the advertised Priority is greater than or equal to the local Priority, set Master_Adver_Interval to the Adver Interval in the advertisement, recompute Master_Down_Interval, and reset the Master_Down_Timer to it (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-9, so no unit is bound to it.

### [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10)

Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and an advertised Priority lower than the local Priority, discard the ADVERTISEMENT (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.2-10, so no unit is bound to it.

### [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1)

Master: respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.3, §8.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-1, so no unit is bound to it.

### [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2)

Master: be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-2, so no unit is bound to it.

### [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3)

Master: respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.3, §8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-3, so no unit is bound to it.

### [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4)

Master: send ND Router Advertisements for the virtual router (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-4, so no unit is bound to it.

### [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5)

Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-5, so no unit is bound to it.

### [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6)

Master: accept packets addressed to the IPvX address(es) associated with the virtual router when it is the address owner or when Accept_Mode is True (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-6, so no unit is bound to it.

### [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7)

Master: never accept those packets when it is neither the address owner nor configured with Accept_Mode True (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-7, so no unit is bound to it.

### [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8)

Master: on a Shutdown event, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-8, so no unit is bound to it.

### [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9)

Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-9, so no unit is bound to it.

### [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10)

Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-10, so no unit is bound to it.

### [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11)

Master: on an ADVERTISEMENT with a higher Priority, or an equal Priority with a greater sender primary IPvX address, cancel the Adver_Timer, set Master_Adver_Interval to the advertised Adver Interval, recompute Skew_Time and Master_Down_Interval, set the Master_Down_Timer, and transition to Backup (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-11, so no unit is bound to it.

### [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12)

Master: on a losing ADVERTISEMENT, one with a lower Priority or an equal Priority with a smaller sender address, discard it (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-12, so no unit is bound to it.

### [`RFC5798-7.1-1`](#rfc5798-7.1-1)

Rx: verify that the VRRP version is 3 (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-1, so no unit is bound to it.

### [`RFC5798-7.1-2`](#rfc5798-7.1-2)

Rx: verify that the received packet contains the complete VRRP packet, the fixed fields and the IPvX address list (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-2, so no unit is bound to it.

### [`RFC5798-7.1-3`](#rfc5798-7.1-3)

Rx: verify that the VRID is configured on the receiving interface and that the local router is not the IPvX address owner (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-3, so no unit is bound to it.

### [`RFC5798-7.1-4`](#rfc5798-7.1-4)

Rx: discard the packet when any mandatory receive check fails (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-4, so no unit is bound to it.

### [`RFC5798-7.2-1`](#rfc5798-7.2-1)

Tx: fill in the VRRP packet fields from the virtual router configuration state and compute the VRRP checksum (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.2-1, so no unit is bound to it.

### [`RFC5798-7.2-2`](#rfc5798-7.2-2)

Tx: set the source MAC address to the virtual router MAC address (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.2-2, so no unit is bound to it.

### [`RFC5798-7.2-3`](#rfc5798-7.2-3)

Tx: set the source IPv4 address to the interface primary IPv4 address, or the source IPv6 address to the interface link-local IPv6 address (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.2-3, so no unit is bound to it.

### [`RFC5798-7.2-4`](#rfc5798-7.2-4)

Tx: set the IPvX protocol to VRRP and send the packet to the VRRP IPvX multicast group (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.2-4, so no unit is bound to it.

### [`RFC5798-7.4-1`](#rfc5798-7.4-1)

Create the Interface Identifiers of an IPv6 router running VRRP in the normal manner, as in Transmission of IPv6 Packets over Ethernet Networks (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.4-1, so no unit is bound to it.

### [`RFC5798-7.4-2`](#rfc5798-7.4-2)

Never use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers (§7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.4-2, so no unit is bound to it.

### [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1)

Master: never respond to a host ARP request for a virtual router IPv4 address with its physical MAC address (§8.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.1.2-1, so no unit is bound to it.

### [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1)

Advertise the virtual router MAC address in the Proxy ARP message when Proxy ARP is used on a VRRP router (§8.1.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.1.3-1, so no unit is bound to it.

### [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1)

Master: never respond to an ND Neighbor Solicitation for a virtual router IPv6 address with its physical MAC address (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-1, so no unit is bound to it.

### [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2)

Master: include the virtual router MAC address in the source link-layer address option of a Neighbor Solicitation it sends for a host's IPv6 address, when it sends that option (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-2, so no unit is bound to it.

### [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3)

Master: never use its physical MAC address in that source link-layer address option (§8.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-3, so no unit is bound to it.

### [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4)

At system boot, delay every ND Router Advertisement, Neighbor Advertisement and Neighbor Solicitation until both the IPv6 address and the virtual router MAC address are configured (§8.2.2; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.2-4, so no unit is bound to it.

### [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1)

Configure Backup routers to send the same Router Advertisement options as the address owner (§8.2.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.2.3-1, so no unit is bound to it.

### [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1)

Interop mode, Master: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even when it is sub-second (§8.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.4.2-1, so no unit is bound to it.

### [`RFC5798-A.2-1`](#rfc5798-a.2-1)

Token Ring: implement the functional-address mode of operation when supporting VRRP on Token Ring (§A.2)

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
| [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) Never forward a datagram destined to 224.0.0.18, regardless of its TTL (§5.1.1.2) | restated | RFC9568-5.1.1.2-1 | RFC 9568 Section 5.1.1.2 keeps the rule for IPv4 word for word |
| [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1) Set the IPv4 TTL of transmitted VRRP packets to 255 (§5.1.1.3) | restated | RFC9568-5.1.1.3-1 | RFC 9568 Section 5.1.1.3 keeps the IPv4 TTL of 255 on transmit |
| [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2) Discard a received IPv4 VRRP packet whose TTL is not 255 (§5.1.1.3, §7.1) | restated | RFC9568-5.1.1.3-2 | RFC 9568 Section 5.1.1.3 keeps the discard rule and Section 7.1 keeps the matching receive check |
| [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1) Never forward a datagram destined to FF02:0:0:0:0:0:0:12, regardless of its Hop Limit (§5.1.2.2) | restated | RFC9568-5.1.2.2-1 | RFC 9568 Section 5.1.2.2 keeps the rule for the IPv6 group |
| [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1) Set the IPv6 Hop Limit of transmitted VRRP packets to 255 (§5.1.2.3) | restated | RFC9568-5.1.2.3-1 | RFC 9568 Section 5.1.2.3 keeps the Hop Limit of 255 on transmit |
| [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2) Discard a received IPv6 VRRP packet whose Hop Limit is not 255 (§5.1.2.3, §7.1) | restated | RFC9568-5.1.2.3-2 | RFC 9568 Section 5.1.2.3 keeps the discard rule and Section 7.1 keeps the matching receive check |
| [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1) Discard a packet with unknown Type; 1 = ADVERTISEMENT is the only type defined (§5.2.2) | restated | RFC9568-5.2.2-1 | RFC 9568 Section 5.2.2 keeps ADVERTISEMENT as the only defined type and keeps the discard rule, and Section 7.1 adds an explicit receive check for it |
| [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1) Use Priority 255 for the VRRP router that owns the IPvX address associated with the virtual router (§5.2.4) | restated | RFC9568-5.2.4-1 | RFC 9568 Section 5.2.4 keeps Priority 255 for the address owner |
| [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2) Use Priority values 1-254 for VRRP routers backing up a virtual router (§5.2.4) | restated | RFC9568-5.2.4-2 | RFC 9568 Section 5.2.4 keeps the 1 to 254 range for backup routers |
| [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1) Set the rsvd field to zero on transmission and ignore it on reception (§5.2.6) | restated | RFC9568-5.2.6-1 | RFC 9568 Section 5.2.6 renames the field to Reserve and keeps both halves of the rule |
| [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) Compute and verify the checksum as the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a pseudo-header defined by RFC 2460, with next header 112 and the checksum field zeroed, for both address families (§5.2.8, §7.1) | restated | RFC9568-5.2.8-1 | RFC 9568 Section 5.2.8 keeps the ones-complement arithmetic and CHANGES the coverage, stating the IPv4 checksum over the VRRP message alone and keeping the RFC 8200 Section 8.1 pseudo-header for IPv6 only. RFC 9568 Section 1.2 lists that as change 4. The two documents therefore disagree about the IPv4 checksum, and the deployed base computes this one |
| [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1) Send the IPv6 link-local address associated with the virtual router as the first address in the list (§5.2.9, §6.1; lowercase "must" in the RFC) | restated | RFC9568-5.2.9-1 | RFC 9568 Section 5.2.9 states the same rule with an uppercase MUST, and erratum 8300 adds the matching receive check |
| [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2) Never carry IPv4 and IPv6 addresses together in one IPvX Address field (§5.2.9) | restated | RFC9568-5.2.9-2 | RFC 9568 Section 5.2.9 states the rule as an address family that MUST be the same as the packet's IPvX header address family |
| [`RFC5798-6.1-1`](#rfc5798-6.1-1) Never drop IPv6 Neighbor Solicitations and Neighbor Advertisements when Accept_Mode is False (§6.1, §6.4.3) | restated | RFC9568-6.1-1 | RFC 9568 Section 6.1 keeps the Accept_Mode note verbatim |
| [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1) Backup: never respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.2) | restated | RFC9568-6.4.2-1 | RFC 9568 Section 6.4.2 keeps the rule and renames the state to Backup Router |
| [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2) Backup: never respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.2) | restated | RFC9568-6.4.2-2 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3) Backup: never send ND Router Advertisement messages for the virtual router (§6.4.2) | restated | RFC9568-6.4.2-3 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4) Backup: discard packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.2) | restated | RFC9568-6.4.2-4 | RFC 9568 Section 6.4.2 keeps the discard unchanged |
| [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5) Backup: never accept packets addressed to the IPvX address(es) associated with the virtual router (§6.4.2) | restated | RFC9568-6.4.2-5 | RFC 9568 Section 6.4.2 keeps the rule unchanged |
| [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6) Backup: on a Shutdown event, cancel the Master_Down_Timer and transition to Initialize (§6.4.2) | restated | RFC9568-6.4.2-6 | RFC 9568 Section 6.4.2 keeps the Shutdown transition and renames the timer to Active_Down_Timer |
| [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7) Backup: when the Master_Down_Timer fires, send an ADVERTISEMENT, broadcast a gratuitous ARP carrying the virtual router MAC for each IPv4 address or, for IPv6, join the Solicited-Node multicast address and send an unsolicited Neighbor Advertisement for each address, set the Adver_Timer to Advertisement_Interval, and transition to Master (§6.4.2) | restated | RFC9568-6.4.2-7 | RFC 9568 Section 6.4.2 keeps the whole sequence, renames the timer and the state, and erratum 7949 corrects the gratuitous ARP to carry the virtual router IPv4 address with the virtual router MAC as the target link-layer address |
| [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8) Backup: on an ADVERTISEMENT with Priority 0, set the Master_Down_Timer to Skew_Time (§6.4.2) | restated | RFC9568-6.4.2-8 | RFC 9568 Section 6.4.2 keeps the Skew_Time rule for a Priority 0 advertisement |
| [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9) Backup: on a non-zero-priority ADVERTISEMENT, when Preempt_Mode is False or the advertised Priority is greater than or equal to the local Priority, set Master_Adver_Interval to the Adver Interval in the advertisement, recompute Master_Down_Interval, and reset the Master_Down_Timer to it (§6.4.2) | restated | RFC9568-6.4.2-9 | RFC 9568 Section 6.4.2 keeps the rule and ADDS one step, recomputing Skew_Time beside Active_Down_Interval, which this document omits |
| [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10) Backup: on a non-zero-priority ADVERTISEMENT with Preempt_Mode True and an advertised Priority lower than the local Priority, discard the ADVERTISEMENT (§6.4.2) | restated | RFC9568-6.4.2-10 | RFC 9568 Section 6.4.2 keeps the discard unchanged |
| [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1) Master: respond to ARP requests for the IPv4 address(es) associated with the virtual router (§6.4.3, §8.1.2) | restated | RFC9568-6.4.3-1 | RFC 9568 Section 6.4.3 keeps the ARP obligation and renames the state to Active Router |
| [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2) Master: be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router (§6.4.3) | restated | RFC9568-6.4.3-2 | RFC 9568 Section 6.4.3 keeps the membership obligation unchanged |
| [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3) Master: respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router (§6.4.3, §8.2.2) | restated | RFC9568-6.4.3-3 | RFC 9568 Section 6.4.3 keeps the obligation and ADDS that the Neighbor Advertisement carries the Router Flag set |
| [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) Master: send ND Router Advertisements for the virtual router (§6.4.3) | restated | RFC9568-6.4.3-4 | RFC 9568 Section 6.4.3 keeps the obligation unchanged |
| [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5) Master: forward packets with a destination link-layer MAC address equal to the virtual router MAC address (§6.4.3) | restated | RFC9568-6.4.3-5 | RFC 9568 Section 6.4.3 keeps the forwarding obligation unchanged |
| [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6) Master: accept packets addressed to the IPvX address(es) associated with the virtual router when it is the address owner or when Accept_Mode is True (§6.4.3) | restated | RFC9568-6.4.3-6 | RFC 9568 Section 6.4.3 keeps both admitting conditions unchanged |
| [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7) Master: never accept those packets when it is neither the address owner nor configured with Accept_Mode True (§6.4.3) | restated | RFC9568-6.4.3-7 | RFC 9568 Section 6.4.3 keeps the refusal unchanged |
| [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8) Master: on a Shutdown event, cancel the Adver_Timer, send an ADVERTISEMENT with Priority = 0, and transition to Initialize (§6.4.3) | restated | RFC9568-6.4.3-8 | RFC 9568 Section 6.4.3 keeps the Shutdown sequence unchanged |
| [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9) Master: when the Adver_Timer fires, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-9 | RFC 9568 Section 6.4.3 keeps the timer-expiry behaviour unchanged |
| [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10) Master: on an ADVERTISEMENT with Priority 0, send an ADVERTISEMENT and reset the Adver_Timer to Advertisement_Interval (§6.4.3) | restated | RFC9568-6.4.3-10 | RFC 9568 Section 6.4.3 keeps the response to a Priority 0 advertisement unchanged |
| [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11) Master: on an ADVERTISEMENT with a higher Priority, or an equal Priority with a greater sender primary IPvX address, cancel the Adver_Timer, set Master_Adver_Interval to the advertised Adver Interval, recompute Skew_Time and Master_Down_Interval, set the Master_Down_Timer, and transition to Backup (§6.4.3) | restated | RFC9568-6.4.3-11 | RFC 9568 Section 6.4.3 keeps the whole sequence and states that the sender address comparison is an unsigned integer comparison in network byte order |
| [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12) Master: on a losing ADVERTISEMENT, one with a lower Priority or an equal Priority with a smaller sender address, discard it (§6.4.3) | restated | RFC9568-6.4.3-12 | RFC 9568 Section 6.4.3 keeps the discard and ADDS a step this document does not have, sending an advertisement immediately so learning bridges relearn the segment. RFC 9568 Section 1.2 lists that as change 5 |
| [`RFC5798-7.1-1`](#rfc5798-7.1-1) Rx: verify that the VRRP version is 3 (§7.1) | restated | RFC9568-7.1-1 | RFC 9568 Section 7.1 keeps the version check |
| [`RFC5798-7.1-2`](#rfc5798-7.1-2) Rx: verify that the received packet contains the complete VRRP packet, the fixed fields and the IPvX address list (§7.1) | restated | RFC9568-7.1-3 | RFC 9568 Section 7.1 keeps the completeness check |
| [`RFC5798-7.1-3`](#rfc5798-7.1-3) Rx: verify that the VRID is configured on the receiving interface and that the local router is not the IPvX address owner (§7.1) | restated | RFC9568-7.1-4 | RFC 9568 Section 7.1 keeps both halves as published, and erratum 8298 splits them, lowering the address-owner half to a SHOULD that logs rather than discards |
| [`RFC5798-7.1-4`](#rfc5798-7.1-4) Rx: discard the packet when any mandatory receive check fails (§7.1) | restated | RFC9568-7.1-6 | RFC 9568 Section 7.1 keeps the discard on a failed mandatory check, with the SHOULD log and the MAY network-management indication beside it |
| [`RFC5798-7.2-1`](#rfc5798-7.2-1) Tx: fill in the VRRP packet fields from the virtual router configuration state and compute the VRRP checksum (§7.2) | restated | RFC9568-7.2-1 | RFC 9568 Section 7.2 keeps the fill-and-checksum step unchanged |
| [`RFC5798-7.2-2`](#rfc5798-7.2-2) Tx: set the source MAC address to the virtual router MAC address (§7.2) | restated | RFC9568-7.2-2 | RFC 9568 Section 7.2 keeps the virtual router MAC as the source link-layer address |
| [`RFC5798-7.2-3`](#rfc5798-7.2-3) Tx: set the source IPv4 address to the interface primary IPv4 address, or the source IPv6 address to the interface link-local IPv6 address (§7.2) | restated | RFC9568-7.2-3 | RFC 9568 Section 7.2 keeps both source-address rules unchanged |
| [`RFC5798-7.2-4`](#rfc5798-7.2-4) Tx: set the IPvX protocol to VRRP and send the packet to the VRRP IPvX multicast group (§7.2) | restated | RFC9568-7.2-4 | RFC 9568 Section 7.2 keeps protocol 112 and the IPvX multicast destination |
| [`RFC5798-7.4-1`](#rfc5798-7.4-1) Create the Interface Identifiers of an IPv6 router running VRRP in the normal manner, as in Transmission of IPv6 Packets over Ethernet Networks (§7.4) | dropped | not stated | RFC 9568 Section 7.4 replaces the sentence rather than restating it. It cites RFC 8064 and RFC 7217 as the default scheme for a stable SLAAC address and states no "normal manner" obligation, so no equivalent requirement remains |
| [`RFC5798-7.4-2`](#rfc5798-7.4-2) Never use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers (§7.4) | restated | RFC9568-7.4-1 | RFC 9568 Section 7.4 keeps the prohibition and restates it over the Net_Iface parameter of the RFC 7217 and RFC 8981 derivation algorithms |
| [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1) Master: never respond to a host ARP request for a virtual router IPv4 address with its physical MAC address (§8.1.2) | restated | RFC9568-8.1.2-1 | RFC 9568 Section 8.1.2 keeps the prohibition unchanged |
| [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) Advertise the virtual router MAC address in the Proxy ARP message when Proxy ARP is used on a VRRP router (§8.1.3; lowercase "must" in the RFC) | restated | RFC9568-8.1.3-1 | RFC 9568 Section 8.1.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1) Master: never respond to an ND Neighbor Solicitation for a virtual router IPv6 address with its physical MAC address (§8.2.2) | restated | RFC9568-8.2.2-1 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) Master: include the virtual router MAC address in the source link-layer address option of a Neighbor Solicitation it sends for a host's IPv6 address, when it sends that option (§8.2.2) | restated | RFC9568-8.2.2-2 | RFC 9568 Section 8.2.2 keeps the obligation unchanged |
| [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) Master: never use its physical MAC address in that source link-layer address option (§8.2.2) | restated | RFC9568-8.2.2-3 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4) At system boot, delay every ND Router Advertisement, Neighbor Advertisement and Neighbor Solicitation until both the IPv6 address and the virtual router MAC address are configured (§8.2.2; lowercase "must" in the RFC) | restated | RFC9568-8.2.2-4 | RFC 9568 Section 8.2.2 states the same delay with an uppercase MUST |
| [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) Configure Backup routers to send the same Router Advertisement options as the address owner (§8.2.3; lowercase "must" in the RFC) | restated | RFC9568-8.2.3-1 | RFC 9568 Section 8.2.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1) Interop mode, Master: send both VRRPv2 and VRRPv3 advertisements at the configured rate, even when it is sub-second (§8.4.2) | restated | RFC9568-8.4.2-1 | RFC 9568 Section 8.4.2 keeps the obligation unchanged |
| [`RFC5798-A.2-1`](#rfc5798-a.2-1) Token Ring: implement the functional-address mode of operation when supporting VRRP on Token Ring (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no functional-address obligation |
| [`RFC5798-7.1-5`](#rfc5798-7.1-5) Log the event when a mandatory receive check fails (§7.1) | restated | RFC9568-7.1-7 | RFC 9568 Section 7.1 keeps the log and adds rate-limiting to it |
| [`RFC5798-7.1-8`](#rfc5798-7.1-8) Log the event when the optional address-list check fails (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 keeps the log, adds rate-limiting, and states it in one sentence covering the Max Advertise Interval mismatch as well |
| [`RFC5798-8.1.1-1`](#rfc5798-8.1.1-1) Set the IPv4 source address of an ICMP redirect to the address the end-host used when making its next-hop routing decision (§8.1.1; lowercase "should" in the RFC) | unextracted | §8.1.1 | RFC 9568 Section 8.1.1 keeps the sentence for IPv4, and rfc/short/rfc9568.md declares a row for the IPv6 counterpart at RFC9568-8.2.1-1 only. The IPv4 half of that pair is an extraction hole in the successor's summary |
| [`RFC5798-8.1.2-2`](#rfc5798-8.1.2-2) After a restart or boot, send no ARP message using the physical MAC address for an owned virtual IPv4 address (§8.1.2) | restated | RFC9568-8.1.2-3 | RFC 9568 Section 8.1.2 keeps the rule unchanged |
| [`RFC5798-8.1.2-3`](#rfc5798-8.1.2-3) When configuring an interface, broadcast a gratuitous ARP request carrying the virtual router MAC address for each IPv4 address on that interface (§8.1.2; lowercase "should" in the RFC) | restated | RFC9568-8.1.2-4 | RFC 9568 Section 8.1.2 keeps this half at SHOULD |
| [`RFC5798-8.1.2-4`](#rfc5798-8.1.2-4) At system boot, delay gratuitous ARP requests and ARP responses until both the IPv4 address and the virtual router MAC address are configured (§8.1.2) | restated | RFC9568-8.1.2-2 | RFC 9568 Section 8.1.2 splits the boot bullet out and RAISES it to a MUST |
| [`RFC5798-8.1.2-5`](#rfc5798-8.1.2-5) Use an IP address known to belong to a particular router when direct access to that router is required, for example ssh (§8.1.2; lowercase "must" inside a SHOULD-level bullet list) | restated | RFC9568-8.1.2-5 | RFC 9568 Section 8.1.2 keeps the recommendation |
| [`RFC5798-8.2.1-1`](#rfc5798-8.2.1-1) Set the IPv6 source address of an ICMPv6 redirect to the address the end-host used when making its next-hop routing decision (§8.2.1; lowercase "should" in the RFC) | restated | RFC9568-8.2.1-1 | RFC 9568 Section 8.2.1 keeps the recommendation |
| [`RFC5798-8.2.2-5`](#rfc5798-8.2.2-5) After a restart or boot, send no ND message using the physical MAC address for an owned virtual IPv6 address (§8.2.2) | restated | RFC9568-8.2.2-5 | RFC 9568 Section 8.2.2 keeps the rule unchanged |
| [`RFC5798-8.2.2-6`](#rfc5798-8.2.2-6) When configuring an interface, send an unsolicited ND Neighbor Advertisement carrying the virtual router MAC address for the IPv6 address on that interface (§8.2.2; lowercase "should" in the RFC) | restated | RFC9568-8.2.2-6 | RFC 9568 Section 8.2.2 keeps the recommendation |
| [`RFC5798-8.2.3-2`](#rfc5798-8.2.3-2) Send Router Advertisement options that advertise special services from the address owner unless the Backup routers can assume those services in full with a complete and synchronized database (§8.2.3; lowercase "should not" in the RFC) | restated | RFC9568-8.2.3-2 | RFC 9568 Section 8.2.3 keeps the recommendation |
| [`RFC5798-8.3.1-1`](#rfc5798-8.3.1-1) Never forward packets addressed to the IPvX address a router becomes Master for when it is not the address owner (§8.3.1) | restated | RFC9568-8.3.1-1 | RFC 9568 Section 8.3.1 keeps the rule and states it over both address families |
| [`RFC5798-8.3.2-1`](#rfc5798-8.3.2-1) Configure no more than one VRRP router on the link with priority 255 for a single VRID (§8.3.2; lowercase in the RFC) | restated | RFC9568-8.3.2-1 | RFC 9568 Section 8.3.2 keeps the recommendation and adds a rate-limited log when several priority-255 advertisers are seen |
| [`RFC5798-8.3.2-2`](#rfc5798-8.3.2-2) Distribute the priority values of multiple Backup routers uniformly to speed convergence (§8.3.2; lowercase in the RFC) | restated | RFC9568-8.3.2-3 | RFC 9568 Section 8.3.2 keeps the recommendation and states it as sufficiently different priorities |
| [`RFC5798-8.4.2-2`](#rfc5798-8.4.2-2) Do not run VRRPv2 and VRRPv3 mixed operation as a permanent deployment; it is an upgrade path (§8.4.2) | restated | RFC9568-8.4.2-5 | RFC 9568 Section 8.4.2 keeps the same statement |
| [`RFC5798-8.4.2-3`](#rfc5798-8.4.2-3) Interop mode, Backup: time out from the rate the Master advertises, translating a VRRPv2 Master's seconds into centiseconds (§8.4.2) | restated | RFC9568-8.4.2-2 | RFC 9568 Section 8.4.2 RAISES both halves to MUST and splits them, keeping the timeout at RFC9568-8.4.2-2 and the seconds-to-centiseconds translation at RFC9568-8.4.2-3 |
| [`RFC5798-8.4.2-4`](#rfc5798-8.4.2-4) Interop mode, Backup: ignore VRRPv2 advertisements from the current Master when VRRPv3 packets are also being received from it (§8.4.2) | restated | RFC9568-8.4.2-4 | RFC 9568 Section 8.4.2 keeps the recommendation |
| [`RFC5798-8.4.3.1-1`](#rfc5798-8.4.3.1-1) Never give a VRRPv2 implementation a higher priority than the VRRPv2/VRRPv3 implementation it interacts with when that peer advertises at a sub-second rate (§8.4.3.1) | restated | RFC9568-8.4.2.1.1-1 | RFC 9568 renumbers the subsection to 8.4.2.1.1 and keeps the recommendation |
| [`RFC5798-A.1-1`](#rfc5798-a.1-1) FDDI: configure the virtual router MAC address by adding a unicast MAC filter in the FDDI device rather than changing its hardware MAC address (§A.1) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no unicast-MAC-filter recommendation |
| [`RFC5798-7.1-6`](#rfc5798-7.1-6) Indicate through network management that a receive error, or a detected misconfiguration, occurred (§7.1) | restated | RFC9568-7.1-10 | RFC 9568 Section 7.1 keeps the network-management indication as a MAY |
| [`RFC5798-7.1-7`](#rfc5798-7.1-7) Verify that "Count IPvX Addrs" and the list of IPvX addresses match the addresses configured for the VRID (§7.1) | restated | RFC9568-7.1-11 | RFC 9568 Section 7.1 keeps the address-list check as a MAY and renames the field to IPvX Addr Count |
| [`RFC5798-8.2.2-7`](#rfc5798-8.2.2-7) Answer Duplicate Address Detection for an owned address from the Backup router while the Master restarts; one solution is not to run DAD in that case (§8.2.2) | restated | RFC9568-8.2.2-7 | RFC 9568 Section 8.2.2 keeps the note |
| [`RFC5798-8.4.2-5`](#rfc5798-8.4.2-5) Implement a configuration flag that tells the router to listen for and send both VRRPv2 and VRRPv3 advertisements (§8.4.2) | restated | RFC9568-8.4.2-6 | RFC 9568 Section 8.4.2 keeps the flag as a MAY |
| [`RFC5798-8.4.2-6`](#rfc5798-8.4.2-6) Report when a VRRPv3 Master is not sending VRRPv2 packets while interop mode is configured (§8.4.2) | restated | RFC9568-8.4.2-7 | RFC 9568 Section 8.4.2 keeps the report as a MAY |
| [`RFC5798-A.2-2`](#rfc5798-a.2-2) Token Ring: support the unicast mode of operation beside the functional-address mode (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no unicast-mode permission |
