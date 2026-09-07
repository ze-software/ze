# RFC 2784 - Generic Routing Encapsulation (GRE)

No row in the public ledger. Every requirement this repository extracted from RFC 2784, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 11 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2784.md` |
| Requirement shard | `rfc/requirements/rfc2784.md` |
| RFC text | `rfc/full/rfc2784.txt` |

## Enrolment

Enrolled: Generic Routing Encapsulation (GRE) base header: eleven MUST-level requirements, all {not-applicable}. ze builds and parses no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go buildGretun sets only the netlink link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go), delegating all header construction (C bit, Reserved0/Reserved1 zeroing, version 0, protocol type 0x0800), checksum handling, reserved-bit discard, and decapsulation/forwarding (destination lookup, TTL decrement, loop discard) to the kernel ip_gre module and the VPP dataplane. This is the same delegation rationale as the enrolled RFC 2890.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2784.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (11):** [`RFC2784-2.2-1`](#rfc2784-2.2-1), [`RFC2784-2.3-1`](#rfc2784-2.3-1), [`RFC2784-2.3-2`](#rfc2784-2.3-2), [`RFC2784-2.3-3`](#rfc2784-2.3-3), [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1), [`RFC2784-2.6-1`](#rfc2784-2.6-1), [`RFC2784-3-1`](#rfc2784-3-1), [`RFC2784-3.1-1`](#rfc2784-3.1-1), [`RFC2784-3.1-2`](#rfc2784-3.1-2), [`RFC2784-3.1-3`](#rfc2784-3.1-3), [`RFC2784-5.2-1`](#rfc2784-5.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2784-2.2-1` | A compliant implementation MUST accept and process the Checksum field when present (C bit set) (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| `RFC2784-2.3-1` | Receiver MUST discard a packet where any of bits 1-5 of Reserved0 are non-zero, unless the receiver implements RFC 1701 (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| `RFC2784-2.3-2` | Reserved0 bits 6-12 MUST be sent as zero (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| `RFC2784-2.3-3` | Reserved0 bits 6-12 MUST be ignored on receipt (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| `RFC2784-2.3.1-1` | Version Number field MUST contain the value zero (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| `RFC2784-2.6-1` | Reserved1 field, if present, MUST be transmitted as zero (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| `RFC2784-3-1` | When IPv4 is the payload, Protocol Type field MUST be set to 0x0800 (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| `RFC2784-3.1-1` | When decapsulating IPv4 payload, the destination address in the IPv4 payload header MUST be used to forward the packet (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| `RFC2784-3.1-2` | TTL of the decapsulated IPv4 payload packet MUST be decremented (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| `RFC2784-3.1-3` | If decapsulated IPv4 payload destination address is the encapsulator (loop detected), the packet MUST be discarded (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| `RFC2784-5.2-1` | Packets from an RFC 1701 transmitter with non-zero bits in bits 1-5 MUST be discarded unless the receiver implements RFC 1701 (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| `RFC2784-2.4-1` | An implementation receiving a Protocol Type not listed in RFC 1700 or ETYPES SHOULD discard the packet (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2784-3.1-4` | Care should be taken when forwarding decapsulated payload to avoid routing loops (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2784-5-1` | Implementations MAY support RFC 1701 features (Routing, Key, Sequence) but MUST also accept packets without them (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2784-2.2-1`](#rfc2784-2.2-1) A compliant implementation MUST accept and process the Checksum field when present (C bit set) (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| [`RFC2784-2.3-1`](#rfc2784-2.3-1) Receiver MUST discard a packet where any of bits 1-5 of Reserved0 are non-zero, unless the receiver implements RFC 1701 (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| [`RFC2784-2.3-2`](#rfc2784-2.3-2) Reserved0 bits 6-12 MUST be sent as zero (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| [`RFC2784-2.3-3`](#rfc2784-2.3-3) Reserved0 bits 6-12 MUST be ignored on receipt (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |
| [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1) Version Number field MUST contain the value zero (§2.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| [`RFC2784-2.6-1`](#rfc2784-2.6-1) Reserved1 field, if present, MUST be transmitted as zero (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| [`RFC2784-3-1`](#rfc2784-3-1) When IPv4 is the payload, Protocol Type field MUST be set to 0x0800 (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this header-construction obligation has no ze code path |
| [`RFC2784-3.1-1`](#rfc2784-3.1-1) When decapsulating IPv4 payload, the destination address in the IPv4 payload header MUST be used to forward the packet (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| [`RFC2784-3.1-2`](#rfc2784-3.1-2) TTL of the decapsulated IPv4 payload packet MUST be decremented (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| [`RFC2784-3.1-3`](#rfc2784-3.1-3) If decapsulated IPv4 payload destination address is the encapsulator (loop detected), the packet MUST be discarded (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this packet-forwarding/decapsulation obligation has no ze code path |
| [`RFC2784-5.2-1`](#rfc2784-5.2-1) Packets from an RFC 1701 transmitter with non-zero bits in bits 1-5 MUST be discarded unless the receiver implements RFC 1701 (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header: it programs kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go:129 buildGretun sets only the netlink.Gretun link descriptor) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:73), and the kernel ip_gre module and VPP dataplane own the GRE header wire bits and all decapsulation/forwarding, so this receive-side obligation has no ze code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2784-2.2-1`](#rfc2784-2.2-1)

A compliant implementation MUST accept and process the Checksum field when present (C bit set) (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.2-1, so no unit is bound to it.

### [`RFC2784-2.3-1`](#rfc2784-2.3-1)

Receiver MUST discard a packet where any of bits 1-5 of Reserved0 are non-zero, unless the receiver implements RFC 1701 (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-1, so no unit is bound to it.

### [`RFC2784-2.3-2`](#rfc2784-2.3-2)

Reserved0 bits 6-12 MUST be sent as zero (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-2, so no unit is bound to it.

### [`RFC2784-2.3-3`](#rfc2784-2.3-3)

Reserved0 bits 6-12 MUST be ignored on receipt (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3-3, so no unit is bound to it.

### [`RFC2784-2.3.1-1`](#rfc2784-2.3.1-1)

Version Number field MUST contain the value zero (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.3.1-1, so no unit is bound to it.

### [`RFC2784-2.6-1`](#rfc2784-2.6-1)

Reserved1 field, if present, MUST be transmitted as zero (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-2.6-1, so no unit is bound to it.

### [`RFC2784-3-1`](#rfc2784-3-1)

When IPv4 is the payload, Protocol Type field MUST be set to 0x0800 (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3-1, so no unit is bound to it.

### [`RFC2784-3.1-1`](#rfc2784-3.1-1)

When decapsulating IPv4 payload, the destination address in the IPv4 payload header MUST be used to forward the packet (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-1, so no unit is bound to it.

### [`RFC2784-3.1-2`](#rfc2784-3.1-2)

TTL of the decapsulated IPv4 payload packet MUST be decremented (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-2, so no unit is bound to it.

### [`RFC2784-3.1-3`](#rfc2784-3.1-3)

If decapsulated IPv4 payload destination address is the encapsulator (loop detected), the packet MUST be discarded (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-3.1-3, so no unit is bound to it.

### [`RFC2784-5.2-1`](#rfc2784-5.2-1)

Packets from an RFC 1701 transmitter with non-zero bits in bits 1-5 MUST be discarded unless the receiver implements RFC 1701 (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2784-5.2-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2784, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2784, so its obligations are stated where they were written.
