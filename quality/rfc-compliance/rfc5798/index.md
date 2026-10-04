# RFC 5798 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

Partial. Every requirement this repository extracted from RFC 5798, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 72.9% | 43 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 6.8% | 4 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 59 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 99.3% | 146 of 147 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 59 | of 85 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 5.1% | 3 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 3.4% | 2 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11.9% | 7 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 85 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 7 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 147 |
| Tagged units | 147 |
| Recorded audit verdicts | 47 |
| Discrimination records | 146 |
| Summary | `rfc/short/rfc5798.md` |
| Requirement shard | `rfc/requirements/rfc5798.md` |
| RFC text | `rfc/full/rfc5798.txt` |

## Enrolment

Enrolled: VRRP Version 3 for IPv4 and IPv6 (RFC 5798, obsoleted by RFC 9568): the VRRPv3 document ze actually speaks on the IPv4 wire. Every one of its 59 gated requirements carries a {superseded} marker naming where the obligation lives in RFC 9568, and 55 of them are restated there unchanged, so ze implements them through the same producers rfc/short/rfc9568.md gates: internal/plugins/vrrp/packet (advert encode/decode, checksum), internal/plugins/vrrp/fsm (Initialize/Backup/Master), internal/plugins/vrrp/transport (proto 112, GTSM, GARP/NA). The row that is NOT a restatement is RFC5798-5.2.8-1: Section 5.2.8 puts a pseudo-header under the checksum for both families, RFC 9568 Section 5.2.8 removes it for IPv4, and ze transmits this document form because keepalived and the pre-RFC-9568 base require it (pseudoSumV4Legacy and FillChecksum, internal/plugins/vrrp/packet/checksum.go). Tagged under RFC5798 ids: 43 of the 59 gated MUSTs carry a positive and a negative test, 4 carry a positive test and a `{single-polarity}` annotation because the producer writes a constant no input changes, 3 are met below ze by the Linux forwarding and SLAAC paths, 2 are conditional on an optional feature ze declined, and 7 are `{gap}` rows ze owes.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- For IPv4, ze transmits the RFC 5798 pseudo-header checksum form, because that is what keepalived (proven on the wire: its own adverts use it) and the rest of the deployed base compute and require
- a message-only advert is rejected by them as "Invalid VRRPv3 checksum". On receive, ze dual-accepts both this form and the RFC 9568 message-only form.


**What the ledger says remains**

RFC 9568 Section 5.2.8 clarifies the IPv4 checksum as message-only (no pseudo-header); ze diverges from that clarification on transmit for interoperability, and counts message-only senders (`checksum-rfc9568-message-only`) so the strict-RFC-9568 population is visible. When that population dominates, the transmit form can be revisited.

- **Enrolled 2026-09-01:** [`rfc/short/rfc5798.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5798.md) declares 85 requirements, 59 of them MUST-level, and every one carries a `{superseded}` marker naming where RFC 9568 states it. 43 of those 59 are proven under RFC5798 ids in both polarities. 4 carry a positive test and a `{single-polarity}` annotation, because the producer writes a constant no input changes: [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1), [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1), [`RFC5798-7.2-2`](#rfc5798-7.2-2) and [`RFC5798-7.2-4`](#rfc5798-7.2-4). [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1) is the weakest of those four, because its only evidence is an integration-gated test the privileged QEMU suite runs and `./le rfc discriminate-record` cannot reach. 3 are met below ze and carry a `{lower-layer}` annotation: [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1) and [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1), where Linux consumes a link-local scope multicast datagram rather than forwarding it, and [`RFC5798-7.4-1`](#rfc5798-7.4-1), where Linux derives each device's Interface Identifier. 2 are conditional on an optional feature ze declined and carry a `{feature-declined}` annotation: [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1), the VRRPv2/VRRPv3 dual-send flag RFC 5798 Section 8.4.2 makes a MAY, and RFC5798-A.2-1, the Token Ring functional-address mode. 7 are `{gap}` rows ze owes: [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4) and [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) (no ND Router Advertisement is sent for a virtual router, so there is no option set to configure either), [`RFC5798-7.4-2`](#rfc5798-7.4-2) (the virtual-MAC macvlan carries no `addr_gen_mode`, so Linux derives its link-local Interface Identifier from the virtual router MAC), [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) (a proxy ARP reply on a VRRP router does not carry the virtual router MAC), [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) and [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) (ze authors no Neighbor Solicitation for a host and installs nothing that sources one from the virtual MAC), and [`RFC5798-7.1-3`](#rfc5798-7.1-3), where Section 7.1 makes the receiver discard an advertisement when the local router is the IPvX address owner and `Decode` ([`internal/plugins/vrrp/packet/validate.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate.go)) verifies only the VRID half, because RFC 9568 erratum 8298 lowers the address-owner half to a log. One RFC 5798 obligation is NOT a restatement: [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1) puts a pseudo-header under the checksum for both address families, which is the divergence this row describes. RFC 5798 Section 5.2.8 cites the pseudo-header "as defined in Section 8.1 of [RFC2460]", an IPv6-only shape, so what ze and the deployed base compute for IPv4 is the classic IPv4 pseudo-header (`pseudoSumV4Legacy`, [`internal/plugins/vrrp/packet/checksum.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum.go)) rather than the shape that sentence names.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 43 | one part of the gated population |
| Annotated (including scoped evidence) | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (43):** [`RFC5798-5.1.1.3-2`](#rfc5798-5.1.1.3-2), [`RFC5798-5.1.2.3-2`](#rfc5798-5.1.2.3-2), [`RFC5798-5.2.2-1`](#rfc5798-5.2.2-1), [`RFC5798-5.2.4-1`](#rfc5798-5.2.4-1), [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2), [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1), [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1), [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1), [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2), [`RFC5798-6.1-1`](#rfc5798-6.1-1), [`RFC5798-6.4.2-1`](#rfc5798-6.4.2-1), [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2), [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3), [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4), [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5), [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6), [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7), [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8), [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9), [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10), [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1), [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2), [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3), [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5), [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6), [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7), [`RFC5798-6.4.3-8`](#rfc5798-6.4.3-8), [`RFC5798-6.4.3-9`](#rfc5798-6.4.3-9), [`RFC5798-6.4.3-10`](#rfc5798-6.4.3-10), [`RFC5798-6.4.3-11`](#rfc5798-6.4.3-11), [`RFC5798-6.4.3-12`](#rfc5798-6.4.3-12), [`RFC5798-7.1-1`](#rfc5798-7.1-1), [`RFC5798-7.1-2`](#rfc5798-7.1-2), [`RFC5798-7.1-4`](#rfc5798-7.1-4), [`RFC5798-7.2-1`](#rfc5798-7.2-1), [`RFC5798-7.2-3`](#rfc5798-7.2-3), [`RFC5798-8.1.2-1`](#rfc5798-8.1.2-1), [`RFC5798-8.1.2-6`](#rfc5798-8.1.2-6), [`RFC5798-8.1.1-2`](#rfc5798-8.1.1-2), [`RFC5798-8.2.1-2`](#rfc5798-8.2.1-2), [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1), [`RFC5798-8.2.2-8`](#rfc5798-8.2.2-8), [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4)

**Annotated (including scoped evidence) (16):** [`RFC5798-5.1.1.2-1`](#rfc5798-5.1.1.2-1), [`RFC5798-5.1.1.3-1`](#rfc5798-5.1.1.3-1), [`RFC5798-5.1.2.2-1`](#rfc5798-5.1.2.2-1), [`RFC5798-5.1.2.3-1`](#rfc5798-5.1.2.3-1), [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4), [`RFC5798-7.1-3`](#rfc5798-7.1-3), [`RFC5798-7.2-2`](#rfc5798-7.2-2), [`RFC5798-7.2-4`](#rfc5798-7.2-4), [`RFC5798-7.4-1`](#rfc5798-7.4-1), [`RFC5798-7.4-2`](#rfc5798-7.4-2), [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1), [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2), [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3), [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1), [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1), [`RFC5798-A.2-1`](#rfc5798-a.2-1)

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
| `RFC5798-5.2.4-1` | The priority value for the VRRP router that owns the IPvX address associated with the virtual router MUST be 255 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L784). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L786) |
| `RFC5798-5.2.4-2` | VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319). **positive:** `unit/verify` [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1594). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) |
| `RFC5798-5.2.6-1` | This field MUST be set to zero on transmission and ignored on reception. (§5.2.6) | MUST | 5.2.6 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107). **positive:** `unit/verify` [`TestEncodeV3ReserveZeroOverDirtyBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L192). **negative:** `unit/verify` [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L639) |
| `RFC5798-5.2.8-1` | The checksum is the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]. The next header field in the "pseudo-header" should be set to 112 (decimal) for VRRP. For computing the checksum, the checksum field is set to zero. (§5.2.8, §7.1) | MUST | 5.2.8 | **positive:** `unit/verify` [`TestFillChecksumFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L68). **positive:** `unit/verify` [`TestFillChecksumZeroesFieldAndCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L233). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L663). **negative:** `unit/verify` [`TestFillChecksumZeroesFieldAndCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L234) |
| `RFC5798-5.2.9-1` | For IPv6, the first address must be the IPv6 link-local address associated with the virtual router. (§5.2.9, §6.1; lowercase "must" in the RFC) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestAdvertParamsIPv6LeadWithLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc5798_tx_order_test.go#L29). **positive:** `unit/verify` [`TestSendAdvertIPv6LeadsWithLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc5798_linklocal_first_test.go#L27). **positive:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L524). **negative:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L526) |
| `RFC5798-5.2.9-2` | This field contains either one or more IPv4 addresses, or one or more IPv6 addresses, that is, IPv4 and IPv6 MUST NOT both be carried in one IPvX Address field. (§5.2.9) | MUST NOT | 5.2.9 | **positive:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L548). **negative:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L550) |
| `RFC5798-6.1-1` | Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L110). **negative:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L112) |
| `RFC5798-6.4.2-1` | (310) + MUST NOT respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L399). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L436) |
| `RFC5798-6.4.2-2` | (320) + MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L720). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L722) |
| `RFC5798-6.4.2-3` | (325) + MUST NOT send ND Router Advertisement messages for the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1115). **negative:** `unit/verify` [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1116) |
| `RFC5798-6.4.2-4` | (335) - MUST discard packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L109). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L123). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L110). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L124) |
| `RFC5798-6.4.2-5` | (340) - MUST NOT accept packets addressed to the IPvX address(es) associated with the virtual router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L401). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L438) |
| `RFC5798-6.4.2-6` | If a Shutdown event is received, then: (350) + Cancel the Master_Down_Timer (355) + Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) |
| `RFC5798-6.4.2-7` | If the Master_Down_Timer fires, then: (370) + Send an ADVERTISEMENT (375) + If the protected IPvX address is an IPv4 address, then: (380) * Broadcast a gratuitous ARP request on that interface containing the virtual router MAC address for each IPv4 address associated with the virtual router. (385) + else // ipv6 (390) * Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the virtual router. (395) * For each IPv6 address associated with the virtual router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) unset, the Override flag (O) set, the target address set to the IPv6 address of the virtual router, and the target link-layer address set to the virtual router MAC address. (400) +endif // was protected addr ipv4? (405) + Set the Adver_Timer to Advertisement_Interval (410) + Transition to the {Master} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L52). **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L448). **positive:** `unit/verify` [`TestPromotionInstallsIPv6AddressesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_v6_test.go#L29). **positive:** `unit/verify` [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L88). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L639) |
| `RFC5798-6.4.2-8` | If the Priority in the ADVERTISEMENT is zero, then: (430) * Set the Master_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) |
| `RFC5798-6.4.2-9` | If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: (450) @ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (455) @ Recompute the Master_Down_Interval (460) @ Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118). **positive:** `unit/verify` [`TestV3BackupAdoptsIntervalOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L292). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120). **negative:** `unit/verify` [`TestV3BackupAdoptsIntervalOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L293) |
| `RFC5798-6.4.2-10` | else // preempt was true or priority was less (470) @ Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124) |
| `RFC5798-6.4.3-1` | (610) + MUST respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L72). **positive:** `unit/verify` [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L29). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L73). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L139). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L74) |
| `RFC5798-6.4.3-2` | (620) + MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L724). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L728) |
| `RFC5798-6.4.3-3` | (625) + MUST respond to ND Neighbor Solicitation message for the IPv6 address(es) associated with the virtual router. (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L729). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L730) |
| `RFC5798-6.4.3-4` | (630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs IPv6 virtual routers and sends no Router Advertisement for them. The announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136, built by BuildNA internal/plugins/vrrp/transport/na.go and selected by frameBuilder internal/plugins/vrrp/transport/transport.go, and no Router Advertisement builder and no RA send path exists under internal/plugins/vrrp |
| `RFC5798-6.4.3-5` | (645) - MUST forward packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L115). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L129). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L116). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L130) |
| `RFC5798-6.4.3-6` | (650) - MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L269). **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L242). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L206) |
| `RFC5798-6.4.3-7` | MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, MUST NOT accept these packets. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L204). **negative:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L270). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L244) |
| `RFC5798-6.4.3-8` | If a Shutdown event is received, then: (660) + Cancel the Adver_Timer (665) + Send an ADVERTISEMENT with Priority = 0 (670) + Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L126). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L128) |
| `RFC5798-6.4.3-9` | If the Adver_Timer fires, then: (685) + Send an ADVERTISEMENT (690) + Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L130). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L641) |
| `RFC5798-6.4.3-10` | If the Priority in the ADVERTISEMENT is zero, then: (710) -* Send an ADVERTISEMENT (715) -* Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L132). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L134) |
| `RFC5798-6.4.3-11` | If the Priority in the ADVERTISEMENT is greater than the local Priority, (730) -* or (735) -* If the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX Address of the sender is greater than the local primary IPvX Address, then: (740) -@ Cancel Adver_Timer (745) -@ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (750) -@ Recompute the Skew_Time (755) @ Recompute the Master_Down_Interval (760) @ Set Master_Down_Timer to Master_Down_Interval (765) @ Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L136). **positive:** `unit/verify` [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L51). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L78). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L138). **negative:** `unit/verify` [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L52). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L79) |
| `RFC5798-6.4.3-12` | else // new Master logic (775) @ Discard ADVERTISEMENT (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L140). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L142) |
| `RFC5798-7.1-1` | - MUST verify that the VRRP version is 3. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L611) |
| `RFC5798-7.1-2` | - MUST verify that the received packet contains the complete VRRP packet (including fixed fields, and IPvX address). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L83). **positive:** `unit/verify` [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L148). **negative:** `unit/verify` [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L149). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L511) |
| `RFC5798-7.1-3` | - MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Decode internal/plugins/vrrp/packet/validate.go verifies the VRID half of this check and ze implements no address-owner half, so an advertisement arriving for a VRID whose address the local router owns is processed rather than discarded. RFC 9568 erratum 8298 lowers that half to a SHOULD that logs and ze follows the successor, so the RFC 5798 MUST as published is unmet |
| `RFC5798-7.1-4` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L104). **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L555). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L530). **negative:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L103) |
| `RFC5798-7.2-1` | The following operations MUST be performed when transmitting a VRRP packet: - Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109). **positive:** `unit/verify` [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L33). **positive:** `unit/verify` [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L37). **negative:** `unit/verify` [`TestBoundaryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L292) |
| `RFC5798-7.2-2` | If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L390). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L40). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L100). **negative:** no negative test. **{single-polarity}:** the source link-layer address is the virtual router MAC that packet.VirtualMAC internal/plugins/vrrp/packet/packet.go derives from the family and the VRID, put on the wire by binding the tx socket to the macvlan created with that address (openV4 and openV6 internal/plugins/vrrp/transport/backend_linux.go), a derivation no input can make yield another MAC |
| `RFC5798-7.2-3` | If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address + Set the source IPv6 address to interface link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L43). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L102). **negative:** `unit/verify` [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L408). **negative:** `unit/verify` [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L360) |
| `RFC5798-7.2-4` | Set the IPvX protocol to VRRP - Send the VRRP packet to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L317). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L46). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L104). **negative:** no negative test. **{single-polarity}:** buildIPv4Header internal/plugins/vrrp/transport/transport.go writes the constant packet.ProtoNumber into the protocol octet and SendAdvert internal/plugins/vrrp/transport/backend_linux.go targets the constants packet.MulticastV4 and packet.MulticastV6, so no configuration reaches another protocol number or another destination |
| `RFC5798-7.4-1` | IPv6 routers running VRRP MUST create their Interface Identifiers in the normal manner (e.g., "Transmission of IPv6 Packets over Ethernet Networks" [RFC2464]). (§7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux SLAAC; internal/plugins/vrrp/dataplane_linux.go::applyDataplaneSysctls is the only IPv6 state ze writes on a VRRP device and it writes accept_dad alone, while Linux derives each device's Interface Identifier from that device's MAC in the RFC 2464 manner this sentence names, so no value ze writes decides the derivation |
| `RFC5798-7.4-2` | They MUST NOT use the virtual router MAC address to create the Modified Extended Unique Identifier (EUI)-64 identifiers. (§7.4) | MUST NOT | 7.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze creates the per-group macvlan carrying the virtual router MAC and writes no addr_gen_mode on it, because applyDataplaneSysctls internal/plugins/vrrp/dataplane_linux.go writes accept_dad and nothing else on an IPv6 group, so Linux derives that device's link-local Interface Identifier from the virtual router MAC, which is the derivation this sentence forbids. Suppressing it, by writing addr_gen_mode on the IPv6 macvlan, is unbuilt |
| `RFC5798-8.1.2-1` | The Virtual Router Master MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2) | MUST NOT | 8.1.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L74). **positive:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L176). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L61). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L44). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L141). **negative:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L177). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L62). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L45) |
| `RFC5798-8.1.2-6` | When a host sends an ARP request for one of the virtual router IPv4 addresses, the Virtual Router Master MUST respond to the ARP request with an ARP response that indicates the virtual MAC address for the virtual router. (§8.1.2) | MUST | 8.1.2 | **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L69). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L54). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L70). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L55) |
| `RFC5798-8.1.1-2` | If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1.1; lowercase "must" in the RFC) | MUST | 8.1.1 | **positive:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L22). **negative:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L23) |
| `RFC5798-8.2.1-2` | If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.2.1; lowercase "must" in the RFC) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L37). **negative:** `unit/verify` [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L38) |
| `RFC5798-8.1.3-1` | If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC) | MUST | 8.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze offers proxy ARP, as the builtin proxy sysctl profile that writes net.ipv4.conf.<iface>.proxy_arp=1 internal/core/sysctl/profiles.go, and nothing makes a proxy ARP reply on a VRRP router carry the virtual router MAC: the kernel answers from the replying device's own address and ze installs no virtual-MAC proxy state, so the obligation is unbuilt rather than out of reach |
| `RFC5798-8.2.2-1` | The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L726). **positive:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L172). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L33). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L48). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L727). **negative:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L173). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L34). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L49) |
| `RFC5798-8.2.2-8` | When a host sends an ND Neighbor Solicitation message for the virtual router IPv6 address, the Virtual Router Master MUST respond to the ND Neighbor Solicitation message with the virtual MAC address for the virtual router. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L37). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L60). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L38). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L61) |
| `RFC5798-8.2.2-2` | When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze authors no Neighbor Solicitation -- the plugin's only ICMPv6 builder is BuildNA, type 136, internal/plugins/vrrp/transport/na.go -- and it installs nothing that makes a host solicitation source from the virtual-MAC macvlan, which doInstallVIPs internal/plugins/vrrp/instance.go gives the virtual address alone. The solicitation a Master sends for a host therefore leaves the parent, carrying the parent's own link-layer address |
| `RFC5798-8.2.2-3` | It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same unbuilt path as RFC5798-8.2.2-2. Ze authors no Neighbor Solicitation and installs nothing that keeps the physical MAC out of the source link-layer address option of one its stack sends for a host |
| `RFC5798-8.2.2-4` | o At system boot, when initializing interfaces for VRRP operation, all ND Router and Neighbor Advertisements and Solicitation messages must be delayed until both the IPv6 address and the virtual router MAC address are configured. (§8.2.2; lowercase "must" in the RFC) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L38). **positive:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L117). **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L773). **negative:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L39). **negative:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L118). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L775) |
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
| `RFC5798-A.2-3` | For both the multicast and unicast mode of operation, VRRP advertisements sent to 224.0.0.18 should be encapsulated as described in [RFC1469]. (§A.2; lowercase "should" in the RFC) | SHOULD | A.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring"; Section A.2 recommends this encapsulation only for a router running VRRP over Token Ring, and ze supports VRRP over Ethernet only, so it never sends an advertisement onto a Token-Ring segment. internal/plugins/vrrp/packet/packet.go::VirtualMAC derives only the Ethernet 00-00-5E-00-01 and 00-00-5E-00-02 addresses and carries no Token-Ring mapping |
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
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L786) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L784) | unit/verify | revert, verified |

### [`RFC5798-5.2.4-2`](#rfc5798-5.2.4-2)

VRRP routers backing up a virtual router MUST use priority values between 1-254 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): config range: TestBoundaryPriority accepts 1 and 254 and refuses 0 and 255; run-time: TestEffectivePriorityWithTracking (now tagged here) requires exact values including the floor at 1 for decrements equal to and past the priority (254/254, 100/100, 100/4064), so removing the clamp goes red. Record: revert EffectivePriority.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) | unit/verify | revert, verified |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319) | unit/verify | revert, verified |
| positive | [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1594) | unit/verify | revert, verified |

### [`RFC5798-5.2.6-1`](#rfc5798-5.2.6-1)

This field MUST be set to zero on transmission and ignored on reception. (§5.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestEncodeGoldenV3IPv4 (body unchanged). TestEncodeV3ReserveZeroOverDirtyBuffer: WriteTo into an all-0xFF buffer writes the rsvd nibble 0 and the bytes equal the goldens; TestDecodeV3ReserveIgnoredOnReceive holds the reception half.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L639) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestEncodeV3ReserveZeroOverDirtyBuffer`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L192) | unit/verify | revert, verified |

### [`RFC5798-5.2.8-1`](#rfc5798-5.2.8-1)

The checksum is the 16-bit one's complement of the one's complement sum of the entire VRRP message starting with the version field and a "pseudo-header" as defined in Section 8.1 of [RFC2460]. The next header field in the "pseudo-header" should be set to 112 (decimal) for VRRP. For computing the checksum, the checksum field is set to zero. (§5.2.8, §7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestFillChecksumZeroesFieldAndCoversPseudoHeader: FillChecksum over a stale 0xABCD field reproduces the IPv4 and IPv6 goldens (field zeroed, pseudo-header covered); the IPv6 golden equals an independent RFC 1071 sum with next header 112; IPv6 adverts summed with next header 58 or without the pseudo-header are refused with ErrChecksum. Receive also accepts the IPv4 message-only form: RFC 9568 Section 5.2.8, which obsoletes this text, requires that form, so accepting it is owed under the forward lineage and is disclosed on the Support row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFillChecksumZeroesFieldAndCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L234) | unit/verify | revert, verified |
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L663) | unit/verify | revert, verified |
| positive | [`TestFillChecksumFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestFillChecksumZeroesFieldAndCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L233) | unit/verify | revert, verified |

### [`RFC5798-5.2.9-1`](#rfc5798-5.2.9-1)

For IPv6, the first address must be the IPv6 link-local address associated with the virtual router. (§5.2.9, §6.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuation 3): Transmit order now proven. TestSendAdvertIPv6LeadsWithLinkLocal reads the sent frame: octets 8-24 are fe80::1 and 24-40 are 2001:db8::1, an order netip sorting would invert (record observed red on encodeLocked); TestAdvertParamsIPv6LeadWithLinkLocal asserts every AdvertParams an IPv6 Master hands updateAdvert lists fe80::1 first (record on doSendAdvert). Negative: TestValidateIPv6LinkLocal refuses a group listing the global address first and accepts the swapped order, so the config keeps and checks the configured order (records on validateGroup).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L526) | unit/verify | revert, verified |
| positive | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L524) | unit/verify | revert, verified |
| positive | [`TestAdvertParamsIPv6LeadWithLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc5798_tx_order_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestSendAdvertIPv6LeadsWithLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc5798_linklocal_first_test.go#L27) | unit/verify | revert, verified |

### [`RFC5798-5.2.9-2`](#rfc5798-5.2.9-2)

This field contains either one or more IPv4 addresses, or one or more IPv6 addresses, that is, IPv4 and IPv6 MUST NOT both be carried in one IPvX Address field. (§5.2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestValidateVIPFamilyMatchesGroupFamily now isolates both mixed-family cases: the IPv4 group lists 192.0.2.1 then 2001:db8::1, the IPv6 group lists fe80::1 then 192.0.2.1 (so the first-address-link-local rule cannot refuse it), and each refusal must carry the family error text; same-family lists validate. Records: revert validateGroup, both polarities.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: Backup installs no VIP (parent carries arp_ignore=1, so nothing answers ARP for it); Master contrast installs it on the vMAC device

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L436) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L399) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-2`](#rfc5798-6.4.2-2)

(320) + MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup installs no IPv6 VIP, Master installs it on the vMAC macvlan only; NS answering follows the kernel address

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L722) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L720) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-3`](#rfc5798-6.4.2-3)

(325) + MUST NOT send ND Router Advertisement messages for the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Backup emits zero announcements; the Master's one announcement is BuildNA type 136, never 134

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1116) | unit/verify | revert, verified |
| positive | [`TestInstanceBackupSendsNoRouterAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L1115) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-4`](#rfc5798-6.4.2-4)

(335) - MUST discard packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuation 3): Wiring now proven on the host: TestBackupDiscardFollowsTheState drives the FSM and asserts the worker sets the discard on the virtual router MAC device at start (idle worker: on then off), entering Backup adds no call, promotion withdraws it once after the only install, and a higher-priority advert demotes and sets it again before the only removal (records observed red on doRemoveVIPs; the run revert was also observed red but the ledger keeps one record per unit/polarity). Wire effect: TestVRRPBackupDoesNotForwardVirtualMACFrames (QEMU guest) forwards a transit datagram sent to the Virtual Router MAC without the nft drop and stops it with it (records on backupFilterTables). The host unit's negative tag is a state contrast; the violating negative (no discard, frame forwarded) is the QEMU control.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L110) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L123) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-5`](#rfc5798-6.4.2-5)

(340) - MUST NOT accept packets addressed to the IPvX address(es) associated with the virtual router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: Backup installs no VIP so no packet to it is delivered locally; Master contrast installs it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L438) | unit/verify | revert, verified |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L401) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-6`](#rfc5798-6.4.2-6)

If a Shutdown event is received, then: (350) + Cancel the Master_Down_Timer (355) + Transition to the {Initialize} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. matrix row backup/shutdown asserts exactly StopTimers + EmitStateChange to Initialize; a non-Shutdown event leaves Backup

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L110) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-7`](#rfc5798-6.4.2-7)

If the Master_Down_Timer fires, then: (370) + Send an ADVERTISEMENT (375) + If the protected IPvX address is an IPv4 address, then: (380) * Broadcast a gratuitous ARP request on that interface containing the virtual router MAC address for each IPv4 address associated with the virtual router. (385) + else // ipv6 (390) * Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the virtual router. (395) * For each IPv6 address associated with the virtual router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) unset, the Override flag (O) set, the target address set to the IPv6 address of the virtual router, and the target link-layer address set to the virtual router MAC address. (400) +endif // was protected addr ipv4? (405) + Set the Adver_Timer to Advertisement_Interval (410) + Transition to the {Master} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): earlier proof stands (promotion action list, StartAdvertTimer at the own interval, GARP per IPv4 VIP with the virtual router MAC, NA per IPv6 VIP R=1 S=0 O=1 TLL 00-00-5e-00-02-0a, stale-generation negative). The Solicited-Node clause is now asserted at Ze's boundary: TestPromotionInstallsIPv6AddressesOnVirtualMACDevice requires nothing installed in Backup and, on the Master_Down_Timer, exactly fe80::1 and 2001:db8::1 installed on the virtual-MAC device, never the parent -- the address add from which Linux addrconf joins each Solicited-Node group (record: doInstallVIPs). Kernel membership itself is not read.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L639) | unit/verify | revert, verified |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L448) | unit/verify | revert, verified |
| positive | [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestPromotionInstallsIPv6AddressesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_v6_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L52) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-8`](#rfc5798-6.4.2-8)

If the Priority in the ADVERTISEMENT is zero, then: (430) * Set the Master_Down_Timer to Skew_Time (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Priority 0 advert arms StartMasterDownTimer(skewDur); non-zero arms mdDur

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L114) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-9`](#rfc5798-6.4.2-9)

If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: (450) @ Set Master_Adver_Interval to Adver Interval contained in the ADVERTISEMENT (455) @ Recompute the Master_Down_Interval (460) @ Reset the Master_Down_Timer to Master_Down_Interval (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestV3BackupAdoptsIntervalOrDiscards: equal, greater and Preempt-off lower adverts carrying 4 s each set Master_Adver_Interval to 4000 and arm the literal 14.4375 s; Preempt-on lower yields nothing and keeps 1000.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L120) | unit/verify | revert, verified |
| negative | [`TestV3BackupAdoptsIntervalOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L118) | unit/verify | revert, verified |
| positive | [`TestV3BackupAdoptsIntervalOrDiscards`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L292) | unit/verify | revert, verified |

### [`RFC5798-6.4.2-10`](#rfc5798-6.4.2-10)

else // preempt was true or priority was less (470) @ Discard the ADVERTISEMENT (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Preempt True and lower priority emits no actions; same advert with Preempt False adopts

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L122) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-1`](#rfc5798-6.4.3-1)

(610) + MUST respond to ARP requests for the IPv4 address(es) associated with the virtual router. (§6.4.3, §8.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent) re-judged: the tagged units changed only by added RFC requirement tag comment lines for the new rows (8.1.2-6 / 8.2.2-8 / 8.2-4 / 8.1.1-2); test bodies byte-identical, records current, verdict unchanged. Prior note: vrrp judge 2026-09-29 (independent, continuation 3): Product install now proven: TestPromotionInstallsIPv4AddressOnVirtualMACDevice (v3 subtest) asserts a Backup installs nothing and the promoted non-owner makes exactly one install, on the virtual-MAC macvlan zv4-2-10 (never the parent), of exactly 192.0.2.1/24 (record observed red on doInstallVIPs). TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly (QEMU guest) proves that installed state answers ARP and that no reply is seen with the address removed (records on vipCIDRs). The dataplane_linux_test sysctl tags stay without records.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L139) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L73) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-2`](#rfc5798-6.4.3-2)

(620) + MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the virtual router. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Master installs the IPv6 VIP on the vMAC macvlan (kernel joins its solicited-node group); Backup installs nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L728) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L724) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-3`](#rfc5798-6.4.3-3)

(625) + MUST respond to ND Neighbor Solicitation message for the IPv6 address(es) associated with the virtual router. (§6.4.3, §8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Master installs the IPv6 VIP on the vMAC macvlan so it answers NS; Backup installs nothing

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L730) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L729) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-4`](#rfc5798-6.4.3-4)

(630) ++ MUST send ND Router Advertisements for the virtual router. (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-6.4.3-4, so no unit is bound to it.

### [`RFC5798-6.4.3-5`](#rfc5798-6.4.3-5)

(645) - MUST forward packets with a destination link-layer MAC address equal to the virtual router MAC address. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuation 3): Wiring now proven on the host: TestBackupDiscardFollowsTheState asserts promotion withdraws the discard exactly once and only after the address install, and that it holds before promotion and after demotion (records observed red on doInstallVIPs and doRemoveVIPs). Wire effect: TestVRRPBackupDoesNotForwardVirtualMACFrames (QEMU guest) forwards the datagram sent to the Virtual Router MAC once the filter is withdrawn and the address installed, and not while the filter holds (records on clearBackupFilter).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L116) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L129) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-6`](#rfc5798-6.4.3-6)

(650) - MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): both admitting conditions and the refusal are asserted: TestActiveAddressOwnerAcceptsWhateverAcceptModeSays (owner, Accept_Mode False, now tagged here) requires exactly one filter with accept=true and the reported accept-mode true; TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery covers Accept_Mode True; the non-owner Accept_Mode False negative requires accept=false. Record: revert EffectiveAcceptMode on the owner unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L269) | unit/verify | revert, verified |
| positive | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L242) | unit/verify | revert, verified |

### [`RFC5798-6.4.3-7`](#rfc5798-6.4.3-7)

MUST accept packets addressed to the IPvX address(es) associated with the virtual router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, MUST NOT accept these packets. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): Forbidden: a Master that is neither owner nor Accept_Mode True accepting packets to the VIP. TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery requires accept=false; both exemptions are now tagged: Accept_Mode True (accept=true) and the address owner with Accept_Mode False (TestActiveAddressOwnerAcceptsWhateverAcceptModeSays, accept=true), so a suppression applied to the owner goes red. Record: revert EffectiveAcceptMode on the owner unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L270) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestInstanceIPv6ElectionUsesUnsignedOrder closes the missing IPv6 operand: an IPv6 Master whose advertisement source is link-local yields to a higher-priority advert and to an equal-priority sender greater in the high octet (fe80::80 vs fe80::7f), asserting advert timer cancelled, the advert's 3000ms interval adopted, Skew_Time and Master_Down_Interval recomputed from it, down timer armed, Backup; the reverse pair, which a signed compare would rank greater, keeps Master, timer armed, 1000ms interval. With TestInstanceTieBreakComparesTheAdvertisementSource (IPv4 operand) and TestFSMTransitionMatrix (timer value). Records: senderWinsTieBreak, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L138) | unit/verify | revert, verified |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L79) | unit/verify | revert, verified |
| negative | [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L136) | unit/verify | revert, verified |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L51) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): TestDecodeV3DiscardsIncompletePacket: IPv4 and IPv6 goldens accepted; 7 octets ErrTruncated; IPv4 Count 2 with one address, Count 3 over two, IPv6 with 1.5 addresses (checksums recomputed) each ErrLength. Residual: the older TestNegativeReferenceBugs N2 tag feeds a LONGER packet, which still contains the complete packet, so that tag's claim is mis-aimed; it does not weaken the new proof.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L149) | unit/verify | revert, verified |
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L511) | unit/verify | revert, verified |
| positive | [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L83) | unit/verify | revert, verified |

### [`RFC5798-7.1-3`](#rfc5798-7.1-3)

- MUST verify that the VRID is configured on the receiving interface and the local router is not the IPvX address owner (Priority = 255 (decimal)). (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-7.1-3, so no unit is bound to it.

### [`RFC5798-7.1-4`](#rfc5798-7.1-4)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestInstanceRxFailedCheckIsDiscarded proves the discard for TTL, IPv6 Hop Limit, version, type, fixed fields short, address short, checksum and unconfigured VRID, each with its own buffer, exact reason, no FSM event, and a control reaching AdvertReceived (records: revert onPacket, observed red). The owner check of the RFC 5798 list is not a discard in ze's VRRPv3 by design: rfc/corrections/rfc5798.md records that RFC 9568, which obsoletes RFC 5798, carries Verified erratum 8298 making it a SHOULD-log (Corrected Text quoted verbatim, checked against rfc/errata/rfc9568/8298.txt); the tag disclaims that half.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L530) | unit/verify | revert, verified |
| negative | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L555) | unit/verify | revert, verified |
| positive | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L104) | unit/verify | revert, verified |

### [`RFC5798-7.2-1`](#rfc5798-7.2-1)

The following operations MUST be performed when transmitting a VRRP packet: - Fill in the VRRP packet fields with the appropriate virtual router configuration state - Compute the VRRP checksum (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, rejudge after the unit gained an ipv6 subtest): fill clause: TestTransmittedAdvertFilledFromInstanceState drives an IPv4 and an IPv6 instance to Active and requires the params doSendAdvert hands the transport to carry version 3, the effective priority (200, then 150 after a tracked interface fails), 1000 ms and both VIPs, and their WriteTo octets to match literals; each encoding then decodes with its checksum verified. Checksum clause through the production path: TestSendAdvertFillsFieldsAndChecksum requires the IPv4 frame encodeLocked sends to carry an RFC 1071 sum over the RFC 5798 Section 5.2.8 pseudo-header that the test computes itself. Negative: TestBoundaryInterval (Validate refuses out-of-range intervals). IPv6 checksum is the kernel's (IPV6_CHECKSUM), not claimed. Each tagged unit has an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L292) | unit/verify | revert, verified |
| positive | [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L33) | unit/verify | revert, verified |

### [`RFC5798-7.2-2`](#rfc5798-7.2-2)

If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestTxAdvertV4OnWire (body unchanged): it and TestTxAdvertV6OnWire (guest) compare the captured source MAC with the literals 00-00-5e-00-01-0a and 00-00-5e-00-02-0a. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L390) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L100) | unit/verify | revert, verified |

### [`RFC5798-7.2-3`](#rfc5798-7.2-3)

If the protected address is an IPv4 address, then: + Set the source MAC address to virtual router MAC Address + Set the source IPv4 address to interface primary IPv4 address - else // ipv6 + Set the source MAC address to virtual router MAC Address + Set the source IPv6 address to interface link-local IPv6 address (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): IPv4 half now tagged: TestTxAdvertV4OnWire (guest) adds a lower secondary 192.0.2.5/24 after the primary 192.0.2.251/24 and requires the captured VRRPv3 source to be the primary (record: resolveParentPrimaryV4, observed red in guest). IPv6 half: TestTxAdvertV6OnWire source equals the macvlan link-local; TestSendAdvertNoLinkLocalSkipsAndCounts is the negative; the source MAC is RFC5798-7.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L408) | unit/verify | revert, verified |
| negative | [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L360) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218) | unit/verify | revert, verified |

### [`RFC5798-7.2-4`](#rfc5798-7.2-4)

Set the IPvX protocol to VRRP - Send the VRRP packet to the VRRP IPvX multicast group (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): re-read after the comment-only edit to TestTxAdvertV4OnWire (body unchanged): it and TestTxAdvertV6OnWire compare the captured protocol/next header and destination with the literals 112, 224.0.0.18 and ff02::12. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L104) | unit/verify | revert, verified |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent) re-judged: the tagged units changed only by added RFC requirement tag comment lines for the new rows (8.1.2-6 / 8.2.2-8 / 8.2-4 / 8.1.1-2); test bodies byte-identical, records current, verdict unchanged. Prior note: vrrp judge 2026-09-29 (independent, continuation 3): Both halves proven. Owner: TestOwnerFilterWiredOnPromotionForEveryFamily (ipv4-v3) drives an owner to setOwnerFilter with the parent and exactly the owned IPv4 address before install, and the same group as non-owner names none (records observed red on doInstallVIPs); TestVRRPOwnerAnswersWithVirtualMACOnly (QEMU guest) captures the parent's physical-MAC ARP reply without the filter and after withdrawal, and only the Virtual Router MAC with it (records on ownerARPReplyTerm, ownerFilterTables). Non-owner: TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly captures the physical MAC without applyDataplaneSysctls and only the virtual MAC with it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L141) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L45) | unit/verify | revert, verified |
| negative | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L177) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L176) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L61) | unit/verify | revert, verified |

### [`RFC5798-8.1.2-6`](#rfc5798-8.1.2-6)

When a host sends an ARP request for one of the virtual router IPv4 addresses, the Virtual Router Master MUST respond to the ARP request with an ARP response that indicates the virtual MAC address for the virtual router. (§8.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent): TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly (Linux integration) captures on the wire that with the product recipe (applyDataplaneSysctls) a LAN host's ARP request for the non-owner VIP is answered with the virtual MAC only; the control (recipe absent) sees the physical MAC answer and, with the address removed, no answer, so the capture discriminates. TestVRRPOwnerAnswersWithVirtualMACOnly: with the owner filter (ownerARPReplyTerm) the owner's ARP replies carry the virtual MAC; without it (ownerFilterTables) the physical MAC is observed. Each tagged unit has an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L69) | unit/verify | revert, verified |

### [`RFC5798-8.1.1-2`](#rfc5798-8.1.1-2)

If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-30 (independent) re-judged: the unit changed only by added tag comment lines for RFC9568-8.1.1-1/-2; body byte-identical, records current. TestVRRPRedirectSourceFollowsVirtualMAC (Linux guest integration, two non-owner virtual-MAC macvlans under the product sysctl recipe, icmp_errors_use_inbound_ifaddr reset to 0 before applyDataplaneSysctls) asserts the IPv4 redirect for a packet sent to a virtual MAC is sourced from that virtual router's VIP despite a reverse route via the real interface; the negative sends to the physical MAC and sees 192.0.2.254, so the capture separates the two. Both polarities recorded observed red on applyDataplaneSysctls.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L22) | unit/verify | revert, verified |

### [`RFC5798-8.2.1-2`](#rfc5798-8.2.1-2)

If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.2.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-30 (independent) re-judged: the {gap} is withdrawn; the earlier judge read only Ze's sysctl branch and not the kernel path. TestVRRPIPv6RedirectFollowsVirtualMAC (Linux guest integration, two non-owner Active Router dataplanes built with the product's CreateMacvlanDevice, VirtualMAC, vipCIDRs and IPv6 applyDataplaneSysctls) sends one datagram to each virtual MAC and one to the physical MAC, and asserts the single ICMPv6 redirect (type 137, checksum and target verified) is sourced from a link-local held by the macvlan whose virtual MAC was hit and by no other device; the negative sees the physical-MAC datagram redirected from a parent link-local held by neither macvlan. Stack-level: Ze supplies the per-VR virtual-MAC macvlan, and Linux ndisc_send_redirect sources the redirect from the ingress device's link-local, so the destination MAC selects the Virtual Router (the method §8.2.1 names). Linux redirects only when the route leaves by the ingress device (ip6_forward), which the test arranges; otherwise no redirect is sent and none can be misattributed. Which macvlan link-local (VIP or VMAC-derived) is used is not pinned: that is the separate RFC9568-8.2.1-1 SHOULD, not judged here. Records: + on packet.VirtualMAC, - on applyDataplaneSysctls, both observed red (revert route).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L37) | unit/verify | revert, verified |

### [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1)

If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5798-8.1.3-1, so no unit is bound to it.

### [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1)

The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent) re-judged: the tagged units changed only by added RFC requirement tag comment lines for the new rows (8.1.2-6 / 8.2.2-8 / 8.2-4 / 8.1.1-2); test bodies byte-identical, records current, verdict unchanged. Prior note: vrrp judge 2026-09-29 (independent, continuation 3): Re-read: the owner units changed only by added RFC 8.1.2-1 tag comments; bodies unchanged. Verdict unchanged. Owner: TestOwnerFilterWiredOnPromotionForEveryFamily (ipv6) wiring, records on doInstallVIPs; TestVRRPOwnerAnswersWithVirtualMACOnly wire effect with a physical-MAC control. Non-owner: TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly (QEMU guest) captures a physical-MAC NA with the address on the parent, only the virtual MAC with it on the macvlan, and no NA once removed; TestInstanceIPv6VIPLivesOnVirtualMACDevice pins the install device.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L727) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L34) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L726) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L172) | unit/verify | revert, verified |

### [`RFC5798-8.2.2-8`](#rfc5798-8.2.2-8)

When a host sends an ND Neighbor Solicitation message for the virtual router IPv6 address, the Virtual Router Master MUST respond to the ND Neighbor Solicitation message with the virtual MAC address for the virtual router. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent): TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly captures NAs whose Target Link-Layer Address option carries only the virtual MAC under the IPv6 recipe; control on the parent sees the physical MAC and, removed, no answer. TestVRRPOwnerAnswersWithVirtualMACOnly: with ownerNDAdvertTerm the owner's NA carries the virtual MAC; without the filter the physical MAC is observed. Each tagged unit has an observed-red record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L60) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestEngineWaitsForALateVirtualMACDevice (ipv6) runs the live macvlanCreator.create over a registry that creates the device 30ms late and a transport that refuses an absent device: order is device, install, Neighbor Advertisement; a device that never appears gives no instance, install or NA. Deleting the wait reddens the positive (records on register.go create, observed red). With TestEngineAnnouncesNothingBeforeTheVirtualMACDevice and TestInstanceDelaysAnnounceUntilParentUsable (address half). Kernel-originated RA/NS are outside ze's announce path and not observed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L118) | unit/verify | revert, verified |
| negative | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L775) | unit/verify | revert, verified |
| positive | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L773) | unit/verify | revert, verified |

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
| Mapped sentences | 51 |
| Declined as scope | 11 |
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
| [`RFC5798-8.1.2-6`](#rfc5798-8.1.2-6) When a host sends an ARP request for one of the virtual router IPv4 addresses, the Virtual Router Master MUST respond to the ARP request with an ARP response that indicates the virtual MAC address for the virtual router. (§8.1.2) | restated | RFC9568-8.1.2-6 | RFC 9568 Section 8.1.2 keeps the obligation and names the Virtual Router Master the Active Router |
| [`RFC5798-8.1.1-2`](#rfc5798-8.1.1-2) If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.1.1; lowercase "must" in the RFC) | restated | RFC9568-8.1.1-2 | RFC 9568 Section 8.1.1 restates it for the Active Router, "then it must determine to which Virtual Router the packet was sent when selecting the redirect source address" |
| [`RFC5798-8.2.1-2`](#rfc5798-8.2.1-2) If a VRRP router is acting as Master for virtual router(s) containing addresses it does not own, then it must determine which virtual router the packet was sent to when selecting the redirect source address. (§8.2.1; lowercase "must" in the RFC) | restated | RFC9568-8.2.1-2 | RFC 9568 Section 8.2.1 restates it for the Active Router, "then it has to determine to which Virtual Router the packet was sent when selecting the redirect source address" |
| [`RFC5798-8.1.3-1`](#rfc5798-8.1.3-1) If Proxy ARP is to be used on a VRRP router, then the VRRP router must advertise the virtual router MAC address in the Proxy ARP message. (§8.1.3; lowercase "must" in the RFC) | restated | RFC9568-8.1.3-1 | RFC 9568 Section 8.1.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.2.2-1`](#rfc5798-8.2.2-1) The Virtual Router Master MUST NOT respond with its physical MAC address. This allows the client to always use the same MAC address regardless of the current Master router. (§8.2.2) | restated | RFC9568-8.2.2-1 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-8`](#rfc5798-8.2.2-8) When a host sends an ND Neighbor Solicitation message for the virtual router IPv6 address, the Virtual Router Master MUST respond to the ND Neighbor Solicitation message with the virtual MAC address for the virtual router. (§8.2.2) | restated | RFC9568-8.2.2-8 | RFC 9568 Section 8.2.2 keeps the obligation and names the Virtual Router Master the Active Router |
| [`RFC5798-8.2.2-2`](#rfc5798-8.2.2-2) When a Virtual Router Master sends an ND Neighbor Solicitation message for a host's IPv6 address, the Virtual Router Master MUST include the virtual MAC address for the virtual router if it sends a source link-layer address option in the neighbor solicitation message. (§8.2.2) | restated | RFC9568-8.2.2-2 | RFC 9568 Section 8.2.2 keeps the obligation unchanged |
| [`RFC5798-8.2.2-3`](#rfc5798-8.2.2-3) It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | restated | RFC9568-8.2.2-3 | RFC 9568 Section 8.2.2 keeps the prohibition unchanged |
| [`RFC5798-8.2.2-4`](#rfc5798-8.2.2-4) o At system boot, when initializing interfaces for VRRP operation, all ND Router and Neighbor Advertisements and Solicitation messages must be delayed until both the IPv6 address and the virtual router MAC address are configured. (§8.2.2; lowercase "must" in the RFC) | restated | RFC9568-8.2.2-4 | RFC 9568 Section 8.2.2 states the same delay with an uppercase MUST |
| [`RFC5798-8.2.3-1`](#rfc5798-8.2.3-1) The Backup routers must be configured to send the same Router Advertisement options as the address owner. (§8.2.3; lowercase "must" in the RFC) | restated | RFC9568-8.2.3-1 | RFC 9568 Section 8.2.3 states the same obligation with an uppercase MUST |
| [`RFC5798-8.4.2-1`](#rfc5798-8.4.2-1) An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. When a virtual router is configured this way and is the Master, it MUST send both types at the configured rate, even if sub-second. (§8.4.2) | restated | RFC9568-8.4.2-1 | RFC 9568 Section 8.4.2 keeps the obligation unchanged |
| [`RFC5798-A.2-1`](#rfc5798-a.2-1) The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no functional-address obligation |
| [`RFC5798-7.1-5`](#rfc5798-7.1-5) If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (§7.1) | restated | RFC9568-7.1-7 | RFC 9568 Section 7.1 keeps the log and adds rate-limiting to it |
| [`RFC5798-7.1-8`](#rfc5798-7.1-8) MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event (§7.1) | restated | RFC9568-7.1-9 | RFC 9568 Section 7.1 keeps the log, adds rate-limiting, and states it in one sentence covering the Max Advertise Interval mismatch as well |
| [`RFC5798-8.1.1-1`](#rfc5798-8.1.1-1) The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision. (§8.1.1; lowercase "should" in the RFC) | restated | RFC9568-8.1.1-1 | RFC 9568 Section 8.1.1 keeps the sentence for IPv4 |
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
| [`RFC5798-A.2-3`](#rfc5798-a.2-3) For both the multicast and unicast mode of operation, VRRP advertisements sent to 224.0.0.18 should be encapsulated as described in [RFC1469]. (§A.2; lowercase "should" in the RFC) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no Token-Ring encapsulation recommendation |
| [`RFC5798-7.1-6`](#rfc5798-7.1-6) If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event, and MAY indicate via network management that an error occurred. - MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. If the above check fails, the receiver SHOULD log the event and MAY indicate via network management that a misconfiguration was detected. (§7.1) | restated | RFC9568-7.1-10 | RFC 9568 Section 7.1 keeps the network-management indication as a MAY |
| [`RFC5798-7.1-7`](#rfc5798-7.1-7) MAY verify that "Count IPvX Addrs" and the list of IPvX address(es) match the IPvX Address(es) configured for the VRID. (§7.1) | restated | RFC9568-7.1-11 | RFC 9568 Section 7.1 keeps the address-list check as a MAY and renames the field to IPvX Addr Count |
| [`RFC5798-8.2.2-7`](#rfc5798-8.2.2-7) Note that on a restarting Master router where the VRRP protected address is the interface address, (that is, priority 255) duplicate address detection (DAD) may fail, as the Backup router may answer that it owns the address. One solution is to not run DAD in this case. (§8.2.2) | restated | RFC9568-8.2.2-7 | RFC 9568 Section 8.2.2 keeps the note |
| [`RFC5798-8.4.2-5`](#rfc5798-8.4.2-5) An implementation MAY implement a configuration flag that tells it to listen for and send both VRRPv2 and VRRPv3 advertisements. (§8.4.2) | restated | RFC9568-8.4.2-6 | RFC 9568 Section 8.4.2 keeps the flag as a MAY |
| [`RFC5798-8.4.2-6`](#rfc5798-8.4.2-6) It MAY report when a VRRPv3 Master is *not* sending VRRPv2 packets: that suggests they don't agree on whether they're supporting VRRPv2 routers. (§8.4.2) | restated | RFC9568-8.4.2-7 | RFC 9568 Section 8.4.2 keeps the report as a MAY |
| [`RFC5798-A.2-2`](#rfc5798-a.2-2) The functional-address mode of operation MUST be implemented by routers supporting VRRP on Token Ring. Additionally, routers MAY support the unicast mode of operation to take advantage of newer Token-Ring adapter implementations that support non-promiscuous reception for multiple unicast MAC addresses and to avoid both the multicast traffic and usage conflicts associated with the use of Token-Ring functional addresses. (§A.2) | dropped | not stated | RFC 9568 removes the legacy-media appendices. Its Section 1.2 lists as change 6 that the appendices describing operation over FDDI, Token Ring and ATM LAN Emulation were removed, so RFC 9568 states no unicast-mode permission |
