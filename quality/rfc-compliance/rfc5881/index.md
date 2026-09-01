# RFC 5881 - Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop)

Partial. Every requirement this repository extracted from RFC 5881, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 57.1% | 12 of 21 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.3% | 3 of 21 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 21 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 33 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 28.6% | 6 of 21 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 21 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 33 |
| Gated MUST-level | 23 |
| Obligations that bind Ze | 21 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 6 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC5881-2-2`](#rfc5881-2-2), [`RFC5881-3-1`](#rfc5881-3-1), [`RFC5881-3-2`](#rfc5881-3-2), [`RFC5881-4-1`](#rfc5881-4-1), [`RFC5881-4-4`](#rfc5881-4-4), [`RFC5881-4-5`](#rfc5881-4-5), [`RFC5881-5-1`](#rfc5881-5-1), [`RFC5881-5-2`](#rfc5881-5-2), [`RFC5881-5-3`](#rfc5881-5-3), [`RFC5881-6-1`](#rfc5881-6-1), [`RFC5881-6-5`](#rfc5881-6-5), [`RFC5881-6-6`](#rfc5881-6-6)

**Annotated instead of tested (11):** [`RFC5881-2-1`](#rfc5881-2-1), [`RFC5881-2-3`](#rfc5881-2-3), [`RFC5881-2-4`](#rfc5881-2-4), [`RFC5881-4-2`](#rfc5881-4-2), [`RFC5881-4-3`](#rfc5881-4-3), [`RFC5881-4-6`](#rfc5881-4-6), [`RFC5881-4-7`](#rfc5881-4-7), [`RFC5881-4-8`](#rfc5881-4-8), [`RFC5881-6-2`](#rfc5881-6-2), [`RFC5881-6-3`](#rfc5881-6-3), [`RFC5881-6-4`](#rfc5881-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5881-2-1` | Each BFD session between a pair of systems must traverse a separate network-layer path in both directions (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze cannot select or guarantee the network-layer path a session's packets take; the datagram is handed to the kernel FIB by internal/component/bfd/transport/udp.go:226 (conn.WriteToUDP), and separating two sessions onto distinct L3 paths is an operator topology property the BFD plugin does not control |
| `RFC5881-2-2` | A separate BFD session must be established for each protocol (IPv4 and IPv6) over a link (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L247). **negative:** `unit/verify` [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L289) |
| `RFC5881-2-3` | Implementations supporting Echo function must ensure ingress filtering is not used on the Echo interface, or make an exception for Echo packets (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ingress filtering (BCP 38) is host and network policy; the echo transport opens a plain UDP socket at internal/component/bfd/bfd.go:393 (newEchoTransport) and the BFD plugin neither configures nor exempts kernel ingress filters |
| `RFC5881-2-4` | A system implementing Echo must be capable of sending packets to its own address (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's echo does not address packets to its own address; sendEchoLocked (internal/component/bfd/engine/echo.go:96) sets the datagram destination to the peer (To: PeerAddr) and relies on the peer's application-level ZEEC reflection (internal/component/bfd/engine/echo.go:203), so the RFC 5881 self-addressed echo is unimplemented |
| `RFC5881-3-1` | Both sides of a session must take the Active role (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37). **negative:** `unit/verify` [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52) |
| `RFC5881-3-2` | A received packet with Your Discriminator = 0 must be associated with the session bound to the remote system, interface, and protocol (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L98). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L122) |
| `RFC5881-4-1` | BFD Control packets must be transmitted in UDP with destination port 3784 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881ControlDestPort3784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L18). **negative:** `unit/verify` [`TestRFC5881ControlPortNotUsedForEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L34) |
| `RFC5881-4-2` | UDP source port must be in the range 49152-65535 (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the control transport binds one UDP socket to port 3784 (internal/component/bfd/bfd.go:356,360) and reuses it for TX (internal/component/bfd/transport/udp.go:225, conn.WriteToUDP), so the source port of transmitted Control packets is 3784, not a value in the ephemeral 49152-65535 range |
| `RFC5881-4-3` | The same UDP source port must be used for all Control packets in a session (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881SingleSourcePortPerSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L71). **negative:** no negative test. **{single-polarity}:** every Control packet in a session leaves from the one UDP socket the (vrf,mode) loop binds (internal/component/bfd/bfd.go:355-367, internal/component/bfd/transport/udp.go:218-228), so the source port is a fixed socket property; there is no per-packet source-port selection that could vary it, hence no negative state to exercise |
| `RFC5881-4-4` | Ultimately, RFC 5880 mechanisms must be used to demultiplex incoming packets to the proper session (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L146). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L169) |
| `RFC5881-4-5` | BFD Echo packets must be transmitted in UDP with destination port 3785 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5881EchoDestPort3785`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L46). **negative:** `unit/verify` [`TestRFC5881EchoPortNotUsedForControl`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L60) |
| `RFC5881-4-6` | Echo destination address must cause the remote system to forward the packet back (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the echo destination is the peer address (internal/component/bfd/engine/echo.go:96, To: PeerAddr), reflected by the peer's ze application (internal/component/bfd/engine/echo.go:203), not an address chosen so the peer's forwarding plane loops the packet back, so the RFC 5881 echo dest-addressing rule is unimplemented |
| `RFC5881-4-7` | Echo source address must preclude the remote system from generating ICMP or ND Redirect messages (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sendEchoLocked (internal/component/bfd/engine/echo.go:83-101) sets no source address on the echo datagram (the kernel selects it) and applies no redirect-avoidance, because ze's echo is peer-addressed and application-reflected rather than looped by the peer's forwarding plane |
| `RFC5881-4-8` | Echo packets must be transmitted so they are received by the remote system (e.g., correct L2 destination on multiaccess media) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/echo_test.go#L69). **negative:** no negative test. **{single-polarity}:** ze transmits echo datagrams to the peer's echo port and the peer receives and reflects them (proven by the round-trip in internal/component/bfd/engine/echo_test.go:68); ze relies on the kernel for L2 delivery and has no path that would stop a well-formed echo reaching the remote, so there is no negative polarity to exercise |
| `RFC5881-5-1` | If authentication is not in use: TTL/Hop Limit must be 255 on transmit (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L24). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L158) |
| `RFC5881-5-2` | If authentication is not in use: received packets must be discarded if TTL/Hop Limit != 255 (§5) | MUST | 5 | **positive:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L16). **negative:** `unit/verify` [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L20) |
| `RFC5881-5-3` | If authentication is in use: TTL/Hop Limit must be 255 on transmit (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L28). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L163) |
| `RFC5881-6-1` | All BFD Control packets must be transmitted over the one-hop path being protected (§6) | MUST | 6 | **positive:** `unit/verify` [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L31). **negative:** `unit/verify` [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L166) |
| `RFC5881-6-2` | On multiaccess networks, Control packets must be transmitted with source and destination addresses on the subnet (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L195). **negative:** no negative test. **{single-polarity}:** ze transmits single-hop Control packets to the operator-configured peer (internal/component/bfd/engine/loop.go:232, To: PeerAddr) and the kernel selects the interface source; subnet membership is set by operator config and routing, and no ze code rewrites either address off-subnet, so there is no negative polarity |
| `RFC5881-6-3` | On point-to-point links, source address must not be used to identify the session (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not model point-to-point links separately; the first-packet demux keys firstPacketKey on the source address in.From (internal/component/bfd/engine/loop.go:88), so an initial packet whose source differs from the configured peer is not associated with the session, whereas RFC 5881 forbids using the source to identify a point-to-point session |
| `RFC5881-6-4` | On point-to-point links, initial BFD packet must be accepted with any source address (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the first-packet demux requires the source to equal the configured peer (internal/component/bfd/engine/loop.go:88-94, byKey lookup on in.From), so ze does not accept a point-to-point initial packet bearing an arbitrary source address; once a discriminator is learned, subsequent packets are demuxed by Your Discriminator alone (internal/component/bfd/engine/loop.go:82) as RFC5881-6-5 requires |
| `RFC5881-6-5` | On point-to-point links, subsequent packets must be demultiplexed solely by Your Discriminator (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L148). **negative:** `unit/verify` [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L171) |
| `RFC5881-6-6` | If received source address changes on point-to-point, local system must not use that address as the destination; must continue using the address configured at session creation (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L188). **negative:** `unit/verify` [`TestRFC5881TransmitDestinationIgnoresChangedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L223) |
| `RFC5881-4-9` | UDP source port number should be unique among all BFD sessions on the system (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-10` | Echo source address should not be part of the subnet bound to the egress interface (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-11` | Echo source address should not be an IPv6 link-local address (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-7-1` | BFD authentication mechanism should be used for tunnels (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-5-4` | If authentication is in use: received packets may be discarded if TTL/Hop Limit != 255 (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-5-5` | TTL/Hop Limit check may be done before cryptographic authentication to save CPU (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-12` | If more than 16384 sessions, UDP source ports may be reused on multiple sessions (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-13` | If source ports are reused, the number of distinct uses of the same port should be minimized (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-4-14` | An implementation may use UDP source port to aid in demultiplexing (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5881-6-7` | An implementation may notify the application that the neighbor's source address has changed (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5881-2-1`](#rfc5881-2-1) Each BFD session between a pair of systems must traverse a separate network-layer path in both directions (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze cannot select or guarantee the network-layer path a session's packets take; the datagram is handed to the kernel FIB by internal/component/bfd/transport/udp.go:226 (conn.WriteToUDP), and separating two sessions onto distinct L3 paths is an operator topology property the BFD plugin does not control |
| [`RFC5881-2-3`](#rfc5881-2-3) Implementations supporting Echo function must ensure ingress filtering is not used on the Echo interface, or make an exception for Echo packets (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ingress filtering (BCP 38) is host and network policy; the echo transport opens a plain UDP socket at internal/component/bfd/bfd.go:393 (newEchoTransport) and the BFD plugin neither configures nor exempts kernel ingress filters |
| [`RFC5881-2-4`](#rfc5881-2-4) A system implementing Echo must be capable of sending packets to its own address (§2) | {gap}, no test | ze's echo does not address packets to its own address; sendEchoLocked (internal/component/bfd/engine/echo.go:96) sets the datagram destination to the peer (To: PeerAddr) and relies on the peer's application-level ZEEC reflection (internal/component/bfd/engine/echo.go:203), so the RFC 5881 self-addressed echo is unimplemented |
| [`RFC5881-4-2`](#rfc5881-4-2) UDP source port must be in the range 49152-65535 (§4) | {gap}, no test | the control transport binds one UDP socket to port 3784 (internal/component/bfd/bfd.go:356,360) and reuses it for TX (internal/component/bfd/transport/udp.go:225, conn.WriteToUDP), so the source port of transmitted Control packets is 3784, not a value in the ephemeral 49152-65535 range |
| [`RFC5881-4-6`](#rfc5881-4-6) Echo destination address must cause the remote system to forward the packet back (§4) | {gap}, no test | the echo destination is the peer address (internal/component/bfd/engine/echo.go:96, To: PeerAddr), reflected by the peer's ze application (internal/component/bfd/engine/echo.go:203), not an address chosen so the peer's forwarding plane loops the packet back, so the RFC 5881 echo dest-addressing rule is unimplemented |
| [`RFC5881-4-7`](#rfc5881-4-7) Echo source address must preclude the remote system from generating ICMP or ND Redirect messages (§4) | {gap}, no test | sendEchoLocked (internal/component/bfd/engine/echo.go:83-101) sets no source address on the echo datagram (the kernel selects it) and applies no redirect-avoidance, because ze's echo is peer-addressed and application-reflected rather than looped by the peer's forwarding plane |
| [`RFC5881-6-3`](#rfc5881-6-3) On point-to-point links, source address must not be used to identify the session (§6) | {gap}, no test | ze does not model point-to-point links separately; the first-packet demux keys firstPacketKey on the source address in.From (internal/component/bfd/engine/loop.go:88), so an initial packet whose source differs from the configured peer is not associated with the session, whereas RFC 5881 forbids using the source to identify a point-to-point session |
| [`RFC5881-6-4`](#rfc5881-6-4) On point-to-point links, initial BFD packet must be accepted with any source address (§6) | {gap}, no test | the first-packet demux requires the source to equal the configured peer (internal/component/bfd/engine/loop.go:88-94, byKey lookup on in.From), so ze does not accept a point-to-point initial packet bearing an arbitrary source address; once a discriminator is learned, subsequent packets are demuxed by Your Discriminator alone (internal/component/bfd/engine/loop.go:82) as RFC5881-6-5 requires |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5881-2-1`](#rfc5881-2-1)

Each BFD session between a pair of systems must traverse a separate network-layer path in both directions (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-1, so no unit is bound to it.

### [`RFC5881-2-2`](#rfc5881-2-2)

A separate BFD session must be established for each protocol (IPv4 and IPv6) over a link (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881SamePeerCoalesces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L289) | unit/verify | unproven |
| positive | [`TestRFC5881PerProtocolSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L247) | unit/verify | unproven |

### [`RFC5881-2-3`](#rfc5881-2-3)

Implementations supporting Echo function must ensure ingress filtering is not used on the Echo interface, or make an exception for Echo packets (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-3, so no unit is bound to it.

### [`RFC5881-2-4`](#rfc5881-2-4)

A system implementing Echo must be capable of sending packets to its own address (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-2-4, so no unit is bound to it.

### [`RFC5881-3-1`](#rfc5881-3-1)

Both sides of a session must take the Active role (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881NonActiveStaysSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC5881SingleHopTakesActiveRole`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5881_test.go#L37) | unit/verify | unproven |

### [`RFC5881-3-2`](#rfc5881-3-2)

A received packet with Your Discriminator = 0 must be associated with the session bound to the remote system, interface, and protocol (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L122) | unit/verify | unproven |
| positive | [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L98) | unit/verify | unproven |

### [`RFC5881-4-1`](#rfc5881-4-1)

BFD Control packets must be transmitted in UDP with destination port 3784 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881ControlPortNotUsedForEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L34) | unit/verify | unproven |
| positive | [`TestRFC5881ControlDestPort3784`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L18) | unit/verify | unproven |

### [`RFC5881-4-2`](#rfc5881-4-2)

UDP source port must be in the range 49152-65535 (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-2, so no unit is bound to it.

### [`RFC5881-4-3`](#rfc5881-4-3)

The same UDP source port must be used for all Control packets in a session (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5881SingleSourcePortPerSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L71) | unit/verify | unproven |

### [`RFC5881-4-4`](#rfc5881-4-4)

Ultimately, RFC 5880 mechanisms must be used to demultiplex incoming packets to the proper session (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L169) | unit/verify | unproven |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L146) | unit/verify | unproven |

### [`RFC5881-4-5`](#rfc5881-4-5)

BFD Echo packets must be transmitted in UDP with destination port 3785 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881EchoPortNotUsedForControl`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L60) | unit/verify | unproven |
| positive | [`TestRFC5881EchoDestPort3785`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L46) | unit/verify | unproven |

### [`RFC5881-4-6`](#rfc5881-4-6)

Echo destination address must cause the remote system to forward the packet back (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-6, so no unit is bound to it.

### [`RFC5881-4-7`](#rfc5881-4-7)

Echo source address must preclude the remote system from generating ICMP or ND Redirect messages (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-4-7, so no unit is bound to it.

### [`RFC5881-4-8`](#rfc5881-4-8)

Echo packets must be transmitted so they are received by the remote system (e.g., correct L2 destination on multiaccess media) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEchoRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/echo_test.go#L69) | unit/verify | unproven |

### [`RFC5881-5-1`](#rfc5881-5-1)

If authentication is not in use: TTL/Hop Limit must be 255 on transmit (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L158) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L24) | unit/verify | unproven |

### [`RFC5881-5-2`](#rfc5881-5-2)

If authentication is not in use: received packets must be discarded if TTL/Hop Limit != 255 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L20) | unit/verify | unproven |
| positive | [`TestTTLGateSingleHop`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/ttl_test.go#L16) | unit/verify | unproven |

### [`RFC5881-5-3`](#rfc5881-5-3)

If authentication is in use: TTL/Hop Limit must be 255 on transmit (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L163) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L28) | unit/verify | unproven |

### [`RFC5881-6-1`](#rfc5881-6-1)

All BFD Control packets must be transmitted over the one-hop path being protected (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUDPDefaultTTLNot255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L166) | unit/verify | unproven |
| positive | [`TestUDPSetOutboundTTL255`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/udp_ttl_linux_test.go#L31) | unit/verify | unproven |

### [`RFC5881-6-2`](#rfc5881-6-2)

On multiaccess networks, Control packets must be transmitted with source and destination addresses on the subnet (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L195) | unit/verify | unproven |

### [`RFC5881-6-3`](#rfc5881-6-3)

On point-to-point links, source address must not be used to identify the session (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-6-3, so no unit is bound to it.

### [`RFC5881-6-4`](#rfc5881-6-4)

On point-to-point links, initial BFD packet must be accepted with any source address (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5881-6-4, so no unit is bound to it.

### [`RFC5881-6-5`](#rfc5881-6-5)

On point-to-point links, subsequent packets must be demultiplexed solely by Your Discriminator (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881DiscriminatorDemuxUnknownDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L171) | unit/verify | unproven |
| positive | [`TestRFC5881DiscriminatorDemuxDelivers`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L148) | unit/verify | unproven |

### [`RFC5881-6-6`](#rfc5881-6-6)

If received source address changes on point-to-point, local system must not use that address as the destination; must continue using the address configured at session creation (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881TransmitDestinationIgnoresChangedSource`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L223) | unit/verify | unproven |
| positive | [`TestRFC5881TransmitDestinationStable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L188) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5881, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5881, so its obligations are stated where they were written.
