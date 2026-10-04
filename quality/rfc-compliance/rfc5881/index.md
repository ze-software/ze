# RFC 5881 - Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop)

Partial. Every requirement this repository extracted from RFC 5881, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.9% | 14 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.3% | 1 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 23 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 88.0% | 44 of 50 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 33 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 23 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 8.7% | 2 of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 23 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 23 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 26.1% | 6 of 23 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 33 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 50 |
| Tagged units | 50 |
| Recorded audit verdicts | 15 |
| Discrimination records | 44 |
| Summary | `rfc/short/rfc5881.md` |
| Requirement shard | `rfc/requirements/rfc5881.md` |
| RFC text | `rfc/full/rfc5881.txt` |

## Enrolment

Enrolled: BFD for IPv4/IPv6 Single Hop (RFC 5881): 12 MET (Control 3784 / Echo 3785 ports, TTL=255 transmit + GTSM receive gate, first-packet + Your-Discriminator demux, Active role, per-protocol sessions, stable transmit destination) + 3 single-polarity positive (fixed single-socket source port, echo reaches remote, on-subnet transmit destination) + 6 gap (source port 3784 not ephemeral 49152-65535, echo uses application reflection not self-addressed/forward-back/redirect-avoiding, point-to-point any-source initial demux unmodelled) + 2 not-applicable (separate L3 path is operator topology, Echo ingress-filtering is host policy)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Single-hop UDP 3784 Control sessions, TTL/GTSM receive gate (TTL=255) plus TTL=255 transmit, first-packet demux by remote address/interface/protocol, Your-Discriminator demultiplexing, Active-role default, stable transmit destination, per-protocol sessions, and echo on UDP 3785
- MUST-level requirements bound per requirement in [`rfc/requirements/rfc5881.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5881.md).


**What the ledger says remains**

Six MUST gaps, gated in [`rfc/short/rfc5881.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5881.md): [`RFC5881-4-2`](#rfc5881-4-2) -- the control socket transmits from port 3784 rather than an ephemeral 49152-65535 source port (single-socket design, [`internal/component/bfd/transport/udp.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp.go)); [`RFC5881-2-4`](#rfc5881-2-4)/4-6/4-7 -- the echo function is application-level ZEEC reflection to the peer ([`internal/component/bfd/engine/echo.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/echo.go)) rather than the RFC self-addressed, forwarding-plane-looped echo, so self-addressing, forward-back destination, and redirect-avoiding source are unimplemented; and [`RFC5881-6-3`](#rfc5881-6-3)/6-4 -- point-to-point links are not modelled distinctly, so the first-packet demux keys on the peer source address ([`internal/component/bfd/engine/loop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/loop.go)) instead of accepting any initial source. IPv6 dual-bind and wider deployment proof remain tracked with BFD.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 14 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (14):** [`RFC5881-2-2`](#rfc5881-2-2), [`RFC5881-3-1`](#rfc5881-3-1), [`RFC5881-3-2`](#rfc5881-3-2), [`RFC5881-4-1`](#rfc5881-4-1), [`RFC5881-4-4`](#rfc5881-4-4), [`RFC5881-4-5`](#rfc5881-4-5), [`RFC5881-4-8`](#rfc5881-4-8), [`RFC5881-5-1`](#rfc5881-5-1), [`RFC5881-5-2`](#rfc5881-5-2), [`RFC5881-5-3`](#rfc5881-5-3), [`RFC5881-6-1`](#rfc5881-6-1), [`RFC5881-6-2`](#rfc5881-6-2), [`RFC5881-6-5`](#rfc5881-6-5), [`RFC5881-6-6`](#rfc5881-6-6)

**Annotated (including scoped evidence) (9):** [`RFC5881-2-1`](#rfc5881-2-1), [`RFC5881-2-3`](#rfc5881-2-3), [`RFC5881-2-4`](#rfc5881-2-4), [`RFC5881-4-2`](#rfc5881-4-2), [`RFC5881-4-3`](#rfc5881-4-3), [`RFC5881-4-6`](#rfc5881-4-6), [`RFC5881-4-7`](#rfc5881-4-7), [`RFC5881-6-3`](#rfc5881-6-3), [`RFC5881-6-4`](#rfc5881-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5881-2-1` | Each BFD session between a pair of systems MUST traverse a separate network-layer path in both directions. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze cannot select or guarantee the network-layer path a session's packets take; the datagram is handed to the kernel FIB by internal/component/bfd/transport/udp.go:226 (conn.WriteToUDP), and separating two sessions onto distinct L3 paths is an operator topology property the BFD plugin does not control |
| `RFC5881-2-2` | If BFD is to be used in conjunction with both IPv4 and IPv6 on a particular path, a separate BFD session MUST be established for each protocol (and thus encapsulated by that protocol) over that link. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5881EachProtocolSessionEncapsulatedInItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_per_protocol_wire_linux_test.go#L66). **positive:** `unit/verify` [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L277). **negative:** `unit/verify` [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L319) |
| `RFC5881-2-3` | Implementations that support the Echo function MUST ensure that ingress filtering is not used on an interface that employs the Echo function or make an exception for ingress filtering Echo packets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ingress filtering (BCP 38) is host and network policy; the echo transport opens a plain UDP socket at internal/component/bfd/bfd.go:393 (newEchoTransport) and the BFD plugin neither configures nor exempts kernel ingress filters |
| `RFC5881-2-4` | A system implementing the Echo function MUST be capable of sending packets to its own address, which will typically require bypassing the normal forwarding lookup. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's echo does not address packets to its own address; sendEchoLocked (internal/component/bfd/engine/echo.go:96) sets the datagram destination to the peer (To: PeerAddr) and relies on the peer's application-level ZEEC reflection (internal/component/bfd/engine/echo.go:203), so the RFC 5881 self-addressed echo is unimplemented |
| `RFC5881-3-1` | both sides of a session MUST take the "Active" role (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881ClientMultiHopPassiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L214). **positive:** `unit/verify` [`TestRFC5881SingleHopActiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L117). **positive:** `unit/verify` [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37). **negative:** `unit/verify` [`TestRFC5881ClientSingleHopPassiveProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L199). **negative:** `unit/verify` [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52). **negative:** `unit/verify` [`TestRFC5881SingleHopPassiveProfileRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L105) |
| `RFC5881-3-2` | any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_first_packet_binding_test.go#L17). **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128). **negative:** `unit/verify` [`TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_first_packet_binding_test.go#L21). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L152) |
| `RFC5881-4-1` | BFD Control packets MUST be transmitted in UDP packets with destination port 3784, within an IPv4 or IPv6 packet. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881ControlSentToPort3784OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L129). **negative:** `unit/verify` [`TestRFC5881EchoTransportNeverAddressesControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L146) |
| `RFC5881-4-2` | The source port MUST be in the range 49152 through 65535. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the control transport binds one UDP socket to port 3784 (internal/component/bfd/bfd.go:356,360) and reuses it for TX (internal/component/bfd/transport/udp.go:225, conn.WriteToUDP), so the source port of transmitted Control packets is 3784, not a value in the ephemeral 49152-65535 range |
| `RFC5881-4-3` | The same UDP source port number MUST be used for all BFD Control packets associated with a particular session. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881ControlSourcePortFixedOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L194). **negative:** no negative test. **{single-polarity}:** every Control packet in a session leaves from the one UDP socket the (vrf,mode) loop binds (internal/component/bfd/bfd.go newUDPTransport and newUDPTransport6 build that socket's transport, internal/component/bfd/transport/udp.go UDP.Send writes every packet through its u.conn), so the source port is a fixed socket property; there is no per-packet source-port selection that could vary it, hence no negative state to exercise |
| `RFC5881-4-4` | but ultimately the mechanisms in [BFD] MUST be used to demultiplex incoming packets to the proper session. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L176). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L199) |
| `RFC5881-4-5` | BFD Echo packets MUST be transmitted in UDP packets with destination UDP port 3785 in an IPv4 or IPv6 packet. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881EchoSentToPort3785OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L163). **negative:** `unit/verify` [`TestRFC5881ControlTransportNeverAddressesEchoPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L177) |
| `RFC5881-4-6` | The destination address MUST be chosen in such a way as to cause the remote system to forward the packet back to the local system. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the echo destination is the peer address (internal/component/bfd/engine/echo.go:96, To: PeerAddr), reflected by the peer's ze application (internal/component/bfd/engine/echo.go:203), not an address chosen so the peer's forwarding plane loops the packet back, so the RFC 5881 echo dest-addressing rule is unimplemented |
| `RFC5881-4-7` | The source address MUST be chosen in such a way as to preclude the remote system from generating ICMP or Neighbor Discovery Redirect messages. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendEchoLocked (internal/component/bfd/engine/echo.go:83-101) sets no source address on the echo datagram (the kernel selects it) and applies no redirect-avoidance, because ze's echo is peer-addressed and application-reflected rather than looped by the peer's forwarding plane |
| `RFC5881-4-8` | BFD Echo packets MUST be transmitted in such a way as to ensure that they are received by the remote system. On multiaccess media, for example, this requires that the destination datalink address corresponds to the remote system. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_echo_test.go#L69). **positive:** `unit/verify` [`TestRFC5881EchoFrameAddressedToTheRemoteSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L103). **negative:** `unit/verify` [`TestRFC5881EchoNeverSentWhereTheRemoteCannotReceiveIt`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L123) |
| `RFC5881-5-1` | If BFD authentication is not in use on a session, all BFD Control packets for the session MUST be sent with a Time to Live (TTL) or Hop Limit value of 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5881ControlLeavesWithTTL255OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_ttl_wire_linux_test.go#L76). **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_udp_ttl_linux_test.go#L24). **negative:** `unit/verify` [`TestRFC5881UnauthenticatedControlBelow255IsDiscardedOffTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_wire_linux_test.go#L224) |
| `RFC5881-5-2` | All received BFD Control packets that are demultiplexed to the session MUST be discarded if the received TTL or Hop Limit is not equal to 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5881TTL255ReachesTheSessionOnBothDemuxPaths`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_demux_test.go#L24). **positive:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_test.go#L16). **negative:** `unit/verify` [`TestRFC5881TTLNot255DiscardedOnBothDemuxPaths`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_demux_test.go#L46). **negative:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_test.go#L20) |
| `RFC5881-5-3` | If BFD authentication is in use on a session, all BFD Control packets MUST be sent with a TTL or Hop Limit value of 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC5881ControlLeavesWithTTL255OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_ttl_wire_linux_test.go#L79). **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_udp_ttl_linux_test.go#L28). **negative:** `unit/verify` [`TestRFC5881AuthenticatedControlBelow255IsDiscardedOffTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_wire_linux_test.go#L238) |
| `RFC5881-6-1` | Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881ControlLeavesOnTheProtectedLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L181). **negative:** `unit/verify` [`TestRFC5881ControlNeverFollowsARouteOffTheProtectedLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L196). **negative:** `unit/verify` [`TestRFC5881IPv6ControlNeverFollowsARouteOffTheSessionLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_v6_linux_test.go#L99). **negative:** `unit/verify` [`TestRFC5881UnboundLoopControlLeavesOnTheSessionLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L217) |
| `RFC5881-6-2` | On a multiaccess network, BFD Control packets MUST be transmitted with source and destination addresses that are part of the subnet (addressed from and to interfaces on the subnet). (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881ControlAddressedFromAndToTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L148). **positive:** `unit/verify` [`TestRFC5881SubnetCheckAllowsOnSubnetAndExemptPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_exemption_test.go#L25). **positive:** `unit/verify` [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L225). **negative:** `unit/verify` [`TestRFC5881ControlNeverAddressedOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_destination_linux_test.go#L21). **negative:** `unit/verify` [`TestRFC5881ControlNeverAddressedToAnIPv4LinkLocalOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_link_local_v4_linux_test.go#L21). **negative:** `unit/verify` [`TestRFC5881ControlNeverSourcedOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L170). **negative:** `unit/verify` [`TestRFC5881SubnetCheckRefusesIPv4LinkLocalOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_exemption_test.go#L57) |
| `RFC5881-6-3` | On a point-to-point link, the source address of a BFD Control packet MUST NOT be used to identify the session. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not model point-to-point links separately; the first-packet demux keys firstPacketKey on the source address in.From (internal/component/bfd/engine/loop.go:88), so an initial packet whose source differs from the configured peer is not associated with the session, whereas RFC 5881 forbids using the source to identify a point-to-point session |
| `RFC5881-6-4` | This means that the initial BFD packet MUST be accepted with any source address (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the first-packet demux requires the source to equal the configured peer (internal/component/bfd/engine/loop.go:88-94, byKey lookup on in.From), so ze does not accept a point-to-point initial packet bearing an arbitrary source address; once a discriminator is learned, subsequent packets are demuxed by Your Discriminator alone (internal/component/bfd/engine/loop.go:82) as RFC5881-6-5 requires |
| `RFC5881-6-5` | and that subsequent BFD packets MUST be demultiplexed solely by the Your Discriminator field (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorAloneSelectsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_demux_solely_test.go#L34). **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L178). **negative:** `unit/verify` [`TestRFC5881AddressingNeverOverridesTheDiscriminator`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_demux_solely_test.go#L51). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L201) |
| `RFC5881-6-6` | If the received source address changes, the local system MUST NOT use that address as the destination in outgoing BFD Control packets; rather, it MUST continue to use the address configured at session creation. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L218). **negative:** `unit/verify` [`TestRFC5881TransmitDestinationIgnoresChangedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L253) |
| `RFC5881-4-9` | The source port number SHOULD be unique among all BFD sessions on the system. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-10` | In particular, the source address SHOULD NOT be part of the subnet bound to the interface over which the BFD Echo packet is being transmitted (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-11` | and it SHOULD NOT be an IPv6 link-local address, unless it is known by other means that the remote system will not send Redirects. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-7-1` | The BFD authentication mechanism SHOULD be used and is strongly encouraged. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-5-4` | All received BFD Control packets that are demultiplexed to the session MAY be discarded if the received TTL or Hop Limit is not equal to 255. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-5-5` | If the TTL/Hop Limit check is made, it MAY be done before any cryptographic authentication takes place if this will avoid unnecessary calculation that would be detrimental to the receiving system. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-12` | If more than 16384 BFD sessions are simultaneously active, UDP source port numbers MAY be reused on multiple sessions (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-13` | but the number of distinct uses of the same UDP source port number SHOULD be minimized. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-14` | An implementation MAY use the UDP port source number to aid in demultiplexing incoming BFD Control packets (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-6-7` | An implementation MAY notify the application that the neighbor's source address has changed, so that the application might choose to change the destination address or take some other action. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5881-2-1`](#rfc5881-2-1) Each BFD session between a pair of systems MUST traverse a separate network-layer path in both directions. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze cannot select or guarantee the network-layer path a session's packets take; the datagram is handed to the kernel FIB by internal/component/bfd/transport/udp.go:226 (conn.WriteToUDP), and separating two sessions onto distinct L3 paths is an operator topology property the BFD plugin does not control |
| [`RFC5881-2-3`](#rfc5881-2-3) Implementations that support the Echo function MUST ensure that ingress filtering is not used on an interface that employs the Echo function or make an exception for ingress filtering Echo packets. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ingress filtering (BCP 38) is host and network policy; the echo transport opens a plain UDP socket at internal/component/bfd/bfd.go:393 (newEchoTransport) and the BFD plugin neither configures nor exempts kernel ingress filters |
| [`RFC5881-2-4`](#rfc5881-2-4) A system implementing the Echo function MUST be capable of sending packets to its own address, which will typically require bypassing the normal forwarding lookup. (§2) | {gap}, no test | ze's echo does not address packets to its own address; sendEchoLocked (internal/component/bfd/engine/echo.go:96) sets the datagram destination to the peer (To: PeerAddr) and relies on the peer's application-level ZEEC reflection (internal/component/bfd/engine/echo.go:203), so the RFC 5881 self-addressed echo is unimplemented |
| [`RFC5881-4-2`](#rfc5881-4-2) The source port MUST be in the range 49152 through 65535. (§4) | {gap}, no test | the control transport binds one UDP socket to port 3784 (internal/component/bfd/bfd.go:356,360) and reuses it for TX (internal/component/bfd/transport/udp.go:225, conn.WriteToUDP), so the source port of transmitted Control packets is 3784, not a value in the ephemeral 49152-65535 range |
| [`RFC5881-4-6`](#rfc5881-4-6) The destination address MUST be chosen in such a way as to cause the remote system to forward the packet back to the local system. (§4) | {gap}, no test | the echo destination is the peer address (internal/component/bfd/engine/echo.go:96, To: PeerAddr), reflected by the peer's ze application (internal/component/bfd/engine/echo.go:203), not an address chosen so the peer's forwarding plane loops the packet back, so the RFC 5881 echo dest-addressing rule is unimplemented |
| [`RFC5881-4-7`](#rfc5881-4-7) The source address MUST be chosen in such a way as to preclude the remote system from generating ICMP or Neighbor Discovery Redirect messages. (§4) | {gap}, no test | sendEchoLocked (internal/component/bfd/engine/echo.go:83-101) sets no source address on the echo datagram (the kernel selects it) and applies no redirect-avoidance, because ze's echo is peer-addressed and application-reflected rather than looped by the peer's forwarding plane |
| [`RFC5881-6-3`](#rfc5881-6-3) On a point-to-point link, the source address of a BFD Control packet MUST NOT be used to identify the session. (§6) | {gap}, no test | ze does not model point-to-point links separately; the first-packet demux keys firstPacketKey on the source address in.From (internal/component/bfd/engine/loop.go:88), so an initial packet whose source differs from the configured peer is not associated with the session, whereas RFC 5881 forbids using the source to identify a point-to-point session |
| [`RFC5881-6-4`](#rfc5881-6-4) This means that the initial BFD packet MUST be accepted with any source address (§6) | {gap}, no test | the first-packet demux requires the source to equal the configured peer (internal/component/bfd/engine/loop.go:88-94, byKey lookup on in.From), so ze does not accept a point-to-point initial packet bearing an arbitrary source address; once a discriminator is learned, subsequent packets are demuxed by Your Discriminator alone (internal/component/bfd/engine/loop.go:82) as RFC5881-6-5 requires |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5881-2-1`](#rfc5881-2-1)

Each BFD session between a pair of systems MUST traverse a separate network-layer path in both directions. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-1, so no unit is bound to it.

### [`RFC5881-2-2`](#rfc5881-2-2)

If BFD is to be used in conjunction with both IPv4 and IPv6 on a particular path, a separate BFD session MUST be established for each protocol (and thus encapsulated by that protocol) over that link. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §2 "a separate BFD session MUST be established for each protocol (and thus encapsulated by that protocol) over that link". The missing encapsulation clause is now observed on the wire: TestRFC5881EachProtocolSessionEncapsulatedInItsProtocol (rootless netns) runs one Loop over a real transport.Dual with a v4 and a v6 session on lo, distinct discriminators; the peer's IPv4 socket gets the v4 session's Control and never the v6's, the IPv6 socket the v6's and never the v4's. With TestRFC5881PerProtocolSessions (+) and TestRFC5881SamePeerCoalesces (-) for separation. Revert record on dual.go::Send observed red; overlay routing every peer to V4 reds the IPv6 clause (author pin-ov3). This unit also exercises the IPV6_PKTINFO pin on send (Interface lo, unbound Dual).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L319) | unit/verify | revert, verified |
| positive | [`TestRFC5881EachProtocolSessionEncapsulatedInItsProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_per_protocol_wire_linux_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L277) | unit/verify | revert, verified |

### [`RFC5881-2-3`](#rfc5881-2-3)

Implementations that support the Echo function MUST ensure that ingress filtering is not used on an interface that employs the Echo function or make an exception for ingress filtering Echo packets. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-3, so no unit is bound to it.

### [`RFC5881-2-4`](#rfc5881-2-4)

A system implementing the Echo function MUST be capable of sending packets to its own address, which will typically require bypassing the normal forwarding lookup. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-4, so no unit is bound to it.

### [`RFC5881-3-1`](#rfc5881-3-1)

both sides of a session MUST take the "Active" role (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27 (stale after DF-BFD-4). One clause: both sides of a single-hop session MUST take the Active role. Forbidden: a single-hop session created in the Passive role. The only producer of SessionRequest.Passive is profileConfig.applyTo (OSPF, BGP and static requests set none), and every entry passes profileConfig.permitsMode: pluginConfig.validate for a pinned session, resolveProfile from pluginService.EnsureSession for a client request, checkClientProfile at commit. Red: TestRFC5881SingleHopPassiveProfileRejected Fatal when validate returns nil for a passive single-hop profile (its positive twin differs only in the passive bit); TestRFC5881ClientSingleHopPassiveProfileRefused Fatal when EnsureSession returns a handle or lacks the `sets passive` refusal. Positive: TestRFC5881SingleHopActiveProfileAccepted (passive unset validates and yields Passive false), TestRFC5881SingleHopTakesActiveRole (Init takes RoleActive and arms TX at creation), TestRFC5881ClientMultiHopPassiveProfileAccepted (scope: a blanket refusal fails). TestRFC5881NonActiveStaysSilent proves the Passive-role semantics of RFC5880-6.8.7-5, not this row; session.Init itself accepts Passive in either mode, so the guard is the config layer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ClientSingleHopPassiveProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L199) | unit/verify | revert, verified |
| negative | [`TestRFC5881SingleHopPassiveProfileRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L105) | unit/verify | revert, verified |
| negative | [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC5881ClientMultiHopPassiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC5881SingleHopActiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37) | unit/verify | unproven |

### [`RFC5881-3-2`](#rfc5881-3-2)

any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §3 "any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol". TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol varies interface and protocol independently of the remote: same IPv4 peer on loop and loop2 plus an IPv6 session on loop2; a YD=0 packet from the IPv4 peer on loop2 is associated with (peer, loop2, IPv4) only, the loop session and the v6 session keep RemoteDiscr 0. Remote-address arm kept (FirstPacketMatchesByTuple, FirstPacketWrongSourceDropped). The protocol arm follows from the address family in the byKey key, so its negative cannot diverge from the remote-address one; accepted. Revert records on loop.go::handleInbound observed red (+/-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_first_packet_binding_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestRFC5881FirstPacketBoundToRemoteInterfaceAndProtocol`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_first_packet_binding_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128) | unit/verify | revert, verified |

### [`RFC5881-4-1`](#rfc5881-4-1)

BFD Control packets MUST be transmitted in UDP packets with destination port 3784, within an IPv4 or IPv6 packet. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). Units now run in a rootless user+net namespace (userns.Enter), which removes the prior weak reason (fixed host ports colliding with a running ze or a parallel run); the judge ran them: each child ran and passed, none skipped. §4 "BFD Control packets MUST be transmitted in UDP packets with destination port 3784". + TestRFC5881ControlSentToPort3784OnTheWire (IPv4 and IPv6), - TestRFC5881EchoTransportNeverAddressesControlPort (echo at 3785, silence at 3784). Revert records on udp.go::destination (+) and bfd.go::newEchoTransport (-) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881EchoTransportNeverAddressesControlPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestRFC5881ControlSentToPort3784OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L129) | unit/verify | revert, verified |

### [`RFC5881-4-2`](#rfc5881-4-2)

The source port MUST be in the range 49152 through 65535. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-2, so no unit is bound to it.

### [`RFC5881-4-3`](#rfc5881-4-3)

The same UDP source port number MUST be used for all BFD Control packets associated with a particular session. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). Unit TestRFC5881ControlSourcePortFixedOnTheWire unchanged: five datagrams observed with one source AddrPort in a rootless user+net namespace, ran and passed. Its producer UDP.Send changed twice (pinning 637aa9c186 and the reaches fix); the revert record was re-observed red against the current Send (sub2-rec-43). The {single-polarity: positive} marker still holds: every Control packet leaves through the one socket the loop binds, so no per-packet source-port choice exists to exercise.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5881ControlSourcePortFixedOnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L194) | unit/verify | revert, verified |

### [`RFC5881-4-4`](#rfc5881-4-4)

but ultimately the mechanisms in [BFD] MUST be used to demultiplex incoming packets to the proper session. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: demultiplexing a subsequent packet by something other than the RFC 5880 discriminator. (a) a packet with an unallocated Your Discriminator from the configured peer's source being associated: TestRFC5881DiscriminatorDemuxUnknownDropped reds (RemoteDiscriminator != 0), catching a source-address fallback; (b) a packet with the right discriminator from another source not delivered: TestRFC5881DiscriminatorDemuxDelivers reds (RemoteDiscriminator != peerMyDiscr). Producer loop.go handleInbound byDiscr.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L199) | unit/verify | unproven |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L176) | unit/verify | unproven |

### [`RFC5881-4-5`](#rfc5881-4-5)

BFD Echo packets MUST be transmitted in UDP packets with destination UDP port 3785 in an IPv4 or IPv6 packet. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). Units now run in a rootless user+net namespace (userns.Enter), which removes the prior weak reason (fixed host ports colliding with a running ze or a parallel run); the judge ran them: each child ran and passed, none skipped. §4 "BFD Echo packets MUST be transmitted in UDP packets with destination UDP port 3785". + TestRFC5881EchoSentToPort3785OnTheWire, - TestRFC5881ControlTransportNeverAddressesEchoPort. Revert records on destination (+) and bfd.go::newUDPTransport (-) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ControlTransportNeverAddressesEchoPort`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestRFC5881EchoSentToPort3785OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_wire_port_linux_test.go#L163) | unit/verify | revert, verified |

### [`RFC5881-4-6`](#rfc5881-4-6)

The destination address MUST be chosen in such a way as to cause the remote system to forward the packet back to the local system. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-6, so no unit is bound to it.

### [`RFC5881-4-7`](#rfc5881-4-7)

The source address MUST be chosen in such a way as to preclude the remote system from generating ICMP or Neighbor Discovery Redirect messages. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-7, so no unit is bound to it.

### [`RFC5881-4-8`](#rfc5881-4-8)

BFD Echo packets MUST be transmitted in such a way as to ensure that they are received by the remote system. On multiaccess media, for example, this requires that the destination datalink address corresponds to the remote system. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §4 "BFD Echo packets MUST be transmitted in such a way as to ensure that they are received by the remote system. On multiaccess media ... the destination datalink address corresponds to the remote system." Over veth pairs in a rootless netns with AF_PACKET capture: + TestRFC5881EchoFrameAddressedToTheRemoteSystem (echo bound p0, frame on p1 with Ethernet dst == remote MAC); - TestRFC5881EchoNeverSentWhereTheRemoteCannotReceiveIt (unbound, /32 via o0 where the remote is not: nothing on o1, frame on p1 with the remote's MAC). {single-polarity} marker removed. Revert records on udp.go::destination (+) and udp.go::pinsToLink (-) observed red; pinsToLink-false overlay reds only the negative (author pin-ov2).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881EchoNeverSentWhereTheRemoteCannotReceiveIt`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_echo_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestRFC5881EchoFrameAddressedToTheRemoteSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L103) | unit/verify | revert, verified |

### [`RFC5881-5-1`](#rfc5881-5-1)

If BFD authentication is not in use on a session, all BFD Control packets for the session MUST be sent with a Time to Live (TTL) or Hop Limit value of 255. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §5 "If BFD authentication is not in use on a session, all BFD Control packets for the session MUST be sent with a Time to Live (TTL) or Hop Limit value of 255." + TestRFC5881ControlLeavesWithTTL255OnTheWire reads 255 off the received IP header (IPv4 and IPv6), now in a rootless netns. - (R1 a, receiving side) TestRFC5881UnauthenticatedControlBelow255IsDiscardedOffTheWire: a peer socket sends Control at TTL 254 / Hop Limit 254 to a started Loop on a real transport.UDP; session stays Down with RemoteDiscr 0, the same packet at 255 reaches Init. Old plain-kernel-socket negative tag removed. Revert records on loop.go::passesTTLGate (-) and udp_linux.go::applySocketOptionsV6 (+) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881UnauthenticatedControlBelow255IsDiscardedOffTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_wire_linux_test.go#L224) | unit/verify | revert, verified |
| positive | [`TestRFC5881ControlLeavesWithTTL255OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_ttl_wire_linux_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_udp_ttl_linux_test.go#L24) | unit/verify | revert, verified |

### [`RFC5881-5-2`](#rfc5881-5-2)

All received BFD Control packets that are demultiplexed to the session MUST be discarded if the received TTL or Hop Limit is not equal to 255. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. §5 "All received BFD Control packets that are demultiplexed to the session MUST be discarded if the received TTL or Hop Limit is not equal to 255." Through handleInbound (loop.go passesTTLGate after the session lookup, so both demux paths): TestRFC5881TTL255ReachesTheSessionOnBothDemuxPaths - a TTL 255 first packet installs RemoteDiscr and Init, a discriminator-demuxed TTL 255 packet reaches Up; TestRFC5881TTLNot255DiscardedOnBothDemuxPaths - first packets at TTL 254/128/1/0 leave RemoteDiscr 0 and Down, and a discriminator-demuxed TTL 254 packet leaves Init. Buffers isolated: the packets differ only in TTL from the accepted ones. Author's overlay removing the handleInbound gate call reddens the new negative; revert records on passesTTLGate verify. Predicate units in ttl_test.go stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881TTLNot255DiscardedOnBothDemuxPaths`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_demux_test.go#L46) | unit/verify | revert, verified |
| negative | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRFC5881TTL255ReachesTheSessionOnBothDemuxPaths`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_demux_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_test.go#L16) | unit/verify | revert, verified |

### [`RFC5881-5-3`](#rfc5881-5-3)

If BFD authentication is in use on a session, all BFD Control packets MUST be sent with a TTL or Hop Limit value of 255. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §5 "If BFD authentication is in use on a session, all BFD Control packets MUST be sent with a TTL or Hop Limit value of 255." + same wire unit with an auth section, now in netns. - (R1 a) TestRFC5881AuthenticatedControlBelow255IsDiscardedOffTheWire: Keyed SHA1 session, signed packets at 254 dropped (Down, RemoteDiscr 0), at 255 accepted (Init). Revert records on passesTTLGate (-) and applySocketOptions (+) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881AuthenticatedControlBelow255IsDiscardedOffTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_ttl_wire_linux_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC5881ControlLeavesWithTTL255OnTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_ttl_wire_linux_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_udp_ttl_linux_test.go#L28) | unit/verify | revert, verified |

### [`RFC5881-6-1`](#rfc5881-6-1)

Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, subnet). §6 "Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD." UDP.Send pins single-hop packets on a device-less socket with an IP_PKTINFO/IPV6_PKTINFO ifindex cmsg (pinsToLink, egressLinkOf, pktinfoPin). + TestRFC5881ControlLeavesOnTheProtectedLink; - TestRFC5881ControlNeverFollowsARouteOffTheProtectedLink, - TestRFC5881UnboundLoopControlLeavesOnTheSessionLink (IPv4 /32 detour via o0), and now - TestRFC5881IPv6ControlNeverFollowsARouteOffTheSessionLink (unbound [::], /128 via o0: no frame on o1, one on p1), closing the IPv4-only residual. The Send record went stale with the subnet change and was re-recorded by the judge (observed red); the v6 record reverts pinsToLink. Send failures now Warn once a minute per session (warnSendFailedLocked).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ControlNeverFollowsARouteOffTheProtectedLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L196) | unit/verify | revert, verified |
| negative | [`TestRFC5881UnboundLoopControlLeavesOnTheSessionLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L217) | unit/verify | revert, verified |
| negative | [`TestRFC5881IPv6ControlNeverFollowsARouteOffTheSessionLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_v6_linux_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC5881ControlLeavesOnTheProtectedLink`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_one_hop_path_linux_test.go#L181) | unit/verify | revert, verified |

### [`RFC5881-6-2`](#rfc5881-6-2)

On a multiaccess network, BFD Control packets MUST be transmitted with source and destination addresses that are part of the subnet (addressed from and to interfaces on the subnet). (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). Section 6: 'On a multiaccess network, BFD Control packets MUST be transmitted with source and destination addresses that are part of the subnet'. Source half unchanged and proven (+ TestRFC5881ControlAddressedFromAndToTheSubnet, - TestRFC5881ControlNeverSourcedOffTheSubnet, records on destination/pinsToLink). Destination half: the D-8 defect named in the prior verdict is fixed at the producer, egressLink.reaches (transport/udp.go), which now exempts only target.Is6() && IsLinkLocalUnicast, with the RFC 4291 Section 2.5.6 quote verified verbatim in rfc/full/rfc4291.txt; netip also answers IsLinkLocalUnicast for IPv4 169.254/16, which carries no zone and can leave by a gateway. - TestRFC5881ControlNeverAddressedOffTheSubnet (global peer via gateway: errUDPOffSubnet, no frame), NEW - TestRFC5881SubnetCheckRefusesIPv4LinkLocalOffTheSubnet (169.254.5.2, ::ffff:169.254.5.2, 10.58.3.2 refused on a link without 169.254; 169.254.5.2 reaches when the link holds 169.254.5.0/24), NEW netns - TestRFC5881ControlNeverAddressedToAnIPv4LinkLocalOffTheSubnet (Send returns errUDPOffSubnet, no frame via the p0 gateway); both new negatives observed red against the pre-fix reaches (job-bfd-red-08cba9d4). NEW + TestRFC5881SubnetCheckAllowsOnSubnetAndExemptPeers covers both exemptions: on-subnet v4/v6, fe80::2, and an off-subnet peer on a point-to-point link (the RFC's 'On a multiaccess network'). Judge ran all netns units in a user+net namespace: none skipped. Records: revert reaches observed red for each new unit. Residual, not counted against the verdict: engine TestRFC5881TransmitDestinationStable carries an unrecorded supplementary + (unproven); the 30 s ifNameTTL subnet cache bounds how late an address change is seen.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ControlNeverSourcedOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L170) | unit/verify | revert, verified |
| negative | [`TestRFC5881ControlNeverAddressedOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_destination_linux_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestRFC5881SubnetCheckRefusesIPv4LinkLocalOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_exemption_test.go#L57) | unit/verify | revert, verified |
| negative | [`TestRFC5881ControlNeverAddressedToAnIPv4LinkLocalOffTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_link_local_v4_linux_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L225) | unit/verify | revert, verified |
| positive | [`TestRFC5881ControlAddressedFromAndToTheSubnet`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_echo_link_linux_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRFC5881SubnetCheckAllowsOnSubnetAndExemptPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5881_subnet_exemption_test.go#L25) | unit/verify | revert, verified |

### [`RFC5881-6-3`](#rfc5881-6-3)

On a point-to-point link, the source address of a BFD Control packet MUST NOT be used to identify the session. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-6-3, so no unit is bound to it.

### [`RFC5881-6-4`](#rfc5881-6-4)

This means that the initial BFD packet MUST be accepted with any source address (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-6-4, so no unit is bound to it.

### [`RFC5881-6-5`](#rfc5881-6-5)

and that subsequent BFD packets MUST be demultiplexed solely by the Your Discriminator field (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). §6 "subsequent BFD packets MUST be demultiplexed solely by the Your Discriminator field". The missing dimensions are now varied: + TestRFC5881DiscriminatorAloneSelectsTheSession (source, local address and ingress interface all differ from the session: delivered); - TestRFC5881AddressingNeverOverridesTheDiscriminator (session 1's exact tuple with session 2's YD goes to 2, 1 untouched). Old source-dimension tags kept. Revert records on loop.go::handleInbound observed red; overlays byDiscr-also-requiring-iface and byKey-before-byDiscr red the new + and - respectively (author e4-ov9a/b).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881AddressingNeverOverridesTheDiscriminator`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_demux_solely_test.go#L51) | unit/verify | revert, verified |
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC5881DiscriminatorAloneSelectsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_demux_solely_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L178) | unit/verify | revert, verified |

### [`RFC5881-6-6`](#rfc5881-6-6)

If the received source address changes, the local system MUST NOT use that address as the destination in outgoing BFD Control packets; rather, it MUST continue to use the address configured at session creation. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after a changed received source, using it as the transmit destination, or not using the configured address. After handleInbound from 198.51.100.7 by discriminator, TestRFC5881TransmitDestinationStable reds if PeerAddr drifts or ct.last.To != configured peer (MUST continue to use configured), and TestRFC5881TransmitDestinationIgnoresChangedSource reds if ct.last.To == changed source (MUST NOT use that address). Producer loop.go sendLocked To: PeerAddr().

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881TransmitDestinationIgnoresChangedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L253) | unit/verify | unproven |
| positive | [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L218) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5881.txt |
| Source fingerprint | 68a1ce86b6d3d042 |
| Record | rfc/extraction/rfc5881.json |
| Mapped sentences | 22 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 5 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 8 | walked | not stated |
| `5` | not stated | 3 | walked | not stated |
| `6` | not stated | 5 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not a BFD speaker, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `2:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2 Applicability is descriptive deployment guidance addressed to the operator ('it is required that the operator correctly provision the rates'), with a lowercase keyword and no obligation on a BFD implementation; the implementation-side congestion obligation is RFC 5880 Section 7. | In these scenarios it is required that the operator correctly provision the rates at which BFD is transmitted to avoid congestion (e.g link, I/O, CPU) and false failure detection. |

## Superseded

No document obsoletes RFC 5881, so its obligations are stated where they were written.
