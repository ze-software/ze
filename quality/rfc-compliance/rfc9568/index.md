# RFC 9568 - Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6

VRRPv3 (IPv4 and IPv6). Every requirement this repository extracted from RFC 9568, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50 | of 66 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 6 | of 66 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 66 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 66 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 112 | 112 of 190 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 66 | of 94 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 66 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 66 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 66 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 66 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 66 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10 | of 66 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 55 | of 66 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 9 counts marked as a part above are the whole of the 66 gated MUSTs: they add up to 66. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | VRRPv3 (IPv4 and IPv6) |
| Enrolment | Enrolled |
| Requirements | 94 |
| Gated MUST-level | 66 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 10 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 190 |
| Tagged units | 190 |
| Recorded audit verdicts | 55 |
| Discrimination records | 112 |
| Summary | `rfc/short/rfc9568.md` |
| Requirement shard | `rfc/requirements/rfc9568.md` |
| RFC text | `rfc/full/rfc9568.txt` |

## Enrolment

Enrolled: VRRP Version 3 for IPv4 and IPv6

## What the public ledger says

**Area:** VRRPv3 (IPv4 and IPv6)

**Proof:** 37 of 66 MUST proven

**Evidence:** unit 37

**Unmet:** RFC9568-5.1.1.2-1, RFC9568-5.1.2.2-1, RFC9568-6.4.2-3, RFC9568-6.4.3-4, RFC9568-8.2.2-2, RFC9568-8.2.2-3, RFC9568-8.2.3-1, RFC9568-8.4.2-1, RFC9568-8.4.2-2, RFC9568-8.4.2-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 50 | one part of the gated population |
| Annotated (including scoped evidence) | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **66** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (50):** [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2), [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2), [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1), [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1), [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2), [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1), [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1), [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1), [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2), [`RFC9568-6.1-1`](#rfc9568-6.1-1), [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1), [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2), [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4), [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5), [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6), [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7), [`RFC9568-6.4.2-8`](#rfc9568-6.4.2-8), [`RFC9568-6.4.2-9`](#rfc9568-6.4.2-9), [`RFC9568-6.4.2-10`](#rfc9568-6.4.2-10), [`RFC9568-6.4.3-1`](#rfc9568-6.4.3-1), [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3), [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5), [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6), [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7), [`RFC9568-6.4.3-8`](#rfc9568-6.4.3-8), [`RFC9568-6.4.3-9`](#rfc9568-6.4.3-9), [`RFC9568-6.4.3-10`](#rfc9568-6.4.3-10), [`RFC9568-6.4.3-11`](#rfc9568-6.4.3-11), [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12), [`RFC9568-7.1-1`](#rfc9568-7.1-1), [`RFC9568-7.1-2`](#rfc9568-7.1-2), [`RFC9568-7.1-3`](#rfc9568-7.1-3), [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1), [`RFC9568-5.2.8-2`](#rfc9568-5.2.8-2), [`RFC9568-5.2.8-3`](#rfc9568-5.2.8-3), [`RFC9568-7.1-4`](#rfc9568-7.1-4), [`RFC9568-7.1-5`](#rfc9568-7.1-5), [`RFC9568-7.1-6`](#rfc9568-7.1-6), [`RFC9568-7.2-5`](#rfc9568-7.2-5), [`RFC9568-7.2-3`](#rfc9568-7.2-3), [`RFC9568-7.4-1`](#rfc9568-7.4-1), [`RFC9568-8.1.1-2`](#rfc9568-8.1.1-2), [`RFC9568-8.2.1-2`](#rfc9568-8.2.1-2), [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1), [`RFC9568-8.1.2-6`](#rfc9568-8.1.2-6), [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2), [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1), [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1), [`RFC9568-8.2.2-4`](#rfc9568-8.2.2-4), [`RFC9568-8.2.2-8`](#rfc9568-8.2.2-8)

**Annotated (including scoped evidence) (16):** [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1), [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1), [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1), [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1), [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3), [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2), [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4), [`RFC9568-7.2-1`](#rfc9568-7.2-1), [`RFC9568-7.2-2`](#rfc9568-7.2-2), [`RFC9568-7.2-4`](#rfc9568-7.2-4), [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2), [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3), [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1), [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1), [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2), [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3)

## Figures

- **MUST bar**: Ze proven 37, Ze correctness gap 29
- **Overview bar**: Ze MUST proven 37, SHOULD proven 3, SHOULD not decided 25, Ze correctness gap 29

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 66 |
| MUST rows kept (the MUST bar) | 66 |
| Ze proven | 37 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 29 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 28 |
| SHOULD proven | 3 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 25 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc9568.json](../../../data/rfc/rfc9568.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9568-5.1.1.2-1` | Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | MUST NOT | 5.1.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is the router this binds, and the forwarding decision is the dataplane's, not the VRRP plugin's: onPacket (internal/plugins/vrrp/instance.go) consumes every received advertisement and re-emits nothing, but whether the kernel or the VPP backend ze programs forwards a transit datagram to 224.0.0.18 is not shown, and no test reads either backend's behaviour or names a verified layer token, so the row is a gap |
| `RFC9568-5.1.1.3-1` | The TTL MUST be set to 255. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L316). **negative:** no negative test. **{single-polarity}:** buildIPv4Header unconditionally writes TTL 255 at internal/plugins/vrrp/transport/transport.go:562, so no input yields a different TTL -- the receive-side TTL!=255 discard is the separate RFC9568-5.1.1.3-2 |
| `RFC9568-5.1.1.3-2` | A VRRP Router receiving a packet with the TTL not equal to 255 MUST discard the packet [RFC5082]. (§5.1.1.3) | MUST | 5.1.1.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L70). **positive:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L68). **negative:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L69). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L485) |
| `RFC9568-5.1.2.2-1` | Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | MUST NOT | 5.1.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is the router this binds, and the forwarding decision is the dataplane's, not the VRRP plugin's: onPacket (internal/plugins/vrrp/instance.go) consumes every received advertisement and re-emits nothing, but whether the kernel or the VPP backend ze programs forwards a transit datagram to ff02::12 is not shown, and no test reads either backend's behaviour or names a verified layer token, so the row is a gap |
| `RFC9568-5.1.2.3-1` | The Hop Limit MUST be set to 255. (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L260). **negative:** no negative test. **{single-polarity}:** the hop limit is a socket option, IPV6_MULTICAST_HOPS 255 set once at openV6 internal/plugins/vrrp/transport/backend_linux.go:185, so every advertisement on that socket carries 255 and no input produces another value -- the receive-side hop-limit discard is the separate RFC9568-5.1.2.3-2. The sole positive test is in internal/plugins/vrrp/transport/transport_integration_linux_test.go, gated //go:build linux; it runs unprivileged in a user and network namespace of its own (userns.Enter, internal/test/userns), and its setupLab fails rather than skips when it lacks a privilege. The evidence itself is genuine: the test getsockopt reads IPV6_MULTICAST_HOPS back from both the tx and the NA socket and requires 255, so the kernel accepted the value |
| `RFC9568-5.1.2.3-2` | A VRRP Router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet [RFC5082]. (§5.1.2.3) | MUST | 5.1.2.3 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L96). **positive:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L70). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L615). **negative:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L71) |
| `RFC9568-5.2.2-1` | A packet with unknown type MUST be discarded. (§5.2.2) | MUST | 5.2.2 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L72). **positive:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L72). **negative:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L73). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L144) |
| `RFC9568-5.2.4-1` | The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1620). **positive:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L811). **negative:** `unit/verify` [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L813) |
| `RFC9568-5.2.4-2` | VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal). (§5.2.4) | MUST | 5.2.4 | **positive:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319). **positive:** `unit/verify` [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1621). **negative:** `unit/verify` [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) |
| `RFC9568-5.2.5-1` | If the received count is 0, the VRRP advertisement MUST be ignored. (§5.2.5, erratum 8299) | MUST | 5.2.5 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74). **positive:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L74). **negative:** `unit/verify` [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L75). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L495) |
| `RFC9568-5.2.6-1` | The Reserve field MUST be set to zero on transmission and ignored on reception. (§5.2.6) | MUST | 5.2.6 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107). **negative:** `unit/verify` [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L589) |
| `RFC9568-5.2.9-1` | For IPv6, the first address MUST be the IPv6 link-local address associated with the Virtual Router. (§5.2.9, §6.1, erratum 8300) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L99). **positive:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L551). **negative:** `unit/verify` [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L553). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L253) |
| `RFC9568-5.2.9-2` | The address family of the addresses, IPv4 or IPv6 but not both, MUST be the same as the VRRP packet's IPvX header address family. (§5.2.9) | MUST | 5.2.9 | **positive:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L575). **negative:** `unit/verify` [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L577) |
| `RFC9568-6.1-1` | Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109). **negative:** `unit/verify` [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L111) |
| `RFC9568-6.4.2-1` | - It MUST NOT respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L417). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L454) |
| `RFC9568-6.4.2-2` | - It MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L738). **negative:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L740) |
| `RFC9568-6.4.2-3` | - It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the VRRP plugin builds no Router Advertisement in any state, since its only ICMPv6 message is the unsolicited Neighbor Advertisement (icmpv6TypeNA in internal/plugins/vrrp/transport/na.go), but ze carries a Router Advertisement builder elsewhere (internal/core/ndp/ra.go) and no test proves that a Backup's interface sends none for the Virtual Router; the Active-side emission is the gap RFC9568-6.4.3-4 |
| `RFC9568-6.4.2-4` | * It MUST discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L110). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L126). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L111). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L127) |
| `RFC9568-6.4.2-5` | * It MUST NOT accept packets addressed to the IPvX address(es) associated with the Virtual Router. (§6.4.2) | MUST NOT | 6.4.2 | **positive:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L419). **negative:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L456) |
| `RFC9568-6.4.2-6` | If a Shutdown event is received, then: - Cancel the Active_Down_Timer - Transition to the {Initialize} state (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111) |
| `RFC9568-6.4.2-7` | If the Active_Down_Timer fires, then: - Send an ADVERTISEMENT - If the protected IPvX address is an IPv4 address, then: o For each IPv4 address associated with the Virtual Router, broadcast a gratuitous ARP message containing the Virtual Router IPv4 address and with the target link-layer address set to the Virtual Router MAC address. - else // IPv6 o Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the Virtual Router. o For each IPv6 address associated with the Virtual Router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) clear, the Override flag (O) set, the target address set to the IPv6 address of the Virtual Router, and the target link-layer address set to the Virtual Router MAC address. - endif // was protected address IPv4? - Set the Adver_Timer to Advertisement_Interval - Transition to the {Active} state (§6.4.2, erratum 7949) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L53). **positive:** `unit/verify` [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L447). **positive:** `unit/verify` [`TestPromotionInstallsIPv6AddressesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_v6_test.go#L28). **positive:** `unit/verify` [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L89). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L638) |
| `RFC9568-6.4.2-8` | If the Priority in the ADVERTISEMENT is 0, then: o Set the Active_Down_Timer to Skew_Time (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L113). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L115) |
| `RFC9568-6.4.2-9` | If Preempt_Mode is False, or if the Priority in the ADVERTISEMENT is greater than or equal to the local Priority, then: + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L117). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L119) |
| `RFC9568-6.4.2-10` | else // preempt was true and priority was less than the local priority + Discard the ADVERTISEMENT (§6.4.2) | MUST | 6.4.2 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L121). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L123) |
| `RFC9568-6.4.3-1` | - It MUST respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.3, §8.1.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L71). **positive:** `unit/verify` [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L28). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L67). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L142). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L68) |
| `RFC9568-6.4.3-2` | - It MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the Virtual Router. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L742). **positive:** `unit/verify` [`TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_solicited_node_integration_linux_test.go#L58). **negative:** no negative test. **{single-polarity}:** on promotion ze joins the group on the virtual-MAC macvlan itself, before the address exists (doInstallVIPs calls joinSolicitedNode, internal/plugins/vrrp/solicitednode_linux.go), and the kernel joins it again when the address install lands, so the not-a-member case is the Backup requirement RFC9568-6.4.2-2 |
| `RFC9568-6.4.3-3` | - It MUST respond to ND Neighbor Solicitation messages (with the Router Flag (R) set) for the IPv6 address(es) associated with the Virtual Router. (§6.4.3, §8.2.2) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestVRRPSolicitedNACarriesRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_nd_router_flag_integration_linux_test.go#L36). **negative:** `unit/verify` [`TestVRRPSolicitedNACarriesRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_nd_router_flag_integration_linux_test.go#L37) |
| `RFC9568-6.4.3-4` | It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3) | MUST | 6.4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| `RFC9568-6.4.3-5` | * It MUST forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L116). **positive:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L132). **negative:** `unit/verify` [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L117). **negative:** `unit/verify` [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L133) |
| `RFC9568-6.4.3-6` | * It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L267). **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L241). **positive:** `unit/verify` [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L453). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L205). **negative:** `unit/verify` [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L421) |
| `RFC9568-6.4.3-7` | It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, it MUST NOT accept these packets. (§6.4.3) | MUST NOT | 6.4.3 | **positive:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L203). **negative:** `unit/verify` [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L268). **negative:** `unit/verify` [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L243) |
| `RFC9568-6.4.3-8` | If a Shutdown event is received, then: - Cancel the Adver_Timer - Send an ADVERTISEMENT with Priority = 0 - Transition to the {Initialize} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L125). **positive:** `unit/verify` [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L524). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L127) |
| `RFC9568-6.4.3-9` | If the Adver_Timer fires, then: - Send an ADVERTISEMENT - Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L129). **negative:** `unit/verify` [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L640) |
| `RFC9568-6.4.3-10` | If the Priority in the ADVERTISEMENT is 0, then: o Send an ADVERTISEMENT o Reset the Adver_Timer to Advertisement_Interval (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L131). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L133) |
| `RFC9568-6.4.3-11` | If the Priority in the ADVERTISEMENT is greater than the local Priority or the Priority in the ADVERTISEMENT is equal to the local Priority and the primary IPvX address of the sender is greater than the local primary IPvX address (based on an unsigned integer comparison of the IPvX addresses in network byte order), then: + Cancel Adver_Timer + Set the Active_Adver_Interval to the Max Advertise Interval contained in the ADVERTISEMENT + Recompute the Skew_Time + Recompute the Active_Down_Interval + Set the Active_Down_Timer to Active_Down_Interval + Transition to the {Backup} state (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L135). **positive:** `unit/verify` [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L49). **positive:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L76). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L137). **negative:** `unit/verify` [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L50). **negative:** `unit/verify` [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L77) |
| `RFC9568-6.4.3-12` | else // new Active Router logic + Discard the ADVERTISEMENT + Send an ADVERTISEMENT immediately to assert the {Active} state to the sending VRRP Router and to update any learning bridges with the correct Active VRRP Router path. (§6.4.3) | MUST | 6.4.3 | **positive:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L139). **negative:** `unit/verify` [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L141) |
| `RFC9568-7.1-1` | * It MUST verify that the VRRP version is 3. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L561) |
| `RFC9568-7.1-2` | * It MUST verify that the VRRP packet type is 1 (ADVERTISEMENT). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L146) |
| `RFC9568-7.1-3` | * It MUST verify that the received packet contains the complete VRRP packet (including fixed fields and the IPvX address). (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78). **positive:** `unit/verify` [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L150). **negative:** `unit/verify` [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L151). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L461) |
| `RFC9568-5.2.8-1` | It MUST verify the VRRP checksum. (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L98). **positive:** `unit/verify` [`TestDecodeVerifiesChecksumBothFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L287). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L613). **negative:** `unit/verify` [`TestDecodeVerifiesChecksumBothFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L288) |
| `RFC9568-5.2.8-2` | For the IPv4 address family, the checksum calculation only includes the VRRP message starting with the Version field and ending after the last IPv4 address (refer to Section 5.2). (§5.2.8) | MUST | 5.2.8 | **positive:** `unit/verify` [`TestDecodeV3IPv4ChecksumOnlyConfiguredForm`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L149). **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L111). **negative:** `unit/verify` [`TestDecodeV3IPv4ChecksumOnlyConfiguredForm`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L150) |
| `RFC9568-5.2.8-3` | For the IPv6 address family, the checksum calculation also includes a prepended "pseudo-header", as defined in Section 8.1 of [RFC8200]. (§5.2.8) | MUST | 5.2.8 | **positive:** `unit/verify` [`TestDecodeV3IPv6ChecksumCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc9568_ipv6_pseudo_header_test.go#L21). **negative:** `unit/verify` [`TestDecodeV3IPv6ChecksumCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc9568_ipv6_pseudo_header_test.go#L22) |
| `RFC9568-7.1-4` | It MUST verify that the VRID is configured on the receiving interface. (§7.1, erratum 8298) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80). **positive:** `unit/verify` [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L130). **positive:** `unit/verify` [`TestRxV3VRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_interface_v3_test.go#L25). **negative:** `unit/verify` [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L131). **negative:** `unit/verify` [`TestRxV3VRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_interface_v3_test.go#L26). **negative:** `unit/verify` [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L157) |
| `RFC9568-7.1-5` | It MUST verify that the Max Advertise Interval is non zero. (§7.1, erratum 8301) | MUST | 7.1 | **positive:** `unit/verify` [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81). **negative:** `unit/verify` [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L505) |
| `RFC9568-7.1-6` | If any one of the above checks fails, the receiver MUST discard the packet (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L102). **positive:** `unit/verify` [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L573). **negative:** `unit/verify` [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L548). **negative:** `unit/verify` [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L101) |
| `RFC9568-7.2-1` | The following operations MUST be performed when transmitting a VRRP packet: * Fill in the VRRP packet fields with the appropriate Virtual Router configuration state (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109). **positive:** `unit/verify` [`TestEncodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L185). **positive:** `unit/verify` [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L39). **positive:** `unit/verify` [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L40). **negative:** no negative test. **{single-polarity}:** the v3 golden encodes over IPv4 and IPv6 and the frames the transport sends pin every VRRP field to the advertisement state -- WriteTo internal/plugins/vrrp/packet/packet.go:251, called by encodeLocked internal/plugins/vrrp/transport/transport.go:535 -- and the fill has no input it refuses, while rejecting a corrupted encoding is the separate receive requirement RFC9568-7.1-3 |
| `RFC9568-7.2-5` | * Compute the VRRP checksum (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L110). **positive:** `unit/verify` [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L37). **positive:** `unit/verify` [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L41). **negative:** `unit/verify` [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L38) |
| `RFC9568-7.2-2` | * Set the source MAC address to the Virtual Router MAC address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L436). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L42). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L105). **negative:** no negative test. **{single-polarity}:** the source MAC is the derived Virtual Router MAC from packet.VirtualMAC internal/plugins/vrrp/packet/packet.go:97, egressed by binding the tx socket to that vMAC macvlan (internal/plugins/vrrp/transport/backend_linux.go:133 for IPv4, :179 for IPv6), a deterministic derivation with no input that yields another MAC |
| `RFC9568-7.2-3` | * If the protected address is an IPv4 address, then: - Set the source IPv4 address to the interface's primary IPv4 address * else // IPv6 - Set the source IPv6 address to the interface's link-local IPv6 address (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L45). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L107). **negative:** `unit/verify` [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L412). **negative:** `unit/verify` [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L364) |
| `RFC9568-7.2-4` | * Set the IPvX protocol to VRRP * Send the VRRP packet to the VRRP IPvX multicast group (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L318). **positive:** `unit/verify` [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L48). **positive:** `unit/verify` [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L109). **negative:** no negative test. **{single-polarity}:** buildIPv4Header writes protocol 112 at internal/plugins/vrrp/transport/transport.go:563 and SendAdvert targets 224.0.0.18 / ff02::12 at internal/plugins/vrrp/transport/backend_linux.go:242,256, all constants with no input that changes them |
| `RFC9568-7.4-1` | The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]. (§7.4) | MUST NOT | 7.4 | **positive:** `unit/verify` [`TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L36). **negative:** `unit/verify` [`TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L37) |
| `RFC9568-8.1.1-2` | If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it must determine to which Virtual Router the packet was sent when selecting the redirect source address. (§8.1.1; lowercase "must" in the RFC) | MUST | 8.1.1 | **positive:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L28). **negative:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L29) |
| `RFC9568-8.2.1-2` | If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it has to determine to which Virtual Router the packet was sent when selecting the redirect source address. (§8.2.1; "has to" in the RFC, no BCP 14 keyword; read as the §8.1.1 "must" restated for IPv6) | MUST | 8.2.1 | **positive:** `unit/verify` [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L39). **negative:** `unit/verify` [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L40) |
| `RFC9568-8.1.2-1` | The Active Router MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2) | MUST NOT | 8.1.2 | **positive:** `unit/verify` [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L73). **positive:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L178). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L61). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L50). **negative:** `unit/verify` [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L144). **negative:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L179). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L62). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L51) |
| `RFC9568-8.1.2-6` | When a host sends an ARP request for one of the Virtual Router IPv4 addresses, the Active Router MUST respond to the ARP request with an ARP response that indicates the Virtual Router MAC address for the Virtual Router. (§8.1.2) | MUST | 8.1.2 | **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L69). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L56). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L70). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L57) |
| `RFC9568-8.1.2-2` | * At system boot, when initializing interfaces for VRRP operation, gratuitous ARP messages MUST be delayed until both the IPv4 address and the Virtual Router MAC address are configured. (§8.1.2) | MUST | 8.1.2 | **positive:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L34). **positive:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L113). **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L789). **negative:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L35). **negative:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L114). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L790) |
| `RFC9568-8.1.3-1` | If Proxy ARP is to be used on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC address in the Proxy ARP message. (§8.1.3) | MUST | 8.1.3 | **positive:** `unit/verify` [`TestProxyARPOnVRRPInterfaceRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/proxyarp_test.go#L34). **negative:** `unit/verify` [`TestNoProxyARPOnVRRPInterfaceAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/proxyarp_test.go#L68) |
| `RFC9568-8.2.2-1` | The Active Router MUST NOT respond with its physical MAC address. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** `unit/verify` [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L744). **positive:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L174). **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L32). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L54). **negative:** `unit/verify` [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L175). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L33). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L55) |
| `RFC9568-8.2.2-2` | When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link- layer address option in the Neighbor Solicitation message. (§8.2.2) | MUST | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the kernel fills the option from the device the host's route resolves through (Linux limitation LL-NS-SOURCE-MAC, docs/architecture/linux-limitations.md), and ze installs nothing that makes that device the virtual-MAC macvlan: an owned VIP sits there only as a /128 (vipMaskBits internal/plugins/vrrp/register.go), so the subnet route stays on the parent and the option carries the parent's physical MAC; no test observes the option; fix owned by plan/immediate/spec-vrrp-owner-arp-rfc-defects.md |
| `RFC9568-8.2.2-3` | It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | MUST NOT | 8.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the kernel fills the option from the device the host's route resolves through (Linux limitation LL-NS-SOURCE-MAC, docs/architecture/linux-limitations.md), and ze installs nothing that makes that device the virtual-MAC macvlan: an owned VIP sits there only as a /128 (vipMaskBits internal/plugins/vrrp/register.go), so the subnet route stays on the parent and the option carries the parent's physical MAC; no test observes the option; fix owned by plan/immediate/spec-vrrp-owner-arp-rfc-defects.md |
| `RFC9568-8.2.2-4` | * At system boot, when initializing interfaces for VRRP operation, all ND Router Advertisements, ND Neighbor Advertisements, and ND Neighbor Solicitation messages MUST be delayed until both the IPv6 address and the Virtual Router MAC address are configured. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L36). **positive:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L115). **positive:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L791). **negative:** `unit/verify` [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L37). **negative:** `unit/verify` [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L116). **negative:** `unit/verify` [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L793) |
| `RFC9568-8.2.2-8` | When a host sends an ND Neighbor Solicitation message for a Virtual Router IPv6 address, the Active Router MUST respond to the ND Neighbor Solicitation message with the Virtual Router MAC address for the Virtual Router. (§8.2.2) | MUST | 8.2.2 | **positive:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L36). **positive:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L62). **negative:** `unit/verify` [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L37). **negative:** `unit/verify` [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L63) |
| `RFC9568-8.2.3-1` | The Backup Routers MUST be configured to send the same Router Advertisement options as the address owner. (§8.2.3) | MUST | 8.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation rests on the Router Advertisement ze does not send, which is the gap at RFC9568-6.4.3-4. The vrrp group carries no Router Advertisement leaf at all -- applyGroupLeaves internal/plugins/vrrp/groups.go accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only -- so there is no option set a Backup could be configured to match |
| `RFC9568-8.4.2-1` | When a Virtual Router is configured this way and is the Active Router, it MUST send both types at the configured rate, even if it is sub-second. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no Active Router sends both versions |
| `RFC9568-8.4.2-2` | When a Virtual Router is configured this way and is the Backup Router, it MUST time out based on the rate advertised by the Active Router. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no Backup Router times out on a VRRPv2 Active Router's rate |
| `RFC9568-8.4.2-3` | In the case of a VRRPv2 Active Router, this means it MUST translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | MUST | 8.4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no group translates a VRRPv2 interval into centiseconds, although per-version conversion helpers exist in internal/plugins/vrrp/packet/packet.go |
| `RFC9568-2.5-1` | If the Active Router observes that this is occurring, it SHOULD log the problem (subject to rate-limiting). (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-7` | If any one of the above checks fails, the receiver MUST discard the packet, SHOULD log the event (subject to rate-limiting) (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-8` | A receiver SHOULD also verify that the Max Advertise Interval in the received VRRP packet matches the Advertisement_Interval configured for the VRID. Instability can occur with differing intervals (refer to Section 5.2.7). If this check fails, the receiver SHOULD log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-9` | If this check fails, the receiver SHOULD log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. A receiver MAY also verify that "IPvX Addr Count" and the list of IPvX address(es) match the IPvX address(es) configured for the VRID. If this check fails, the receiver SHOULD log (subject to rate- limiting) the event (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-7.1-12` | It SHOULD verify that the local router is not the IPvX address owner (Priority = 255 (decimal)) and log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected. (§7.1, erratum 8298) | SHOULD | 7.1 | **positive:** `unit/verify` [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_owner_conflict_test.go#L65). **negative:** `unit/verify` [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_owner_conflict_test.go#L66) |
| `RFC9568-8.1.1-1` | The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision. (§8.1.1; lowercase "should" in the RFC) | SHOULD | 8.1.1 | **positive:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L30). **negative:** `unit/verify` [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L31) |
| `RFC9568-8.1.2-3` | When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner (as defined in Section 1.7), and it should only send ARP messages that include Virtual Router MAC addresses. (§8.1.2) | SHOULD NOT | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-4` | * When configuring an interface, Active Routers SHOULD broadcast a gratuitous ARP message containing the Virtual Router MAC address for each IPv4 address on that interface. (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.1.2-5` | * When, for example, Secure Shell (SSH) access to a particular VRRP Router is required, an IPv4 address known to belong to that router SHOULD be used. (§8.1.2) | SHOULD | 8.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9568-8.2.1-1` | The IPv6 source address of an ICMPv6 redirect SHOULD be the address that the end-host used when making its next-hop routing decision. (§8.2.1) | SHOULD | 8.2.1 | **positive:** `unit/verify` [`TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L97). **negative:** `unit/verify` [`TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L98) |
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
| [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1) Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2) | {gap}, no test | ze is the router this binds, and the forwarding decision is the dataplane's, not the VRRP plugin's: onPacket (internal/plugins/vrrp/instance.go) consumes every received advertisement and re-emits nothing, but whether the kernel or the VPP backend ze programs forwards a transit datagram to 224.0.0.18 is not shown, and no test reads either backend's behaviour or names a verified layer token, so the row is a gap |
| [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1) Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2) | {gap}, no test | ze is the router this binds, and the forwarding decision is the dataplane's, not the VRRP plugin's: onPacket (internal/plugins/vrrp/instance.go) consumes every received advertisement and re-emits nothing, but whether the kernel or the VPP backend ze programs forwards a transit datagram to ff02::12 is not shown, and no test reads either backend's behaviour or names a verified layer token, so the row is a gap |
| [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3) - It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2) | {gap}, no test | the VRRP plugin builds no Router Advertisement in any state, since its only ICMPv6 message is the unsolicited Neighbor Advertisement (icmpv6TypeNA in internal/plugins/vrrp/transport/na.go), but ze carries a Router Advertisement builder elsewhere (internal/core/ndp/ra.go) and no test proves that a Backup's interface sends none for the Virtual Router; the Active-side emission is the gap RFC9568-6.4.3-4 |
| [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4) It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3) | {gap}, no test | ze runs IPv6 Virtual Routers but sends no Router Advertisement for them -- the announcer's only IPv6 frame is the unsolicited Neighbor Advertisement, icmpv6TypeNA 136 at internal/plugins/vrrp/transport/na.go:25 built by BuildNA na.go:56 and selected by frameBuilder internal/plugins/vrrp/transport/transport.go:511, and no RA builder or RA socket exists in the plugin |
| [`RFC9568-8.2.2-2`](#rfc9568-8.2.2-2) When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link- layer address option in the Neighbor Solicitation message. (§8.2.2) | {gap}, no test | the kernel fills the option from the device the host's route resolves through (Linux limitation LL-NS-SOURCE-MAC, docs/architecture/linux-limitations.md), and ze installs nothing that makes that device the virtual-MAC macvlan: an owned VIP sits there only as a /128 (vipMaskBits internal/plugins/vrrp/register.go), so the subnet route stays on the parent and the option carries the parent's physical MAC; no test observes the option; fix owned by plan/immediate/spec-vrrp-owner-arp-rfc-defects.md |
| [`RFC9568-8.2.2-3`](#rfc9568-8.2.2-3) It MUST NOT use its physical MAC address in the source link-layer address option. (§8.2.2) | {gap}, no test | the kernel fills the option from the device the host's route resolves through (Linux limitation LL-NS-SOURCE-MAC, docs/architecture/linux-limitations.md), and ze installs nothing that makes that device the virtual-MAC macvlan: an owned VIP sits there only as a /128 (vipMaskBits internal/plugins/vrrp/register.go), so the subnet route stays on the parent and the option carries the parent's physical MAC; no test observes the option; fix owned by plan/immediate/spec-vrrp-owner-arp-rfc-defects.md |
| [`RFC9568-8.2.3-1`](#rfc9568-8.2.3-1) The Backup Routers MUST be configured to send the same Router Advertisement options as the address owner. (§8.2.3) | {gap}, no test | the obligation rests on the Router Advertisement ze does not send, which is the gap at RFC9568-6.4.3-4. The vrrp group carries no Router Advertisement leaf at all -- applyGroupLeaves internal/plugins/vrrp/groups.go accepts vrid, virtual-address, priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, accept-mode and version only -- so there is no option set a Backup could be configured to match |
| [`RFC9568-8.4.2-1`](#rfc9568-8.4.2-1) When a Virtual Router is configured this way and is the Active Router, it MUST send both types at the configured rate, even if it is sub-second. (§8.4.2) | {gap}, no test | ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no Active Router sends both versions |
| [`RFC9568-8.4.2-2`](#rfc9568-8.4.2-2) When a Virtual Router is configured this way and is the Backup Router, it MUST time out based on the rate advertised by the Active Router. (§8.4.2) | {gap}, no test | ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no Backup Router times out on a VRRPv2 Active Router's rate |
| [`RFC9568-8.4.2-3`](#rfc9568-8.4.2-3) In the case of a VRRPv2 Active Router, this means it MUST translate the timeout value it receives (in seconds) into centiseconds. (§8.4.2) | {gap}, no test | ze implements no Section 8.4.2 VRRPv2 and VRRPv3 interoperation mode: a group runs exactly one version (parseVersion in internal/plugins/vrrp/groups.go), advertises in that version only (doSendAdvert in internal/plugins/vrrp/instance.go), and the receive path discards an advertisement of another version with ErrVersion (internal/plugins/vrrp/packet/validate.go); no owner decision declines the mode, so no group translates a VRRPv2 interval into centiseconds, although per-version conversion helpers exist in internal/plugins/vrrp/packet/packet.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9568-5.1.1.2-1`](#rfc9568-5.1.1.2-1)

Routers MUST NOT forward a datagram with this destination address, regardless of its TTL. (§5.1.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.1.2-1, so no unit is bound to it.

### [`RFC9568-5.1.1.3-1`](#rfc9568-5.1.1.3-1)

The TTL MUST be set to 255. (§5.1.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 5.1.1.3 says "The TTL MUST be set to 255." TestSendAdvertV3IPv4HeaderTTLProtoDst changed only by the checksum-mode plumbing (a Checksum: packet.ChecksumRFC9568 field added to the GroupSpec, AdvertParams or packet.Local literal, or the mode argument added to packet.FillChecksum); every assertion and every isolating input is byte-for-byte the same; it still requires byte 8 of the sent frame to be 255. Single-polarity. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L316) | unit/verify | unproven |

### [`RFC9568-5.1.1.3-2`](#rfc9568-5.1.1.3-2)

A VRRP Router receiving a packet with the TTL not equal to 255 MUST discard the packet [RFC5082]. (§5.1.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 5.1.1.3 says "A VRRP Router receiving a packet with the TTL not equal to 255 MUST discard the packet [RFC5082]." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. N4 with TTL 64 still requires ErrTTL, and TestInstanceRxDiscardsBadTTLTypeAndCountZero, unchanged, proves the discard. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L485) | unit/verify | unproven |
| negative | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L70) | unit/verify | unproven |
| positive | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L68) | unit/verify | revert, verified |

### [`RFC9568-5.1.2.2-1`](#rfc9568-5.1.2.2-1)

Routers MUST NOT forward a datagram with this destination address, regardless of its Hop Limit. (§5.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-5.1.2.2-1, so no unit is bound to it.

### [`RFC9568-5.1.2.3-1`](#rfc9568-5.1.2.3-1)

The Hop Limit MUST be set to 255. (§5.1.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after d3195edb70 rewrote the row annotation and openV6 changed on 2026-10-27): RFC 9568 Section 5.1.2.3 says "The Hop Limit MUST be set to 255." openV6 (internal/plugins/vrrp/transport/backend_linux.go) still sets IPV6_MULTICAST_HOPS 255 on the advertisement tx socket, and advertisements go only to ff02::12 through SendAdvert's Sendmsg on that socket, so the multicast hop limit is the one every advertisement carries; openNASocket sets the same on the NA socket. TestIntegrationOpenInstanceSocketOptions (//go:build linux, opens with userns.Enter, which runs it unprivileged in its own user, network and mount namespace and carries the child's fail or skip back) reads IPV6_MULTICAST_HOPS back with getsockopt from both the tx and NA socket and requires exactly 255; the kernel default is 1, so an openV6 that stopped setting it fails, and the recorded mutant (255 -> 254 in openV6) went red. Single-polarity per the row annotation: a socket constant no input changes; receive-side discard is the separate -5.1.2.3-2 row. The rewritten annotation now matches the test. Findings outside this verdict: the tag's claim still cites backend_linux.go:185 where the call is now at :187, and the summary's Support remaining text still calls this evidence integration-gated, QEMU-only and unreachable by ./le rfc record, which is no longer true. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationOpenInstanceSocketOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_integration_linux_test.go#L260) | unit/verify | mutant, verified |

### [`RFC9568-5.1.2.3-2`](#rfc9568-5.1.2.3-2)

A VRRP Router receiving a packet with the Hop Limit not equal to 255 MUST discard the packet [RFC5082]. (§5.1.2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): Same unit, IPv6: Hop Limit 255 reaches the FSM; 0, 64, 254 record exactly ttl and never reach it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L615) | unit/verify | unproven |
| negative | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L96) | unit/verify | unproven |
| positive | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L70) | unit/verify | revert, verified |

### [`RFC9568-5.2.2-1`](#rfc9568-5.2.2-1)

A packet with unknown type MUST be discarded. (§5.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 5.2.2 says "A packet with unknown type MUST be discarded." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. Type 2 still requires ErrType before the VRID lookup. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L144) | unit/verify | unproven |
| negative | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L72) | unit/verify | unproven |
| positive | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L72) | unit/verify | revert, verified |

### [`RFC9568-5.2.4-1`](#rfc9568-5.2.4-1)

The priority value for the VRRP Router that owns the IPvX address associated with the Virtual Router MUST be 255 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 re-read after a tag was added to TestEffectivePriorityWithTracking (body unchanged): TestOwnerAutoDetection requires EffectivePriority 255 for the owner configured at 120 and the configured 100 for a non-owner; TestEffectivePriorityWithTracking keeps the owner at 255 under any decrement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L813) | unit/verify | revert, verified |
| positive | [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1620) | unit/verify | revert, verified |
| positive | [`TestOwnerAutoDetection`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L811) | unit/verify | revert, verified |

### [`RFC9568-5.2.4-2`](#rfc9568-5.2.4-2)

VRRP Routers backing up a Virtual Router MUST use priority values between 1-254 (decimal). (§5.2.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 re-read after a tag was added to TestEffectivePriorityWithTracking (body unchanged): TestBoundaryPriority accepts 1 and 254 and rejects 0 and 255 through extractGroupSpecs/validateGroups; TestEffectivePriorityWithTracking floors a tracked decrement at 1

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L321) | unit/verify | unproven |
| positive | [`TestBoundaryPriority`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L319) | unit/verify | unproven |
| positive | [`TestEffectivePriorityWithTracking`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L1621) | unit/verify | revert, verified |

### [`RFC9568-5.2.5-1`](#rfc9568-5.2.5-1)

If the received count is 0, the VRRP advertisement MUST be ignored. (§5.2.5, erratum 8299)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 5.2.5 says "If the received count is 0, the VRRP advertisement MUST be ignored." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. The Count 0 advert still requires ErrCountZero. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L495) | unit/verify | unproven |
| negative | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L74) | unit/verify | unproven |
| positive | [`TestInstanceRxDiscardsBadTTLTypeAndCountZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_discard_test.go#L74) | unit/verify | revert, verified |

### [`RFC9568-5.2.6-1`](#rfc9568-5.2.6-1)

The Reserve field MUST be set to zero on transmission and ignored on reception. (§5.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 5.2.6 says "The Reserve field MUST be set to zero on transmission and ignored on reception." TestEncodeGoldenV3IPv4 now runs a subtest per checksum mode, and each still requires the Reserve nibble to be zero and the exact golden bytes; the receive half TestDecodeV3ReserveIgnoredOnReceive is unchanged. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3ReserveIgnoredOnReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L589) | unit/verify | unproven |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L107) | unit/verify | revert, verified |

### [`RFC9568-5.2.9-1`](#rfc9568-5.2.9-1)

For IPv6, the first address MUST be the IPv6 link-local address associated with the Virtual Router. (§5.2.9, §6.1, erratum 8300)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestValidateIPv6LinkLocal refuses a global-first IPv6 group and accepts a link-local-first one (transmit side); TestValidationOrder row 13 rejects a received global-first advert with ErrFirstNotLinkLocal

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L553) | unit/verify | unproven |
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L253) | unit/verify | unproven |
| positive | [`TestValidateIPv6LinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L551) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L99) | unit/verify | unproven |

### [`RFC9568-5.2.9-2`](#rfc9568-5.2.9-2)

The address family of the addresses, IPv4 or IPv6 but not both, MUST be the same as the VRRP packet's IPvX header address family. (§5.2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent): the cascade confound is removed: the IPv6 group's IPv4 address now follows fe80::1, so the first-address-link-local rule cannot refuse it, and every refusal must carry the family error text ('is not an IPv6 address' / 'is not an IPv4 address'); same-family IPv4 and IPv6 lists validate. Records: revert validateGroup, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L577) | unit/verify | revert, verified |
| positive | [`TestValidateVIPFamilyMatchesGroupFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/groups_test.go#L575) | unit/verify | revert, verified |

### [`RFC9568-6.1-1`](#rfc9568-6.1-1)

Note: IPv6 Neighbor Solicitations and Neighbor Advertisements MUST NOT be dropped when Accept_Mode is False. (§6.1, §6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop requires ICMPv6 135 and 136 accept terms ahead of the first drop term in the Accept_Mode False table, and fails if the table has no drop

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L111) | unit/verify | unproven |
| positive | [`TestAcceptFilterAcceptsNeighborDiscoveryBeforeAnyDrop`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-1`](#rfc9568-6.4.2-1)

- It MUST NOT respond to ARP requests for the IPv4 address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: TestInstanceStartupNonOwnerGoesBackup requires a Backup to install no virtual address, the state Ze hands the kernel that answers ARP; TestInstanceOwnerStartupGoesMaster is the Active contrast

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L454) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L417) | unit/verify | unproven |

### [`RFC9568-6.4.2-2`](#rfc9568-6.4.2-2)

- It MUST NOT respond to ND Neighbor Solicitation messages for the IPv6 address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestInstanceIPv6VIPLivesOnVirtualMACDevice requires a Backup to install no IPv6 virtual address and the Active router to install it on the vMAC macvlan only

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L740) | unit/verify | unproven |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L738) | unit/verify | unproven |

### [`RFC9568-6.4.2-3`](#rfc9568-6.4.2-3)

- It MUST NOT send ND Router Advertisement messages for the Virtual Router. (§6.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.2-3, so no unit is bound to it.

### [`RFC9568-6.4.2-4`](#rfc9568-6.4.2-4)

* It MUST discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 6.4.2 says "MUST discard packets with a destination link-layer MAC address equal to the Virtual Router MAC address." TestVRRPBackupDoesNotForwardVirtualMACFrames is otherwise unchanged: with the recipe in force and no filter a datagram sent to the virtual MAC is forwarded (control), with setBackupFilter the same datagram is not forwarded, each asserted from a capture on the next hop. TestBackupDiscardFollowsTheState (unchanged) still proves the worker sets and withdraws the discard with the state. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L111) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L126) | unit/verify | revert, verified |

### [`RFC9568-6.4.2-5`](#rfc9568-6.4.2-5)

* It MUST NOT accept packets addressed to the IPvX address(es) associated with the Virtual Router. (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: TestInstanceStartupNonOwnerGoesBackup requires no virtual address installed in Backup, so the kernel accepts nothing addressed to it; Active contrast installs it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L456) | unit/verify | unproven |
| positive | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L419) | unit/verify | unproven |

### [`RFC9568-6.4.2-6`](#rfc9568-6.4.2-6)

If a Shutdown event is received, then: - Cancel the Active_Down_Timer - Transition to the {Initialize} state (§6.4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestFSMTransitionMatrix backup/shutdown requires exactly StopTimers and a transition to Initialize; the negative is a contrast row (stale timer event keeps Backup)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L111) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L109) | unit/verify | unproven |

### [`RFC9568-6.4.2-7`](#rfc9568-6.4.2-7)

If the Active_Down_Timer fires, then: - Send an ADVERTISEMENT - If the protected IPvX address is an IPv4 address, then: o For each IPv4 address associated with the Virtual Router, broadcast a gratuitous ARP message containing the Virtual Router IPv4 address and with the target link-layer address set to the Virtual Router MAC address. - else // IPv6 o Compute and join the Solicited-Node multicast address [RFC4291] for the IPv6 address(es) associated with the Virtual Router. o For each IPv6 address associated with the Virtual Router, send an unsolicited ND Neighbor Advertisement with the Router Flag (R) set, the Solicited Flag (S) clear, the Override flag (O) set, the target address set to the IPv6 address of the Virtual Router, and the target link-layer address set to the Virtual Router MAC address. - endif // was protected address IPv4? - Set the Adver_Timer to Advertisement_Interval - Transition to the {Active} state (§6.4.2, erratum 7949)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp final judge 2026-09-29 (independent): earlier proof stands (promotion action list with StartAdvertTimer, GARP per IPv4 VIP with the Virtual Router MAC, NA per IPv6 VIP R=1 S=0 O=1 with the Virtual Router MAC). The Solicited-Node clause is now asserted at the boundary Ze owns: TestPromotionInstallsIPv6AddressesOnVirtualMACDevice requires an IPv6 Backup to install nothing, then on its own down timer to install exactly fe80::1 and 2001:db8::1 on the virtual-MAC device and never the parent; that address add is what makes Linux addrconf join each Solicited-Node group (Ze builds no MLD). Observed-red record: doInstallVIPs. Kernel membership itself is not read.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMStaleTimerGenerationIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L638) | unit/verify | unproven |
| positive | [`TestFSMMasterDownPromotion`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L447) | unit/verify | unproven |
| positive | [`TestPromotionSetsAdverTimerToOwnInterval`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/rfc_verdict_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestPromotionInstallsIPv6AddressesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_v6_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestAnnounceMasterFramesPerAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_announce_verdict_test.go#L53) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 6.4.3 says "MUST respond to ARP requests for the IPv4 address(es) associated with the Virtual Router." TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly, otherwise unchanged, observes ARP replies for the virtual address held on the virtual-MAC macvlan and none once the address is removed; the sender hardware address read is arp[8:14]. TestPromotionInstallsIPv4AddressOnVirtualMACDevice (unchanged) proves the product installs that address on promotion only. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L142) | unit/verify | unproven |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L71) | unit/verify | unproven |
| positive | [`TestPromotionInstallsIPv4AddressOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/promote_install_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L67) | unit/verify | revert, verified |

### [`RFC9568-6.4.3-2`](#rfc9568-6.4.3-2)

- It MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the Virtual Router. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after f1b637444b tagged a second unit and corrected the row annotation): RFC 9568 Section 6.4.3 says an Active router protecting an IPv6 address "MUST be a member of the Solicited-Node multicast address for the IPv6 address(es) associated with the Virtual Router." Two producers meet it: doInstallVIPs (internal/plugins/vrrp/instance.go) calls joinSolicitedNode (solicitednode_linux.go), which adds a link-layer PACKET_MR_MULTICAST membership for 33:33:ff:xx:xx:xx on the virtual-MAC macvlan and syncs the receive filter down the device stack before returning, then registers the addresses, whose install on that macvlan makes the kernel take the IPv6 group membership itself. TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled (//go:build linux, userns.Enter) drives the real FSM and promotion with the real joinSolicitedNode and address-owner registry on a macvlan with the product's recipe: it requires no membership as Backup, the group in the macvlan's /proc/thread-self/net/dev_mcast and synced onto the parent when the promotion returns while the address is provably not installed, a reconfiguration while Active to add the new group, drop the removed one and never drop the shared fe80::1 group (sampled concurrently), the group gone after demotion, and after a second promotion and install a single multicast solicitation answered with the Virtual Router MAC, which shows the group is functional at the IPv6 level, not just listed. The recorded mutant (solicitedNodeMACs(nil) in joinSolicitedNode) went red. TestInstanceIPv6VIPLivesOnVirtualMACDevice (instance_test.go) covers the other half: the Active router installs every IPv6 virtual address on the virtual-MAC device and never on the parent, so the kernel's membership lands on the device that answers; a Backup installs none. Single-polarity per the row annotation: the not-a-member case is the Backup requirement RFC9568-6.4.2-2 (both units still assert it). Finding outside this verdict: the unit-test tag cites instance.go:369 for doInstallVIPs, which now sits near line 617. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L742) | unit/verify | unproven |
| positive | [`TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_solicited_node_integration_linux_test.go#L58) | unit/verify | mutant, verified |

### [`RFC9568-6.4.3-3`](#rfc9568-6.4.3-3)

- It MUST respond to ND Neighbor Solicitation messages (with the Router Flag (R) set) for the IPv6 address(es) associated with the Virtual Router. (§6.4.3, §8.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPSolicitedNACarriesRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_nd_router_flag_integration_linux_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestVRRPSolicitedNACarriesRouterFlag`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_nd_router_flag_integration_linux_test.go#L36) | unit/verify | revert, verified |

### [`RFC9568-6.4.3-4`](#rfc9568-6.4.3-4)

It MUST send ND Router Advertisements for the Virtual Router. (§6.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9568-6.4.3-4, so no unit is bound to it.

### [`RFC9568-6.4.3-5`](#rfc9568-6.4.3-5)

* It MUST forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 6.4.3 says "MUST forward packets with a destination link-layer MAC address equal to the Virtual Router MAC address." TestVRRPBackupDoesNotForwardVirtualMACFrames, otherwise unchanged, forwards the datagram to the next hop once the filter is withdrawn and the addresses installed, and not while the filter holds, read from the capture. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L117) | unit/verify | revert, verified |
| negative | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestBackupDiscardFollowsTheState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/backupfilter_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestVRRPBackupDoesNotForwardVirtualMACFrames`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L132) | unit/verify | revert, verified |

### [`RFC9568-6.4.3-6`](#rfc9568-6.4.3-6)

* It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): Re-read: the unit's body is unchanged; only the 6.4.2 and 6.4.3 Virtual Router MAC proxy tags left it for TestVRRPBackupDoesNotForwardVirtualMACFrames, so the verdict stands. Prior note: vrrp judge 2026-09-29 re-read after RFC5798 tags were added to TestActiveAddressOwnerAcceptsWhateverAcceptModeSays (body unchanged): owner installs and accepts (instance and acceptfilter tests), Accept_Mode True hands no suppression, and a non-owner with Accept_Mode False gets a suppression: both conditions and the contrast are asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L205) | unit/verify | unproven |
| negative | [`TestInstanceStartupNonOwnerGoesBackup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L421) | unit/verify | unproven |
| positive | [`TestActiveAddressOwnerAcceptsWhateverAcceptModeSays`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L267) | unit/verify | unproven |
| positive | [`TestActiveNonOwnerWithAcceptModeTrueAcceptsLocalDelivery`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/acceptfilter_test.go#L241) | unit/verify | unproven |
| positive | [`TestInstanceOwnerStartupGoesMaster`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L453) | unit/verify | unproven |

### [`RFC9568-6.4.3-7`](#rfc9568-6.4.3-7)

It MUST accept packets addressed to the IPvX address(es) associated with the Virtual Router if it is the IPvX address owner or if Accept_Mode is True. Otherwise, it MUST NOT accept these packets. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 re-read after RFC5798 tags were added to TestActiveAddressOwnerAcceptsWhateverAcceptModeSays (body unchanged): TestActiveNonOwnerWithAcceptModeFalseSuppressesLocalDelivery requires accept=false for exactly the VIP; Accept_Mode True and owner contrasts require accept=true

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
| positive | [`TestInstanceShutdownAsMasterSendsPriorityZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L524) | unit/verify | unproven |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestInstanceIPv6ElectionUsesUnsignedOrder closes both named gaps: the IPv6 operand through the engine (link-local advertisement source) and the unsigned network-byte-order compare (fe80::80 vs fe80::7f, which a signed octet compare inverts). Yield rows assert Adver_Timer cancelled, Active_Adver_Interval set to the advert's 3000ms, Skew_Time and Active_Down_Interval recomputed from it, Active_Down_Timer armed, Backup; the hold row keeps Active, timer armed, 1000ms. With TestInstanceTieBreakComparesTheAdvertisementSource (IPv4). Records: senderWinsTieBreak, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L137) | unit/verify | unproven |
| negative | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L77) | unit/verify | revert, verified |
| negative | [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L135) | unit/verify | unproven |
| positive | [`TestInstanceTieBreakComparesTheAdvertisementSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6ElectionUsesUnsignedOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/tiebreak_v6_test.go#L49) | unit/verify | revert, verified |

### [`RFC9568-6.4.3-12`](#rfc9568-6.4.3-12)

else // new Active Router logic + Discard the ADVERTISEMENT + Send an ADVERTISEMENT immediately to assert the {Active} state to the sending VRRP Router and to update any learning bridges with the correct Active VRRP Router path. (§6.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. losing lower-priority, smaller-ip and equal-ip rows require exactly one SendAdvert and state Active; the winning row demotes instead

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L141) | unit/verify | unproven |
| positive | [`TestFSMTransitionMatrix`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/fsm/fsm_test.go#L139) | unit/verify | unproven |

### [`RFC9568-7.1-1`](#rfc9568-7.1-1)

* It MUST verify that the VRRP version is 3. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "It MUST verify that the VRRP version is 3." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. N9 still requires ErrVersion for a v2 advert at a v3 group. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L561) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L75) | unit/verify | unproven |

### [`RFC9568-7.1-2`](#rfc9568-7.1-2)

* It MUST verify that the VRRP packet type is 1 (ADVERTISEMENT). (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "It MUST verify that the VRRP packet type is 1 (ADVERTISEMENT)." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. TestValidationOrder still requires ErrType for type 2 before the VRID lookup. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L146) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L77) | unit/verify | unproven |

### [`RFC9568-7.1-3`](#rfc9568-7.1-3)

* It MUST verify that the received packet contains the complete VRRP packet (including fixed fields and the IPvX address). (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "It MUST verify that the received packet contains the complete VRRP packet (including fixed fields and the IPvX address)." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. TestDecodeV3DiscardsIncompletePacket switched its IPv4 golden to G2 and recomputes in the default form, so the truncation and short-address cases still fail on ErrTruncated or ErrLength rather than ErrChecksum. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L151) | unit/verify | revert, verified |
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L461) | unit/verify | unproven |
| positive | [`TestDecodeV3DiscardsIncompletePacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L78) | unit/verify | unproven |

### [`RFC9568-5.2.8-1`](#rfc9568-5.2.8-1)

It MUST verify the VRRP checksum. (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "It MUST verify the VRRP checksum." TestDecodeVerifiesChecksumBothFamilies lost only the assertion on the removed MsgOnlyChecksum flag; it still accepts the message-only IPv4 golden and the IPv6 golden under the default mode and refuses a flipped checksum bit in each family with ErrChecksum. An overlay mutant that made the default mode sum the IPv4 pseudo-header reddened it. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeVerifiesChecksumBothFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L288) | unit/verify | revert, verified |
| negative | [`TestDecodeV3IPv6ChecksumAndHopLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L613) | unit/verify | unproven |
| positive | [`TestDecodeVerifiesChecksumBothFamilies`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc_rx_verdict_test.go#L287) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L98) | unit/verify | unproven |

### [`RFC9568-5.2.8-2`](#rfc9568-5.2.8-2)

For the IPv4 address family, the checksum calculation only includes the VRRP message starting with the Version field and ending after the last IPv4 address (refer to Section 5.2). (§5.2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after d2abad70ad). RFC 9568 Section 5.2.8 says "For the IPv4 address family, the checksum calculation only includes the VRRP message starting with the Version field and ending after the last IPv4 address (refer to Section 5.2)." Producers: pseudoSumFor returns no pseudo-header for ChecksumRFC9568, FillChecksum applies it on transmit and verifyReceived on receive in the one configured form. Positive TestEncodeGoldenV3IPv4 pins G2 0x828A (re-derived independently over the 16-octet message; the pseudo-header sum for 192.0.2.251 to 224.0.0.18 is 0xDEFB). Positive/negative TestDecodeV3IPv4ChecksumOnlyConfiguredForm: the default mode accepts G2 and refuses G2c with ErrChecksum, and each mode refuses the other form, in separate subtests, not one assertion wearing two hats. Overlay mutants run 2026-10-18 over the whole vrrp tree: restoring dual-accept in verifyReceived reds the two refusal subtests; summing the IPv4 pseudo-header under ChecksumRFC9568 reds the golden and the default accept and refuse subtests. The hop that previously made this weak is closed: doSendAdvert hard-coded to ChecksumRFC5798Compatible, or with Checksum dropped, now reds TestTransmittedAdvertFilledFromInstanceState, which is tagged RFC9568-7.2-5 rather than this row; the config default (groups.go struct literal) is caught only by the untagged TestLeafShapesFromTextConfig "checksum absent is rfc9568". Both are hops that choose the form, owned by the 7.2 transmit step and the parser, while this row's own obligation, what the computation covers, is pinned at its producers in both directions. Claims match assertions. No tagged unit is a QEMU-guest unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv4ChecksumOnlyConfiguredForm`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestDecodeV3IPv4ChecksumOnlyConfiguredForm`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L111) | unit/verify | revert, verified |

### [`RFC9568-5.2.8-3`](#rfc9568-5.2.8-3)

For the IPv6 address family, the checksum calculation also includes a prepended "pseudo-header", as defined in Section 8.1 of [RFC8200]. (§5.2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp closure judge 2026-09-30 (independent): TestDecodeV3IPv6ChecksumCoversPseudoHeader recomputes the IPv6 golden's checksum with refChecksum (independent of checksum.go): the RFC 8200 pseudo-header plus message sum is accepted, the message-only sum is refused with ErrChecksum, so a v6 verifier that dropped the pseudo-header or fell back to message-only (as the v4 path does) goes red. Producer verifyReceived family==V6 uses pseudoSumV6 only. Both polarities recorded observed red (pseudoSumV6, verifyReceived).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeV3IPv6ChecksumCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc9568_ipv6_pseudo_header_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestDecodeV3IPv6ChecksumCoversPseudoHeader`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/rfc9568_ipv6_pseudo_header_test.go#L21) | unit/verify | revert, verified |

### [`RFC9568-7.1-4`](#rfc9568-7.1-4)

It MUST verify that the VRID is configured on the receiving interface. (§7.1, erratum 8298)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 7.1 says "It MUST verify that the VRID is configured on the receiving interface" TestRxAdvertScopedToReceivingInterface sends, for v2 and v3, a VRID 10 advert on the interface that holds only VRID 20 and fails if it reaches the VRID 10 instance, requires it to arrive at the VRID 20 instance (so absence is not vacuous) and decode there as ErrUnknownVRID, then requires the same advert on the VRID 10 interface to reach its instance and decode. Its other change is rp_filter 0 on the second macvlan so the IPv4 multicast is received at all. The unchanged packet/validate_test.go and rfc9568_rx_interface_v3_test.go tags stand. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L157) | unit/verify | unproven |
| negative | [`TestRxV3VRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_interface_v3_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L80) | unit/verify | unproven |
| positive | [`TestRxV3VRIDCheckedOnTheReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_rx_interface_v3_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRxAdvertScopedToReceivingInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go#L130) | unit/verify | revert, verified |

### [`RFC9568-7.1-5`](#rfc9568-7.1-5)

It MUST verify that the Max Advertise Interval is non zero. (§7.1, erratum 8301)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "It MUST verify that the Max Advertise Interval is non zero." TestDecodeGoldenV3IPv4 now decodes the RFC 9568 message-only golden G2 under the default-mode lookup instead of the pseudo-header golden G2c, and lost only the removed MsgOnlyChecksum assertion; the subtests of TestNegativeReferenceBugs other than N1 gained only the FillChecksum mode argument. The positive is still a fully conforming advert, the checksum check precedes the checks this row names so a wrong form would fail with ErrChecksum and not pass silently, and each negative still asserts its exact sentinel. N6 still zeroes the interval, recomputes the checksum and requires ErrIntervalZero. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegativeReferenceBugs`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L505) | unit/verify | unproven |
| positive | [`TestDecodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/validate_test.go#L81) | unit/verify | unproven |

### [`RFC9568-7.1-6`](#rfc9568-7.1-6)

If any one of the above checks fails, the receiver MUST discard the packet (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after a78870fd39). RFC 9568 Section 7.1 says "If any one of the above checks fails, the receiver MUST discard the packet". TestInstanceRxValidAdvertReachesFSM changed only by the checksum-mode plumbing (a Checksum: packet.ChecksumRFC9568 field added to the GroupSpec, AdvertParams or packet.Local literal, or the mode argument added to packet.FillChecksum); every assertion and every isolating input is byte-for-byte the same. TestInstanceRxFailedCheckIsDiscarded is unchanged, and an overlay mutant giving instance.lookup the wrong checksum mode reddened both, so every mandatory check still discards through onPacket. Verdict unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceRxDecodeErrorMapsReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L548) | unit/verify | revert, verified |
| negative | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestInstanceRxValidAdvertReachesFSM`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L573) | unit/verify | unproven |
| positive | [`TestInstanceRxFailedCheckIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rx_discard_test.go#L102) | unit/verify | revert, verified |

### [`RFC9568-7.2-1`](#rfc9568-7.2-1)

The following operations MUST be performed when transmitting a VRRP packet: * Fill in the VRRP packet fields with the appropriate Virtual Router configuration state (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-18 (independent auditor, after d2abad70ad). RFC 9568 Section 7.2 says "The following operations MUST be performed when transmitting a VRRP packet: * Fill in the VRRP packet fields with the appropriate Virtual Router configuration state". TestTransmittedAdvertFilledFromInstanceState requires the params doSendAdvert hands the transport to carry version 3, the effective priority (200, then 150 after eth1 fails), 1000 ms and both VIPs in order, for IPv4 in both checksum modes and IPv6; TestSendAdvertFillsFieldsAndChecksum pins every field of the IPv4 and IPv6 frames encodeLocked produces; the v3 goldens pin WriteTo. Overlay mutants run 2026-10-18 over the whole vrrp tree: doSendAdvert filling the configured priority instead of the effective one reds all three subtests of TestTransmittedAdvertFilledFromInstanceState. The {single-polarity} argument holds for WriteTo, which refuses nothing; encodeLocked does refuse through Validate (ErrIntervalRange, ErrCountRange), a guard RFC5798-7.2-1 carries as its negative. Nit: the annotation line numbers are stale (encodeLocked is at transport.go:537, WriteTo at packet.go:246). No tagged unit is a QEMU-guest unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv6`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L185) | unit/verify | revert, verified |
| positive | [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L39) | unit/verify | revert, verified |

### [`RFC9568-7.2-5`](#rfc9568-7.2-5)

* Compute the VRRP checksum (§7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-10-31 (independent auditor, authored none of 7682a5dc93). RFC 9568 Section 7.2 says "The following operations MUST be performed when transmitting a VRRP packet: ... * Compute the VRRP checksum", in the Section 5.2.8 form: message-only for IPv4, and for IPv6 "the checksum calculation also includes a prepended \"pseudo-header\", as defined in Section 8.1 of [RFC8200]." 7682a5dc93 moved openV6's RX socket from the parent to the instance MACVLAN; the TX socket still sets IPV6_CHECKSUM 6, so the kernel still computes the IPv6 checksum with its pseudo-header. Tagged units are byte-identical to the 2026-10-18 judgement. IPv4: proven (TestTransmittedAdvertFilledFromInstanceState asserts p.Checksum == spec.Checksum in both modes; TestSendAdvertFillsFieldsAndChecksum computes the expected sums itself; the compatible-mode subtest is the negative). IPv6: still unproven by any tagged unit; the transport test's fake backend carries no IPv6 checksum and the instance test computes FillChecksum in its own body, so removing the setsockopt leaves every tagged unit green. Only the untagged integration QEMU units TestIntegrationOpenInstanceSocketOptions and TestIntegrationAdvertV6OnWire read it. Fix unchanged: tag one positive for this row with its QEMU discrimination record. Verdict unchanged: weak.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestTransmittedAdvertFilledFromInstanceState`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/advert_fill_rfc5798_test.go#L41) | unit/verify | revert, verified |
| positive | [`TestEncodeGoldenV3IPv4`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestSendAdvertFillsFieldsAndChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/advert_fill_checksum_test.go#L37) | unit/verify | revert, verified |

### [`RFC9568-7.2-2`](#rfc9568-7.2-2)

* Set the source MAC address to the Virtual Router MAC address (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 7.2 says "Set the source MAC address to the Virtual Router MAC address" TestTxAdvertV4OnWire (v2 and v3) and TestTxAdvertV6OnWire capture the transmitted frame on the peer veth and compare its source MAC with the literals 00-00-5e-00-01-0a and 00-00-5e-00-02-0a, never the package constants. Single-polarity. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConstants`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/packet_test.go#L436) | unit/verify | unproven |
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L105) | unit/verify | revert, verified |

### [`RFC9568-7.2-3`](#rfc9568-7.2-3)

* If the protected address is an IPv4 address, then: - Set the source IPv4 address to the interface's primary IPv4 address * else // IPv6 - Set the source IPv6 address to the interface's link-local IPv6 address (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 7.2 says "Set the source IPv4 address to the interface's primary IPv4 address" TestTxAdvertV4OnWire adds a lower secondary 192.0.2.5/24 after the primary and requires the captured source to be the primary 192.0.2.251; TestTxAdvertV6OnWire requires the captured source to be the macvlan's link-local address; TestSendAdvertNoLinkLocalSkipsAndCounts (unchanged) is the negative. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSendAdvertNoLinkLocalSkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L412) | unit/verify | revert, verified |
| negative | [`TestSendAdvertNoPrimaryV4SkipsAndCounts`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L364) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestSendAdvertUsesParentPrimaryV4Source`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L218) | unit/verify | revert, verified |

### [`RFC9568-7.2-4`](#rfc9568-7.2-4)

* Set the IPvX protocol to VRRP * Send the VRRP packet to the VRRP IPvX multicast group (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 7.2 says "Send the VRRP packet to the VRRP IPvX multicast group" Both wire tests compare protocol/next header 112 and destination 224.0.0.18 / ff02::12 with literals from the captured frame. Single-polarity. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTxAdvertV4OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestTxAdvertV6OnWire`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestSendAdvertV3IPv4HeaderTTLProtoDst`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/transport/transport_test.go#L318) | unit/verify | unproven |

### [`RFC9568-7.4-1`](#rfc9568-7.4-1)

The Virtual Router MAC MUST NOT be used for the Net_Iface parameter used in the Interface Identifier (IID) derivation algorithms in [RFC7217] and [RFC8981]. (§7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestVRRPMacvlanDerivesNoLinkLocalFromVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L36) | unit/verify | revert, verified |

### [`RFC9568-8.1.1-2`](#rfc9568-8.1.1-2)

If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it must determine to which Virtual Router the packet was sent when selecting the redirect source address. (§8.1.1; lowercase "must" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.1.1 says "If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it must determine to which Virtual Router the packet was sent when selecting the redirect source address." TestVRRPRedirectSourceFollowsVirtualMAC, otherwise unchanged, pins the reverse route through the parent with source 192.0.2.254, then requires each of two virtual MACs to draw a redirect sourced from its own virtual address and the physical MAC to draw one from 192.0.2.254. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L28) | unit/verify | revert, verified |

### [`RFC9568-8.2.1-2`](#rfc9568-8.2.1-2)

If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it has to determine to which Virtual Router the packet was sent when selecting the redirect source address. (§8.2.1; "has to" in the RFC, no BCP 14 keyword; read as the §8.1.1 "must" restated for IPv6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.2.1 says "If a VRRP Router is acting as the Active Router for Virtual Router(s) containing address(es) it does not own, then it has to determine to which Virtual Router the packet was sent when selecting the redirect source address." TestVRRPIPv6RedirectFollowsVirtualMAC, otherwise unchanged, requires the ICMPv6 redirect for a datagram sent to each virtual MAC to come from a link-local held by that macvlan and by neither the other macvlan nor the parent, and the physical-MAC datagram's redirect from a parent link-local. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestVRRPIPv6RedirectFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_v6_integration_linux_test.go#L39) | unit/verify | revert, verified |

### [`RFC9568-8.1.2-1`](#rfc9568-8.1.2-1)

The Active Router MUST NOT respond with its physical MAC address in the ARP response. (§8.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.1.2 says "The Active Router MUST NOT respond with its physical MAC address in the ARP response." TestVRRPOwnerAnswersWithVirtualMACOnly (owner case; its helper ownerAwaitUsable now also syncs the macvlan receive filter before probing, outside the unit) sees the physical MAC as ARP sender hardware address without the owner filter, only the virtual MAC with it, and the physical MAC again once withdrawn; TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly gives the non-owner case with the recipe as control. ownerExpect fails on any MAC other than the two and requires the virtual MAC in every phase. The owner unit still t.Skipf's when the nft backend cannot load, as before. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDataplaneRestoreOnLastGroup`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L144) | unit/verify | unproven |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L51) | unit/verify | revert, verified |
| negative | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L179) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestDataplaneApplyIPv4SetsRecipe`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/dataplane_linux_test.go#L73) | unit/verify | unproven |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L50) | unit/verify | revert, verified |
| positive | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L61) | unit/verify | revert, verified |

### [`RFC9568-8.1.2-6`](#rfc9568-8.1.2-6)

When a host sends an ARP request for one of the Virtual Router IPv4 addresses, the Active Router MUST respond to the ARP request with an ARP response that indicates the Virtual Router MAC address for the Virtual Router. (§8.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.1.2 says "the Active Router MUST respond to the ARP request with an ARP response that indicates the Virtual Router MAC address for the Virtual Router." TestVRRPOwnerAnswersWithVirtualMACOnly (owner case; its helper ownerAwaitUsable now also syncs the macvlan receive filter before probing, outside the unit) sees the physical MAC as ARP sender hardware address without the owner filter, only the virtual MAC with it, and the physical MAC again once withdrawn; TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly gives the non-owner case with the recipe as control. ownerExpect fails on any MAC other than the two and requires the virtual MAC in every phase. The owner unit still t.Skipf's when the nft backend cannot load, as before. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/vmac_state_integration_linux_test.go#L69) | unit/verify | revert, verified |

### [`RFC9568-8.1.2-2`](#rfc9568-8.1.2-2)

* At system boot, when initializing interfaces for VRRP operation, gratuitous ARP messages MUST be delayed until both the IPv4 address and the Virtual Router MAC address are configured. (§8.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestEngineWaitsForALateVirtualMACDevice (ipv4) closes the gap: the live macvlanCreator.create runs over a registry that creates the device late and a transport refusing an absent device, and the order is device, install, gratuitous ARP; a device that never appears gives no instance, install or ARP. Deleting waitDevicePresent reddens the positive (records on register.go create, observed red). TestInstanceDelaysAnnounceUntilParentUsable covers the IPv4 address half.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L790) | unit/verify | unproven |
| positive | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L789) | unit/verify | unproven |

### [`RFC9568-8.1.3-1`](#rfc9568-8.1.3-1)

If Proxy ARP is to be used on a VRRP Router, then the VRRP Router MUST advertise the Virtual Router MAC address in the Proxy ARP message. (§8.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoProxyARPOnVRRPInterfaceAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/proxyarp_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestProxyARPOnVRRPInterfaceRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/proxyarp_test.go#L34) | unit/verify | revert, verified |

### [`RFC9568-8.2.2-1`](#rfc9568-8.2.2-1)

The Active Router MUST NOT respond with its physical MAC address. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.2.2 says "The Active Router MUST NOT respond with its physical MAC address." TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly, otherwise unchanged, sees the physical MAC in the Target Link-Layer Address option with the address on the parent, only the virtual MAC once it lives on the macvlan, and no advertisement once removed; TestVRRPOwnerAnswersWithVirtualMACOnly gives the owner case with and without the owner filter. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L55) | unit/verify | revert, verified |
| negative | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestInstanceIPv6VIPLivesOnVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L744) | unit/verify | unproven |
| positive | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestOwnerFilterWiredOnPromotionForEveryFamily`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/ownerfilter_test.go#L174) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. vrrp judge 2026-09-29 (independent, continuations 1-2): TestEngineWaitsForALateVirtualMACDevice (ipv6): device, install, Neighbor Advertisement order under a late device through the live create, nothing at all when the device never appears; deleting the wait reddens it (records on register.go create, observed red). With TestEngineAnnouncesNothingBeforeTheVirtualMACDevice and TestInstanceDelaysAnnounceUntilParentUsable (address half).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L116) | unit/verify | revert, verified |
| negative | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L793) | unit/verify | unproven |
| positive | [`TestEngineWaitsForALateVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_late_device_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestEngineAnnouncesNothingBeforeTheVirtualMACDevice`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/boot_order_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestInstanceDelaysAnnounceUntilParentUsable`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/instance_test.go#L791) | unit/verify | unproven |

### [`RFC9568-8.2.2-8`](#rfc9568-8.2.2-8)

When a host sends an ND Neighbor Solicitation message for a Virtual Router IPv6 address, the Active Router MUST respond to the ND Neighbor Solicitation message with the Virtual Router MAC address for the Virtual Router. (§8.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.2.2 says "the Active Router MUST respond to the ND Neighbor Solicitation message with the Virtual Router MAC address for the Virtual Router." TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly, otherwise unchanged, sees the physical MAC in the Target Link-Layer Address option with the address on the parent, only the virtual MAC once it lives on the macvlan, and no advertisement once removed; TestVRRPOwnerAnswersWithVirtualMACOnly gives the owner case with and without the owner filter. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/nd_nonowner_integration_linux_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestVRRPOwnerAnswersWithVirtualMACOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/owner_answer_integration_linux_test.go#L62) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-20 (independent auditor, after 30aabfda88). Erratum 8298 (rfc/errata/rfc9568/8298.txt, Verified) corrected text: "It SHOULD verify that the local router is not the IPvX address owner (Priority = 255 (decimal)) and log the event (subject to rate-limiting) and MAY indicate via network management that a misconfiguration was detected." onPacket gained one branch before noteOwnerConflict: an owner in a group configured checksum rfc5798-compatible records ReasonOwner and returns. Every other VRRPv3 group still reaches noteOwnerConflict (owner-conflict counter on every advert, Warn log at most once per ownerConflictLogInterval) and processes the advertisement. TestInstanceV3OwnerConflictLoggedAndProcessed runs under testSpec(), whose Checksum is packet.ChecksumRFC9568, so the new branch is not taken and the test still pins the counter, the log, its rate limit and delivery; the negative (non-owner, nothing counted, no log) is unchanged. PASS at HEAD. Finding, no verdict change: in an rfc5798-compatible IPv4 group the owner now counts under `owner` and writes no log line, so this SHOULD's "log the event" is not done there (an owner-approved, mode-scoped departure toward RFC 5798 Section 7.1 as published, disclosed in docs/guide/vrrp.md). rfc/short/rfc9568.md Support remaining does not mention it and should, since the rfc-compliance rule requires an owner-approved deviation to keep its section and reason in the scope record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_owner_conflict_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestInstanceV3OwnerConflictLoggedAndProcessed`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_owner_conflict_test.go#L65) | unit/verify | revert, verified |

### [`RFC9568-8.1.1-1`](#rfc9568-8.1.1-1)

The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision. (§8.1.1; lowercase "should" in the RFC)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor, after the user-namespace conversion): every tagged unit now opens with `if !userns.Enter(t) { return }`, which re-executes the test in its own user, network and mount namespace and carries the child's fail or skip to the parent (Enter, internal/test/userns/userns_linux.go), and the file builds under plain linux instead of integration && linux. RFC 9568 Section 8.1.1 says "The IPv4 source address of an ICMP redirect should be the address that the end-host used when making its next-hop routing decision." TestVRRPRedirectSourceFollowsVirtualMAC, otherwise unchanged, pins the reverse route through the parent with source 192.0.2.254, then requires each of two virtual MACs to draw a redirect sourced from its own virtual address and the physical MAC to draw one from 192.0.2.254. Verdict enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestVRRPRedirectSourceFollowsVirtualMAC`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/redirect_integration_linux_test.go#L30) | unit/verify | revert, verified |

### [`RFC9568-8.2.1-1`](#rfc9568-8.2.1-1)

The IPv6 source address of an ICMPv6 redirect SHOULD be the address that the end-host used when making its next-hop routing decision. (§8.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestVRRPIPv6RedirectSourcedFromVirtualLinkLocal`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc9568_vmac_link_local_integration_linux_test.go#L97) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc9568.txt |
| Source fingerprint | 98495f2abdfc1a49 |
| Record | rfc/extraction/rfc9568.json |
| Mapped sentences | 49 |
| Declined as scope | 14 |
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
| `8.2.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose with a lowercase "may" noting that extra configuration can be needed; the normative statement is the SHOULD at RFC9568-8.2.4-1. | Additional configuration may be required in order for Unsolicited Neighbor Advertisements to update the corresponding neighbor cache. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Statement that confidentiality is NOT needed, with a lowercase "must"; it removes an obligation rather than stating one. | Confidentiality is not necessary for the correct operation of VRRP, and there is no information in the VRRP messages that must be kept secret from other nodes on the LAN. |

## Superseded

No document obsoletes RFC 9568, so its obligations are stated where they were written.
