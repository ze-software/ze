# RFC 4302 - IP Authentication Header

Supported in OSPFv3 manual IPsec path. Every requirement this repository extracted from RFC 4302, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 34 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 34 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 34 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 34 | of 44 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 34 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 34 of 34 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 34 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported in OSPFv3 manual IPsec path |
| Enrolment | Enrolled |
| Requirements | 44 |
| Gated MUST-level | 34 |
| Obligations that bind Ze | 34 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 34 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4302.md` |
| Requirement shard | `rfc/requirements/rfc4302.md` |
| RFC text | `rfc/full/rfc4302.txt` |

## Enrolment

Enrolled: IP Authentication Header (RFC 4302): control plane only. Ze speaks AH through RFC 4552 manually-keyed IPsec for OSPFv3: validateIPsecInterface (plugins/ospf/config_ipsec.go) accepts protocol ah with an SPI at or above 256, an HMAC-SHA algorithm and a hex key of that algorithm length, and refuses an encryption key beside AH; buildIPsecSA and buildIPsecPolicies (plugins/ospf/ipsec_install.go) install one transport-mode SA with Proto ProtoAH and a {::/0, ::/0, proto 89} state selector plus out/in/fwd policies scoped to the interface; planStateAlgos (ike/dataplane/dataplane.go) gives an AH state an integrity transform and no encryption transform, and xfrmStateFromParams (ike/dataplane/xfrm_linux.go) writes it. Every per-packet AH obligation -- header construction, sequence numbers, mutable-field zeroing, ICV, replay window, discard on failure -- is performed by Linux XFRM, which the whole-stack conformance ruling of 2026-08-31 counts as an implementation of the obligation rather than an exemption. Ze never negotiates an AH Child SA over IKEv2, so every AH SA it installs is manually keyed and carries no replay window, which Section 5 asks for. 34 gated rows, none tested and none annotated at enrolment; the interop scenario ospf-ipsec-ah-frr already reads the installed kernel state and is the carrier the coverage work will tag.

## What the public ledger says

**Status:** Supported in OSPFv3 manual IPsec path

**What the ledger says is covered:**

AH algorithm planning and RFC 4552 OSPFv3 use.

**What the ledger says remains:**

Scoped to configured manual IPsec support.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 34 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **34** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (34):** [`RFC4302-2-1`](#rfc4302-2-1), [`RFC4302-2-2`](#rfc4302-2-2), [`RFC4302-2.3-1`](#rfc4302-2.3-1), [`RFC4302-2.4-1`](#rfc4302-2.4-1), [`RFC4302-2.4-2`](#rfc4302-2.4-2), [`RFC4302-2.4-3`](#rfc4302-2.4-3), [`RFC4302-2.4-4`](#rfc4302-2.4-4), [`RFC4302-2.4-5`](#rfc4302-2.4-5), [`RFC4302-2.4-6`](#rfc4302-2.4-6), [`RFC4302-2.5-1`](#rfc4302-2.5-1), [`RFC4302-2.5-2`](#rfc4302-2.5-2), [`RFC4302-2.5-3`](#rfc4302-2.5-3), [`RFC4302-2.5-5`](#rfc4302-2.5-5), [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1), [`RFC4302-2.6-1`](#rfc4302-2.6-1), [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1), [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1), [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1), [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2), [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3), [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4), [`RFC4302-3.3.4-1`](#rfc4302-3.3.4-1), [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1), [`RFC4302-3.4.2-1`](#rfc4302-3.4.2-1), [`RFC4302-3.4.3-1`](#rfc4302-3.4.3-1), [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2), [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3), [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4), [`RFC4302-3.4.3-5`](#rfc4302-3.4.3-5), [`RFC4302-4-1`](#rfc4302-4-1), [`RFC4302-5-1`](#rfc4302-5-1), [`RFC4302-5-2`](#rfc4302-5-2), [`RFC4302-A2-1`](#rfc4302-a2-1), [`RFC4302-A2-2`](#rfc4302-a2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4302-2-1` | The protocol header immediately preceding the AH header SHALL contain the value 51 in its Protocol or Next Header field (§2) | SHALL | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2-2` | AH carries no version number, so backward-compatibility concerns MUST be addressed by a signaling mechanism between the two peers, an SA management protocol or an out-of-band configuration mechanism (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.3-1` | The RESERVED field MUST be set to zero by the sender (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-1` | The SPI field is mandatory, and the mechanism for mapping inbound traffic to unicast SAs MUST be supported by all AH implementations (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-2` | An implementation that supports multicast MUST support multicast SAs using the SAD search order this section gives (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-3` | A multicast-capable implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-4` | An accelerated SAD search MUST be functionally equivalent, in externally visible behavior, to searching the SAD in the order given (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-5` | Whether source and destination address matching is required to map inbound traffic to an SA MUST be set as a side effect of manual SA configuration or by an SA management protocol (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-6` | The SPI value zero is reserved for local implementation-specific use and MUST NOT be sent on the wire (§2.4) | MUST NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5-1` | For a unicast SA or a single-sender multicast SA the sender MUST increment the Sequence Number for every transmitted packet (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5-2` | The Sequence Number field is mandatory and MUST always be present and transmitted, even when the receiver does not enable anti-replay (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5-3` | All AH implementations MUST be capable of performing the sequence number generation and the sequence number verification this document describes (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5-5` | The sender's and receiver's counters MUST be reset, by establishing a new SA and a new key, before the 2^32nd packet is transmitted on an SA (§2.5) | MUST | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5.1-1` | Use of an Extended Sequence Number MUST be negotiated by an SA management protocol (§2.5.1) | MUST | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.6-1` | All implementations MUST support explicit ICV padding and MUST insert only enough padding to satisfy the IPv4 or IPv6 alignment requirement (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.2-1` | The sender MUST NOT send a packet on an SA if doing so would cause the Sequence Number to cycle (§3.3.2) | MUST NOT | 3.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.3.1-1` | For the ICV computation a field that may be modified in transit is set to zero, a mutable but predictable field is set to the value it will have at the receiver, and the ICV field itself is set to zero (§3.3.3.1) | MUST | 3.3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.3.2.2-1` | When the packet length does not match the integrity algorithm's blocksize, implicit padding MUST be appended to the end of the packet before the ICV is computed (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.3.2.2-2` | The implicit padding octets MUST have a value of zero (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.3.2.2-3` | The document defining the integrity algorithm MUST be consulted to decide whether implicit padding is required (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.3.2.2-4` | When that document gives no answer, implicit padding is assumed to be required and its octets MUST have a value of zero (§3.3.3.2.2) | MUST | 3.3.3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.4-1` | An AH implementation MUST support generation of ICMP PMTU messages, or the equivalent internal signaling for a native host implementation (§3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.1-1` | A packet offered to AH that appears to be an IP fragment MUST be discarded, and that is an auditable event (§3.4.1) | MUST | 3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.2-1` | When no valid Security Association exists for a packet the receiver MUST discard it, and that is an auditable event (§3.4.2) | MUST | 3.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-1` | All AH implementations MUST support the anti-replay service, whose use the receiver enables or disables per SA (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-2` | When anti-replay is enabled for an SA the receive packet counter MUST be initialized to zero as the SA is established (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-3` | For each received packet the receiver MUST verify that the Sequence Number does not duplicate one already received on this SA (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-4` | When ICV validation fails the receiver MUST discard the datagram as invalid (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-5` | A minimum replay window size of 32 packets MUST be supported (§3.4.3) | MUST | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-4-1` | When AH is incorporated into a system that supports auditing, the AH implementation MUST also support auditing and MUST allow an administrator to enable or disable auditing for AH (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-1` | An implementation claiming conformance MUST fully implement the AH syntax and processing for unicast traffic, and MUST comply with all requirements of the Security Architecture document (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-2` | An implementation claiming to support multicast traffic MUST comply with the additional requirements specified for such traffic (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-A2-1` | When the IP implementation leaves a Fragmentation Extension Header in place after reassembly, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2) | MUST | A2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-A2-2` | When the IP implementation hands AH a send-side packet carrying a Fragmentation Extension Header with Offset zero and More Fragments zero, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2) | MUST | A2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.3-2` | The RESERVED field SHOULD be ignored by the recipient (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.5.1-2` | A 64-bit sequence number option SHOULD be offered as an extension to the 32-bit Sequence Number field (§2.5.1) | SHOULD | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-6` | A replay window size of 64 is preferred and SHOULD be employed as the default (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-7` | The replay check SHOULD be the first AH check applied to a packet once it is matched to an SA (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-8` | When an SA establishment protocol is in use, a receiver that will not provide anti-replay protection SHOULD say so during SA establishment (§3.4.3) | SHOULD | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-3` | A compliant implementation SHOULD NOT provide the anti-replay service with an SA that is manually keyed (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-2.4-7` | An implementation MAY choose any method to accelerate the SAD search (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.3.4-2` | An AH implementation MAY choose not to support fragmentation and MAY mark transmitted packets with the DF bit (§3.3.4) | MAY | 3.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-3.4.3-9` | A replay window larger than the minimum MAY be chosen by the receiver (§3.4.3) | MAY | 3.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4302-5-4` | Algorithms beyond those mandated for AH MAY be supported (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4302-2-1`](#rfc4302-2-1) The protocol header immediately preceding the AH header SHALL contain the value 51 in its Protocol or Next Header field (§2) | no test | no test carries this requirement id |
| [`RFC4302-2-2`](#rfc4302-2-2) AH carries no version number, so backward-compatibility concerns MUST be addressed by a signaling mechanism between the two peers, an SA management protocol or an out-of-band configuration mechanism (§2) | no test | no test carries this requirement id |
| [`RFC4302-2.3-1`](#rfc4302-2.3-1) The RESERVED field MUST be set to zero by the sender (§2.3) | no test | no test carries this requirement id |
| [`RFC4302-2.4-1`](#rfc4302-2.4-1) The SPI field is mandatory, and the mechanism for mapping inbound traffic to unicast SAs MUST be supported by all AH implementations (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.4-2`](#rfc4302-2.4-2) An implementation that supports multicast MUST support multicast SAs using the SAD search order this section gives (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.4-3`](#rfc4302-2.4-3) A multicast-capable implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.4-4`](#rfc4302-2.4-4) An accelerated SAD search MUST be functionally equivalent, in externally visible behavior, to searching the SAD in the order given (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.4-5`](#rfc4302-2.4-5) Whether source and destination address matching is required to map inbound traffic to an SA MUST be set as a side effect of manual SA configuration or by an SA management protocol (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.4-6`](#rfc4302-2.4-6) The SPI value zero is reserved for local implementation-specific use and MUST NOT be sent on the wire (§2.4) | no test | no test carries this requirement id |
| [`RFC4302-2.5-1`](#rfc4302-2.5-1) For a unicast SA or a single-sender multicast SA the sender MUST increment the Sequence Number for every transmitted packet (§2.5) | no test | no test carries this requirement id |
| [`RFC4302-2.5-2`](#rfc4302-2.5-2) The Sequence Number field is mandatory and MUST always be present and transmitted, even when the receiver does not enable anti-replay (§2.5) | no test | no test carries this requirement id |
| [`RFC4302-2.5-3`](#rfc4302-2.5-3) All AH implementations MUST be capable of performing the sequence number generation and the sequence number verification this document describes (§2.5) | no test | no test carries this requirement id |
| [`RFC4302-2.5-5`](#rfc4302-2.5-5) The sender's and receiver's counters MUST be reset, by establishing a new SA and a new key, before the 2^32nd packet is transmitted on an SA (§2.5) | no test | no test carries this requirement id |
| [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1) Use of an Extended Sequence Number MUST be negotiated by an SA management protocol (§2.5.1) | no test | no test carries this requirement id |
| [`RFC4302-2.6-1`](#rfc4302-2.6-1) All implementations MUST support explicit ICV padding and MUST insert only enough padding to satisfy the IPv4 or IPv6 alignment requirement (§2.6) | no test | no test carries this requirement id |
| [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1) The sender MUST NOT send a packet on an SA if doing so would cause the Sequence Number to cycle (§3.3.2) | no test | no test carries this requirement id |
| [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1) For the ICV computation a field that may be modified in transit is set to zero, a mutable but predictable field is set to the value it will have at the receiver, and the ICV field itself is set to zero (§3.3.3.1) | no test | no test carries this requirement id |
| [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1) When the packet length does not match the integrity algorithm's blocksize, implicit padding MUST be appended to the end of the packet before the ICV is computed (§3.3.3.2.2) | no test | no test carries this requirement id |
| [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2) The implicit padding octets MUST have a value of zero (§3.3.3.2.2) | no test | no test carries this requirement id |
| [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3) The document defining the integrity algorithm MUST be consulted to decide whether implicit padding is required (§3.3.3.2.2) | no test | no test carries this requirement id |
| [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4) When that document gives no answer, implicit padding is assumed to be required and its octets MUST have a value of zero (§3.3.3.2.2) | no test | no test carries this requirement id |
| [`RFC4302-3.3.4-1`](#rfc4302-3.3.4-1) An AH implementation MUST support generation of ICMP PMTU messages, or the equivalent internal signaling for a native host implementation (§3.3.4) | no test | no test carries this requirement id |
| [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1) A packet offered to AH that appears to be an IP fragment MUST be discarded, and that is an auditable event (§3.4.1) | no test | no test carries this requirement id |
| [`RFC4302-3.4.2-1`](#rfc4302-3.4.2-1) When no valid Security Association exists for a packet the receiver MUST discard it, and that is an auditable event (§3.4.2) | no test | no test carries this requirement id |
| [`RFC4302-3.4.3-1`](#rfc4302-3.4.3-1) All AH implementations MUST support the anti-replay service, whose use the receiver enables or disables per SA (§3.4.3) | no test | no test carries this requirement id |
| [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2) When anti-replay is enabled for an SA the receive packet counter MUST be initialized to zero as the SA is established (§3.4.3) | no test | no test carries this requirement id |
| [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3) For each received packet the receiver MUST verify that the Sequence Number does not duplicate one already received on this SA (§3.4.3) | no test | no test carries this requirement id |
| [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4) When ICV validation fails the receiver MUST discard the datagram as invalid (§3.4.3) | no test | no test carries this requirement id |
| [`RFC4302-3.4.3-5`](#rfc4302-3.4.3-5) A minimum replay window size of 32 packets MUST be supported (§3.4.3) | no test | no test carries this requirement id |
| [`RFC4302-4-1`](#rfc4302-4-1) When AH is incorporated into a system that supports auditing, the AH implementation MUST also support auditing and MUST allow an administrator to enable or disable auditing for AH (§4) | no test | no test carries this requirement id |
| [`RFC4302-5-1`](#rfc4302-5-1) An implementation claiming conformance MUST fully implement the AH syntax and processing for unicast traffic, and MUST comply with all requirements of the Security Architecture document (§5) | no test | no test carries this requirement id |
| [`RFC4302-5-2`](#rfc4302-5-2) An implementation claiming to support multicast traffic MUST comply with the additional requirements specified for such traffic (§5) | no test | no test carries this requirement id |
| [`RFC4302-A2-1`](#rfc4302-a2-1) When the IP implementation leaves a Fragmentation Extension Header in place after reassembly, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2) | no test | no test carries this requirement id |
| [`RFC4302-A2-2`](#rfc4302-a2-2) When the IP implementation hands AH a send-side packet carrying a Fragmentation Extension Header with Offset zero and More Fragments zero, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4302-2-1`](#rfc4302-2-1)

The protocol header immediately preceding the AH header SHALL contain the value 51 in its Protocol or Next Header field (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2-1, so no unit is bound to it.

### [`RFC4302-2-2`](#rfc4302-2-2)

AH carries no version number, so backward-compatibility concerns MUST be addressed by a signaling mechanism between the two peers, an SA management protocol or an out-of-band configuration mechanism (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2-2, so no unit is bound to it.

### [`RFC4302-2.3-1`](#rfc4302-2.3-1)

The RESERVED field MUST be set to zero by the sender (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.3-1, so no unit is bound to it.

### [`RFC4302-2.4-1`](#rfc4302-2.4-1)

The SPI field is mandatory, and the mechanism for mapping inbound traffic to unicast SAs MUST be supported by all AH implementations (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-1, so no unit is bound to it.

### [`RFC4302-2.4-2`](#rfc4302-2.4-2)

An implementation that supports multicast MUST support multicast SAs using the SAD search order this section gives (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-2, so no unit is bound to it.

### [`RFC4302-2.4-3`](#rfc4302-2.4-3)

A multicast-capable implementation MUST correctly de-multiplex inbound traffic even in the context of SPI collisions (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-3, so no unit is bound to it.

### [`RFC4302-2.4-4`](#rfc4302-2.4-4)

An accelerated SAD search MUST be functionally equivalent, in externally visible behavior, to searching the SAD in the order given (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-4, so no unit is bound to it.

### [`RFC4302-2.4-5`](#rfc4302-2.4-5)

Whether source and destination address matching is required to map inbound traffic to an SA MUST be set as a side effect of manual SA configuration or by an SA management protocol (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-5, so no unit is bound to it.

### [`RFC4302-2.4-6`](#rfc4302-2.4-6)

The SPI value zero is reserved for local implementation-specific use and MUST NOT be sent on the wire (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.4-6, so no unit is bound to it.

### [`RFC4302-2.5-1`](#rfc4302-2.5-1)

For a unicast SA or a single-sender multicast SA the sender MUST increment the Sequence Number for every transmitted packet (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-1, so no unit is bound to it.

### [`RFC4302-2.5-2`](#rfc4302-2.5-2)

The Sequence Number field is mandatory and MUST always be present and transmitted, even when the receiver does not enable anti-replay (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-2, so no unit is bound to it.

### [`RFC4302-2.5-3`](#rfc4302-2.5-3)

All AH implementations MUST be capable of performing the sequence number generation and the sequence number verification this document describes (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-3, so no unit is bound to it.

### [`RFC4302-2.5-5`](#rfc4302-2.5-5)

The sender's and receiver's counters MUST be reset, by establishing a new SA and a new key, before the 2^32nd packet is transmitted on an SA (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5-5, so no unit is bound to it.

### [`RFC4302-2.5.1-1`](#rfc4302-2.5.1-1)

Use of an Extended Sequence Number MUST be negotiated by an SA management protocol (§2.5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.5.1-1, so no unit is bound to it.

### [`RFC4302-2.6-1`](#rfc4302-2.6-1)

All implementations MUST support explicit ICV padding and MUST insert only enough padding to satisfy the IPv4 or IPv6 alignment requirement (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-2.6-1, so no unit is bound to it.

### [`RFC4302-3.3.2-1`](#rfc4302-3.3.2-1)

The sender MUST NOT send a packet on an SA if doing so would cause the Sequence Number to cycle (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.2-1, so no unit is bound to it.

### [`RFC4302-3.3.3.1-1`](#rfc4302-3.3.3.1-1)

For the ICV computation a field that may be modified in transit is set to zero, a mutable but predictable field is set to the value it will have at the receiver, and the ICV field itself is set to zero (§3.3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.1-1, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-1`](#rfc4302-3.3.3.2.2-1)

When the packet length does not match the integrity algorithm's blocksize, implicit padding MUST be appended to the end of the packet before the ICV is computed (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-1, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-2`](#rfc4302-3.3.3.2.2-2)

The implicit padding octets MUST have a value of zero (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-2, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-3`](#rfc4302-3.3.3.2.2-3)

The document defining the integrity algorithm MUST be consulted to decide whether implicit padding is required (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-3, so no unit is bound to it.

### [`RFC4302-3.3.3.2.2-4`](#rfc4302-3.3.3.2.2-4)

When that document gives no answer, implicit padding is assumed to be required and its octets MUST have a value of zero (§3.3.3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.3.2.2-4, so no unit is bound to it.

### [`RFC4302-3.3.4-1`](#rfc4302-3.3.4-1)

An AH implementation MUST support generation of ICMP PMTU messages, or the equivalent internal signaling for a native host implementation (§3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.3.4-1, so no unit is bound to it.

### [`RFC4302-3.4.1-1`](#rfc4302-3.4.1-1)

A packet offered to AH that appears to be an IP fragment MUST be discarded, and that is an auditable event (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.1-1, so no unit is bound to it.

### [`RFC4302-3.4.2-1`](#rfc4302-3.4.2-1)

When no valid Security Association exists for a packet the receiver MUST discard it, and that is an auditable event (§3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.2-1, so no unit is bound to it.

### [`RFC4302-3.4.3-1`](#rfc4302-3.4.3-1)

All AH implementations MUST support the anti-replay service, whose use the receiver enables or disables per SA (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-1, so no unit is bound to it.

### [`RFC4302-3.4.3-2`](#rfc4302-3.4.3-2)

When anti-replay is enabled for an SA the receive packet counter MUST be initialized to zero as the SA is established (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-2, so no unit is bound to it.

### [`RFC4302-3.4.3-3`](#rfc4302-3.4.3-3)

For each received packet the receiver MUST verify that the Sequence Number does not duplicate one already received on this SA (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-3, so no unit is bound to it.

### [`RFC4302-3.4.3-4`](#rfc4302-3.4.3-4)

When ICV validation fails the receiver MUST discard the datagram as invalid (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-4, so no unit is bound to it.

### [`RFC4302-3.4.3-5`](#rfc4302-3.4.3-5)

A minimum replay window size of 32 packets MUST be supported (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-3.4.3-5, so no unit is bound to it.

### [`RFC4302-4-1`](#rfc4302-4-1)

When AH is incorporated into a system that supports auditing, the AH implementation MUST also support auditing and MUST allow an administrator to enable or disable auditing for AH (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-4-1, so no unit is bound to it.

### [`RFC4302-5-1`](#rfc4302-5-1)

An implementation claiming conformance MUST fully implement the AH syntax and processing for unicast traffic, and MUST comply with all requirements of the Security Architecture document (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-5-1, so no unit is bound to it.

### [`RFC4302-5-2`](#rfc4302-5-2)

An implementation claiming to support multicast traffic MUST comply with the additional requirements specified for such traffic (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-5-2, so no unit is bound to it.

### [`RFC4302-A2-1`](#rfc4302-a2-1)

When the IP implementation leaves a Fragmentation Extension Header in place after reassembly, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-A2-1, so no unit is bound to it.

### [`RFC4302-A2-2`](#rfc4302-a2-2)

When the IP implementation hands AH a send-side packet carrying a Fragmentation Extension Header with Offset zero and More Fragments zero, AH MUST remove or skip it and repair the preceding Next Header before ICV processing (§A2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4302-A2-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude-opus-5 (ipsecwalk), spec-rfcgate-6-supported-extraction-signoff |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc4302.txt |
| Source fingerprint | 9966a2ddb94e62b7 |
| Record | rfc/extraction/rfc4302.json |
| Mapped sentences | 33 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Status of This Memo, Copyright Notice, Abstract and Table of Contents. |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 1 | walked | not stated |
| `2.4` | not stated | 6 | walked | not stated |
| `2.5` | not stated | 5 | walked | not stated |
| `2.5.1` | not stated | 1 | walked | not stated |
| `2.6` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.3.3` | not stated | 0 | walked | not stated |
| `3.3.3.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.1.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.1` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.2` | not stated | 0 | walked | not stated |
| `3.3.3.1.2.3` | not stated | 0 | walked | not stated |
| `3.3.3.2` | not stated | 0 | walked | not stated |
| `3.3.3.2.1` | not stated | 0 | walked | not stated |
| `3.3.3.2.2` | not stated | 4 | walked | not stated |
| `3.3.4` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 1 | walked | not stated |
| `3.4.2` | not stated | 1 | walked | not stated |
| `3.4.3` | not stated | 5 | walked | not stated |
| `3.4.4` | not stated | 1 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | skipped (appendix-non-normative) | Security Considerations: it discusses the assurance AH provides and states no obligation the extractor or this reviewer could find. |
| `7` | not stated | 0 | skipped (appendix-non-normative) | Differences from RFC 2402: a change log against the document this one obsoletes. |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `9` | References | 0 | skipped (references) | References. |
| `9.1` | Normative References | 0 | skipped (references) | Normative References. |
| `9.2` | not stated | 2 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the obligation site 2.5:2 already carries: the Sequence Number field is mandatory and the sender always transmits it. The clause after the comma, that the receiver need not act upon it, is a permission rather than an obligation and site 3.4.3:1 carries the receiver's own rule. | Thus, the sender MUST always transmit this field, but the receiver need not act upon it. |
| `2.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an integrity-algorithm specification, and the sentence obliges that document to state the ICV length and the validation rules. Ze publishes no integrity-algorithm specification and no producer in this repository could: it CONSUMES them, and ipsecAuthKeyLen (internal/plugins/ospf/config.go) plus xfrmAuthTruncLen (internal/component/ike/dataplane/xfrm_linux.go) hold the key length and the ICV truncation RFC 4868 already specified for HMAC-SHA-256-128, SHA-384-192 and SHA-512-256. The obligation the sentence places on ze as a reader of such a document is site 3.3.3.2.2:3, which is mapped. | The integrity algorithm specification MUST specify the length of the ICV and the comparison rules and processing steps for validation. |
| `3.4.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the discard obligation site 3.4.3:4 carries. Both sentences say that a failed ICV comparison discards the datagram as invalid; this one is the Integrity Check Value Verification section repeating the Sequence Number Verification section. | If the test fails, then the receiver MUST discard the received IP datagram as invalid. |

## Superseded

No document obsoletes RFC 4302, so its obligations are stated where they were written.
