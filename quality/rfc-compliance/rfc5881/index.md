# RFC 5881 - Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop)

Partial. Every requirement this repository extracted from RFC 5881, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 52.2% | 12 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 13.0% | 3 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 12.9% | 4 of 31 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| Audit verdicts | 15 | of 23 gated MUSTs judged | 12 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 23 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 33 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 31 |
| Tagged units | 31 |
| Recorded audit verdicts | 15 |
| Discrimination records | 4 |
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
| Positive and negative tests | 12 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC5881-2-2`](#rfc5881-2-2), [`RFC5881-3-1`](#rfc5881-3-1), [`RFC5881-3-2`](#rfc5881-3-2), [`RFC5881-4-1`](#rfc5881-4-1), [`RFC5881-4-4`](#rfc5881-4-4), [`RFC5881-4-5`](#rfc5881-4-5), [`RFC5881-5-1`](#rfc5881-5-1), [`RFC5881-5-2`](#rfc5881-5-2), [`RFC5881-5-3`](#rfc5881-5-3), [`RFC5881-6-1`](#rfc5881-6-1), [`RFC5881-6-5`](#rfc5881-6-5), [`RFC5881-6-6`](#rfc5881-6-6)

**Annotated instead of tested (11):** [`RFC5881-2-1`](#rfc5881-2-1), [`RFC5881-2-3`](#rfc5881-2-3), [`RFC5881-2-4`](#rfc5881-2-4), [`RFC5881-4-2`](#rfc5881-4-2), [`RFC5881-4-3`](#rfc5881-4-3), [`RFC5881-4-6`](#rfc5881-4-6), [`RFC5881-4-7`](#rfc5881-4-7), [`RFC5881-4-8`](#rfc5881-4-8), [`RFC5881-6-2`](#rfc5881-6-2), [`RFC5881-6-3`](#rfc5881-6-3), [`RFC5881-6-4`](#rfc5881-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5881-2-1` | Each BFD session between a pair of systems MUST traverse a separate network-layer path in both directions. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze cannot select or guarantee the network-layer path a session's packets take; the datagram is handed to the kernel FIB by internal/component/bfd/transport/udp.go:226 (conn.WriteToUDP), and separating two sessions onto distinct L3 paths is an operator topology property the BFD plugin does not control |
| `RFC5881-2-2` | If BFD is to be used in conjunction with both IPv4 and IPv6 on a particular path, a separate BFD session MUST be established for each protocol (and thus encapsulated by that protocol) over that link. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L277). **negative:** `unit/verify` [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L319) |
| `RFC5881-2-3` | Implementations that support the Echo function MUST ensure that ingress filtering is not used on an interface that employs the Echo function or make an exception for ingress filtering Echo packets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ingress filtering (BCP 38) is host and network policy; the echo transport opens a plain UDP socket at internal/component/bfd/bfd.go:393 (newEchoTransport) and the BFD plugin neither configures nor exempts kernel ingress filters |
| `RFC5881-2-4` | A system implementing the Echo function MUST be capable of sending packets to its own address, which will typically require bypassing the normal forwarding lookup. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's echo does not address packets to its own address; sendEchoLocked (internal/component/bfd/engine/echo.go:96) sets the datagram destination to the peer (To: PeerAddr) and relies on the peer's application-level ZEEC reflection (internal/component/bfd/engine/echo.go:203), so the RFC 5881 self-addressed echo is unimplemented |
| `RFC5881-3-1` | both sides of a session MUST take the "Active" role (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881ClientMultiHopPassiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L214). **positive:** `unit/verify` [`TestRFC5881SingleHopActiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L126). **positive:** `unit/verify` [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37). **negative:** `unit/verify` [`TestRFC5881ClientSingleHopPassiveProfileRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L199). **negative:** `unit/verify` [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52). **negative:** `unit/verify` [`TestRFC5881SingleHopPassiveProfileRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L114) |
| `RFC5881-3-2` | any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L152) |
| `RFC5881-4-1` | BFD Control packets MUST be transmitted in UDP packets with destination port 3784, within an IPv4 or IPv6 packet. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881ControlDestPort3784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L20). **negative:** `unit/verify` [`TestRFC5881ControlPortNotUsedForEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L36) |
| `RFC5881-4-2` | The source port MUST be in the range 49152 through 65535. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the control transport binds one UDP socket to port 3784 (internal/component/bfd/bfd.go:356,360) and reuses it for TX (internal/component/bfd/transport/udp.go:225, conn.WriteToUDP), so the source port of transmitted Control packets is 3784, not a value in the ephemeral 49152-65535 range |
| `RFC5881-4-3` | The same UDP source port number MUST be used for all BFD Control packets associated with a particular session. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881SingleSourcePortPerSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L73). **negative:** no negative test. **{single-polarity}:** every Control packet in a session leaves from the one UDP socket the (vrf,mode) loop binds (internal/component/bfd/bfd.go:355-367, internal/component/bfd/transport/udp.go:218-228), so the source port is a fixed socket property; there is no per-packet source-port selection that could vary it, hence no negative state to exercise |
| `RFC5881-4-4` | but ultimately the mechanisms in [BFD] MUST be used to demultiplex incoming packets to the proper session. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L176). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L199) |
| `RFC5881-4-5` | BFD Echo packets MUST be transmitted in UDP packets with destination UDP port 3785 in an IPv4 or IPv6 packet. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881EchoDestPort3785`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L48). **negative:** `unit/verify` [`TestRFC5881EchoPortNotUsedForControl`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L62) |
| `RFC5881-4-6` | The destination address MUST be chosen in such a way as to cause the remote system to forward the packet back to the local system. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the echo destination is the peer address (internal/component/bfd/engine/echo.go:96, To: PeerAddr), reflected by the peer's ze application (internal/component/bfd/engine/echo.go:203), not an address chosen so the peer's forwarding plane loops the packet back, so the RFC 5881 echo dest-addressing rule is unimplemented |
| `RFC5881-4-7` | The source address MUST be chosen in such a way as to preclude the remote system from generating ICMP or Neighbor Discovery Redirect messages. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendEchoLocked (internal/component/bfd/engine/echo.go:83-101) sets no source address on the echo datagram (the kernel selects it) and applies no redirect-avoidance, because ze's echo is peer-addressed and application-reflected rather than looped by the peer's forwarding plane |
| `RFC5881-4-8` | BFD Echo packets MUST be transmitted in such a way as to ensure that they are received by the remote system. On multiaccess media, for example, this requires that the destination datalink address corresponds to the remote system. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/echo_test.go#L69). **negative:** no negative test. **{single-polarity}:** ze transmits echo datagrams to the peer's echo port and the peer receives and reflects them (proven by the round-trip in internal/component/bfd/engine/echo_test.go:68); ze relies on the kernel for L2 delivery and has no path that would stop a well-formed echo reaching the remote, so there is no negative polarity to exercise |
| `RFC5881-5-1` | If BFD authentication is not in use on a session, all BFD Control packets for the session MUST be sent with a Time to Live (TTL) or Hop Limit value of 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L24). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L158) |
| `RFC5881-5-2` | All received BFD Control packets that are demultiplexed to the session MUST be discarded if the received TTL or Hop Limit is not equal to 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L16). **negative:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L20) |
| `RFC5881-5-3` | If BFD authentication is in use on a session, all BFD Control packets MUST be sent with a TTL or Hop Limit value of 255. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L28). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L163) |
| `RFC5881-6-1` | Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L31). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L166) |
| `RFC5881-6-2` | On a multiaccess network, BFD Control packets MUST be transmitted with source and destination addresses that are part of the subnet (addressed from and to interfaces on the subnet). (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L225). **negative:** no negative test. **{single-polarity}:** ze transmits single-hop Control packets to the operator-configured peer (internal/component/bfd/engine/loop.go:232, To: PeerAddr) and the kernel selects the interface source; subnet membership is set by operator config and routing, and no ze code rewrites either address off-subnet, so there is no negative polarity |
| `RFC5881-6-3` | On a point-to-point link, the source address of a BFD Control packet MUST NOT be used to identify the session. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not model point-to-point links separately; the first-packet demux keys firstPacketKey on the source address in.From (internal/component/bfd/engine/loop.go:88), so an initial packet whose source differs from the configured peer is not associated with the session, whereas RFC 5881 forbids using the source to identify a point-to-point session |
| `RFC5881-6-4` | This means that the initial BFD packet MUST be accepted with any source address (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the first-packet demux requires the source to equal the configured peer (internal/component/bfd/engine/loop.go:88-94, byKey lookup on in.From), so ze does not accept a point-to-point initial packet bearing an arbitrary source address; once a discriminator is learned, subsequent packets are demuxed by Your Discriminator alone (internal/component/bfd/engine/loop.go:82) as RFC5881-6-5 requires |
| `RFC5881-6-5` | and that subsequent BFD packets MUST be demultiplexed solely by the Your Discriminator field (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L178). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L201) |
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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: one session carrying both IPv4 and IPv6, or a protocol's packets not encapsulated in that protocol. TestRFC5881PerProtocolSessions reds if a v4 and a v6 peer collapse (n != 2, key equal, discriminator equal), and TestRFC5881SamePeerCoalesces pins keying. No tagged assertion covers '(and thus encapsulated by that protocol)': nothing checks that the v6 session transmits in IPv6 (sendLocked To/transport family).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L319) | unit/verify | unproven |
| positive | [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L277) | unit/verify | unproven |

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
| negative | [`TestRFC5881SingleHopPassiveProfileRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC5881ClientMultiHopPassiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/enabled_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC5881SingleHopActiveProfileAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37) | unit/verify | unproven |

### [`RFC5881-3-2`](#rfc5881-3-2)

any BFD packet from the remote machine with a zero value of Your Discriminator MUST be associated with the session bound to the remote system, interface, and protocol. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a Your-Discriminator-zero packet not associated with the session bound to (remote, interface, protocol), or associated with another. TestRFC5881FirstPacketMatchesByTuple reds on non-association; TestRFC5881FirstPacketWrongSourceDropped reds if the remote-address dimension is ignored. No tagged unit varies the interface or the protocol (the interface arm lives in untagged TestFirstPacketMatchesWhatTheTransportSurfaces), so matching that ignored interface or protocol stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L152) | unit/verify | unproven |
| positive | [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128) | unit/verify | unproven |

### [`RFC5881-4-1`](#rfc5881-4-1)

BFD Control packets MUST be transmitted in UDP packets with destination port 3784, within an IPv4 or IPv6 packet. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: Control packets sent to a destination port other than 3784. TestRFC5881ControlDestPort3784 asserts the BIND port and the constant; UDP.Send (transport/udp.go) takes the destination from Bind.Port, but no tagged assertion observes the destination port of a transmitted packet, so a Send that addressed another port stays green. IPv4/IPv6 encapsulation is not asserted either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ControlPortNotUsedForEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L36) | unit/verify | unproven |
| positive | [`TestRFC5881ControlDestPort3784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L20) | unit/verify | unproven |

### [`RFC5881-4-2`](#rfc5881-4-2)

The source port MUST be in the range 49152 through 65535. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-2, so no unit is bound to it.

### [`RFC5881-4-3`](#rfc5881-4-3)

The same UDP source port number MUST be used for all BFD Control packets associated with a particular session. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: Control packets of one session leaving from different source ports. TestRFC5881SingleSourcePortPerSession asserts only the transport's configured Bind.Port across two constructions; it never sends and observes source ports, so a Send that opened a fresh socket per packet stays green. {single-polarity} marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5881SingleSourcePortPerSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L73) | unit/verify | unproven |

### [`RFC5881-4-4`](#rfc5881-4-4)

but ultimately the mechanisms in [BFD] MUST be used to demultiplex incoming packets to the proper session. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: demultiplexing a subsequent packet by something other than the RFC 5880 discriminator. (a) a packet with an unallocated Your Discriminator from the configured peer's source being associated: TestRFC5881DiscriminatorDemuxUnknownDropped reds (RemoteDiscriminator != 0), catching a source-address fallback; (b) a packet with the right discriminator from another source not delivered: TestRFC5881DiscriminatorDemuxDelivers reds (RemoteDiscriminator != peerMyDiscr). Producer loop.go handleInbound byDiscr.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L199) | unit/verify | unproven |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L176) | unit/verify | unproven |

### [`RFC5881-4-5`](#rfc5881-4-5)

BFD Echo packets MUST be transmitted in UDP packets with destination UDP port 3785 in an IPv4 or IPv6 packet. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: Echo packets sent to a destination port other than 3785. TestRFC5881EchoDestPort3785 asserts the Echo transport's BIND port and the constant, not the destination port of a transmitted echo; a Send that addressed another port stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881EchoPortNotUsedForControl`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L62) | unit/verify | unproven |
| positive | [`TestRFC5881EchoDestPort3785`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L48) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: echo transmitted so it does not reach the remote. TestEchoRoundTrip reds if no RTT sample is recorded on A, which needs B to receive and reflect. It runs over the in-memory transport.Pair, so the multiaccess clause (destination datalink address corresponds to the remote) has no assertion. {single-polarity} marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/echo_test.go#L69) | unit/verify | unproven |

### [`RFC5881-5-1`](#rfc5881-5-1)

If BFD authentication is not in use on a session, all BFD Control packets for the session MUST be sent with a Time to Live (TTL) or Hop Limit value of 255. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a Control packet sent with TTL or Hop Limit other than 255 (auth not in use). TestUDPSetOutboundTTL255 reads IP_TTL back from an IPv4 socket after Start and reds if applySocketOptions drops IP_TTL=255. The IPv6 Hop Limit (applySocketOptionsV6, IPV6_UNICAST_HOPS=255) has no tagged assertion. TestUDPDefaultTTLNot255 checks a plain socket, not ze code.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L158) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L24) | unit/verify | unproven |

### [`RFC5881-5-2`](#rfc5881-5-2)

All received BFD Control packets that are demultiplexed to the session MUST be discarded if the received TTL or Hop Limit is not equal to 255. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a demultiplexed single-hop Control packet with TTL != 255 reaching the session. TestTTLGateSingleHop asserts the predicate passesTTLGate (254/253/128/0 false, 255 true) but never drives handleInbound, so removing the call at loop.go handleInbound (if !passesTTLGate ... return) leaves it green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L20) | unit/verify | unproven |
| positive | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L16) | unit/verify | unproven |

### [`RFC5881-5-3`](#rfc5881-5-3)

If BFD authentication is in use on a session, all BFD Control packets MUST be sent with a TTL or Hop Limit value of 255. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: with auth in use, a Control packet sent with TTL or Hop Limit other than 255. The IP_TTL=255 setsockopt is unconditional and TestUDPSetOutboundTTL255 reds on its absence for IPv4; the IPv6 Hop Limit clause (IPV6_UNICAST_HOPS) has no tagged assertion, and no tagged unit runs with authentication in use.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L163) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L28) | unit/verify | unproven |

### [`RFC5881-6-1`](#rfc5881-6-1)

Implementations MUST ensure that all BFD Control packets are transmitted over the one-hop path being protected by BFD. (§6)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Forbidden: a Control packet leaving over a path other than the protected one-hop path (wrong interface, routed off-link). The tagged units assert IP_TTL=255 on the socket, which is RFC5881-5-1; TTL 255 does not confine a packet to one hop (it permits the most hops). The tag prose 'without TTL 255 the packet is not confined to the one-hop path' asserts a neighbouring rule. Interface binding (SO_BINDTODEVICE in applySocketOptions) is not asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L166) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L31) | unit/verify | unproven |

### [`RFC5881-6-2`](#rfc5881-6-2)

On a multiaccess network, BFD Control packets MUST be transmitted with source and destination addresses that are part of the subnet (addressed from and to interfaces on the subnet). (§6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: on multiaccess, a Control packet whose source or destination is off the subnet. TestRFC5881TransmitDestinationStable asserts ct.last.To == configured peer; nothing asserts the peer or the source is on the interface subnet, and the source address is not observed. {single-polarity} marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L225) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: subsequent packets demultiplexed on anything besides Your Discriminator. TestRFC5881DiscriminatorDemuxDelivers reds if the source address participates (other source still delivered); TestRFC5881DiscriminatorDemuxUnknownDropped reds on a source fallback. Neither varies the ingress interface or local address, so a byDiscr lookup that additionally required the interface to match stays green; 'solely' is proven for the source dimension only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L201) | unit/verify | unproven |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L178) | unit/verify | unproven |

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
