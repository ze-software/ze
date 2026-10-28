# RFC 9329 - TCP Encapsulation of Internet Key Exchange Protocol (IKE) and IPsec Packets

Future. Every requirement this repository extracted from RFC 9329, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 30 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 30 | of 74 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 30 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 30 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 74 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 30 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9329.md` |
| Requirement shard | `rfc/requirements/rfc9329.md` |
| RFC text | `rfc/full/rfc9329.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-concentrator profile. The extraction is COMPLETE: the source text is at rfc/full/rfc9329.txt and the extraction sign-off is rfc/extraction/rfc9329.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: all 30 MUST-level requirements are classified, and all 30 are `{gap}`, because Ze carries IKE and ESP over UDP ports 500 and 4500 and directly over IP only. `internal/component/ike/transport/udp.go::newTransport` opens a UDP socket, and nothing under `internal/component/ike/` listens on or dials TCP, frames a Length-prefixed stream or sends the IKETCP stream prefix.

## What the public ledger says

**Status:** Future

**What the ledger says is covered:**

None: IKE runs over UDP 500 and 4500 ([`internal/component/ike/transport/udp.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/transport/udp.go)), ESP directly over IP or in UDP 4500 per RFC 3948.

**What the ledger says remains**

No TCP listener or originator, no IKETCP stream prefix, no 16-bit Length framing for IKE or ESP, no ESP-in-TCP datapath, no UDP-to-TCP fallback, and no MOBIKE switching between UDP and TCP. Some prohibitions (preferring UDP or direct ESP over TCP, dropping NAT-keepalives on TCP) hold today only because TCP is never used, and no test proves them. All 30 MUST-level requirements are annotated `{gap}` in [`rfc/short/rfc9329.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9329.md). Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 30 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (30):** [`RFC9329-1.2-1`](#rfc9329-1.2-1), [`RFC9329-2-1`](#rfc9329-2-1), [`RFC9329-3-1`](#rfc9329-3-1), [`RFC9329-3.1-1`](#rfc9329-3.1-1), [`RFC9329-3.1-2`](#rfc9329-3.1-2), [`RFC9329-3.2-1`](#rfc9329-3.2-1), [`RFC9329-3.2-2`](#rfc9329-3.2-2), [`RFC9329-3.2-3`](#rfc9329-3.2-3), [`RFC9329-4-1`](#rfc9329-4-1), [`RFC9329-5-1`](#rfc9329-5-1), [`RFC9329-5-2`](#rfc9329-5-2), [`RFC9329-5.1-1`](#rfc9329-5.1-1), [`RFC9329-6.1-1`](#rfc9329-6.1-1), [`RFC9329-6.1-2`](#rfc9329-6.1-2), [`RFC9329-6.1-3`](#rfc9329-6.1-3), [`RFC9329-6.1-4`](#rfc9329-6.1-4), [`RFC9329-6.1-5`](#rfc9329-6.1-5), [`RFC9329-6.1-6`](#rfc9329-6.1-6), [`RFC9329-6.1-7`](#rfc9329-6.1-7), [`RFC9329-6.1-8`](#rfc9329-6.1-8), [`RFC9329-6.1-9`](#rfc9329-6.1-9), [`RFC9329-6.2-1`](#rfc9329-6.2-1), [`RFC9329-6.3-1`](#rfc9329-6.3-1), [`RFC9329-6.6-1`](#rfc9329-6.6-1), [`RFC9329-6.7-1`](#rfc9329-6.7-1), [`RFC9329-7.1-1`](#rfc9329-7.1-1), [`RFC9329-7.1-2`](#rfc9329-7.1-2), [`RFC9329-7.2-1`](#rfc9329-7.2-1), [`RFC9329-7.5-1`](#rfc9329-7.5-1), [`RFC9329-9-1`](#rfc9329-9-1)

## Attainable ceiling

Mandatory ceiling: 30 of 30 (100.0%). Optional ceiling: 44 of 44 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc9329 30 of 30 (100.0%). Lowest optional ceiling: rfc9329 44 of 44 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9329-1.2-1` | If an IKE SA is rekeyed one or more times, the TCP Originator MUST remain the peer that originally initiated the first IKE SA. (§1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no TCP connection and no TCP Originator role; internal/component/ike/transport/udp.go::newTransport opens UDP only; internal/component/ike/transport/udp.go |
| `RFC9329-2-1` | Compliant implementations MUST support TCP encapsulation on TCP port 4500, which is reserved for IPsec NAT traversal. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not listen on or connect to TCP port 4500; internal/component/ike/transport/udp.go::newTransport binds a UDP socket and no TCP transport exists under internal/component/ike/; internal/component/ike/transport/udp.go |
| `RFC9329-3-1` | Empty messages (where the Length field equals 2) MUST be silently ignored by receiver. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze parses no Length-framed TCP stream; internal/component/ike/transport/udp.go |
| `RFC9329-3.1-1` | The value in the Length field MUST NOT be 0 or 1. (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze writes no TCP-encapsulated IKE message; internal/component/ike/transport/udp.go |
| `RFC9329-3.1-2` | The receiver MUST treat these values as fatal errors and MUST close the TCP connection. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no TCP-encapsulated IKE message and holds no TCP connection to close; internal/component/ike/transport/udp.go |
| `RFC9329-3.2-1` | The Security Parameter Index (SPI) field [RFC7296] in the ESP header MUST NOT be a zero value. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends no ESP packet over TCP, so no TCP-encapsulated ESP header exists to carry an SPI; internal/component/ike/transport/udp.go |
| `RFC9329-3.2-2` | The value in the Length field MUST NOT be 0 or 1. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze writes no TCP-encapsulated ESP packet; internal/component/ike/transport/udp.go |
| `RFC9329-3.2-3` | The receiver MUST treat these values as fatal errors and MUST close TCP connection. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reads no TCP-encapsulated ESP packet and holds no TCP connection to close; internal/component/ike/transport/udp.go |
| `RFC9329-4-1` | Each stream of bytes used for IKE and IPsec encapsulation MUST begin with a fixed sequence of 6 bytes as a magic value, containing the characters "IKETCP" as ASCII values. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends and checks no IKETCP stream prefix; internal/component/ike/transport/udp.go |
| `RFC9329-5-1` | If a Responder is configured to accept and is allowed to use TCP encapsulation, it MUST listen on the configured port(s) in case any peers will initiate new IKE sessions. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no TCP encapsulation configuration and no TCP listener; internal/component/ike/transport/udp.go::newTransport; internal/component/ike/transport/udp.go |
| `RFC9329-5-2` | If TCP encapsulation is being used for a specific IKE SA, all IKE messages for that IKE SA and ESP packets for its Child SAs MUST be sent over a TCP connection until the SA is deleted or IKEv2 Mobility and Multihoming (MOBIKE) is used to change the SA endpoints and/or the encapsulation protocol. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no IKE SA of Ze's ever uses TCP encapsulation; internal/component/ike/transport/udp.go |
| `RFC9329-5.1-1` | When switching from UDP to TCP, a new IKE_SA_INIT exchange MUST be initiated with the Initiator's new SPI and with recalculated content of NAT_DETECTION_*_IP notifications. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never falls back from UDP to TCP; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-1` | The first bytes sent on the TCP stream MUST be the stream prefix value (Section 4). (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze opens no TCP stream; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-2` | The TCP Responder MUST wait for the entire stream prefix to be received on the stream before trying to parse out any IKE or ESP messages. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze accepts no TCP stream; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-3` | Since new TCP connections may use different IP addresses and/or ports due to NAT mappings or local address or port allocations changing, the TCP Responder MUST allow packets for existing SAs to be received from new source IP addresses and ports. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze accepts no TCP connection for an existing SA; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-4` | Note that the IPv6 Flow-ID header MUST remain constant when a new TCP connection is created to avoid ECMP load balancing. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze creates no TCP connection, so no Flow Label is chosen for one; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-5` | A peer MUST discard a partially received message due to a broken connection. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reassembles no message from a TCP stream; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-6` | Whenever the TCP Originator opens a new TCP connection to be used for an existing IKE SA, it MUST send the stream prefix first, before any IKE or ESP messages. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze opens no TCP connection for an IKE SA; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-7` | Multiple IKE SAs MUST NOT share a single TCP connection, unless one is a rekey of an existing IKE SA, in which case there will temporarily be two IKE SAs on the same TCP connection. (§6.1) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze carries no IKE SA over TCP, so none share a connection, but no test proves it; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-8` | The TCP Responder MUST NOT accept any messages for the existing IKE session on a new incoming connection, unless that connection begins with the stream prefix. (§6.1) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze accepts no incoming TCP connection at all, but no test proves it; internal/component/ike/transport/udp.go |
| `RFC9329-6.1-9` | If there is instead a syntax issue within an IKE message, an implementation MUST send the INVALID_SYNTAX notify payload and tear down the IKE SA as usual, rather than tearing down the TCP connection directly. (§6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze receives no IKE message over TCP, so the TCP-specific choice between INVALID_SYNTAX and closing the connection never arises; internal/component/ike/transport/udp.go |
| `RFC9329-6.2-1` | If a new TCP connection for the IKE SA is established while the exchange Initiator is waiting for a response, the Initiator MUST retransmit its request over this connection and continue to wait for a response. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no TCP connection on which to retransmit; internal/component/ike/transport/udp.go |
| `RFC9329-6.3-1` | Note that the Responder MUST NOT include the Initiator's TCP port into the Cookie calculation (*) since the Cookie can be returned over a new TCP connection with a different port. (§6.3) | MUST NOT | 6.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze computes no cookie for a TCP-encapsulated IKE_SA_INIT, because none arrives over TCP; internal/component/ike/transport/udp.go |
| `RFC9329-6.6-1` | Any implementation using TCP encapsulation MUST silently drop incoming NAT-keepalive packets and not treat them as errors. (§6.6) | MUST | 6.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze receives no TCP-encapsulated stream, so it has no TCP NAT-keepalive handling; internal/component/ike/transport/udp.go |
| `RFC9329-6.7-1` | These MUST NOT be used as indications of IKE peer liveness, for which purpose the standard IKEv2 mechanism of exchanging (usually empty) INFORMATIONAL messages is used (see Section 1.4 of [RFC7296]). (§6.7) | MUST NOT | 6.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze uses no TCP or TLS keep-alive, so none stands in for IKE liveness, but no test proves it; internal/component/ike/transport/udp.go |
| `RFC9329-7.1-1` | Implementations that implement both MOBIKE and TCP encapsulation within the same connection configuration MUST support dynamically enabling and disabling TCP encapsulation as interfaces change. (§7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements MOBIKE but not TCP encapsulation, so MOBIKE never switches an SA to or from TCP; internal/component/ike/transport/udp.go |
| `RFC9329-7.1-2` | A new INFORMATIONAL exchange MUST NOT be started in this situation. (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence governs a MOBIKE update retried over a new TCP connection, and Ze has no TCP path; internal/component/ike/transport/udp.go |
| `RFC9329-7.2-1` | In other words, with the next security gateway, the client MUST first try UDP and then fall back to TCP while establishing a new IKE SA, regardless of the transport of the SA the redirect notification was received over (unless the client's configuration instructs it to instantly use TCP for the gateway it is redirected to). (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements neither the RFC 5685 redirect nor a TCP fallback; internal/component/ike/transport/udp.go |
| `RFC9329-7.5-1` | Even if fragmentation is negotiated, an implementation SHOULD NOT send fragments when going over a TCP connection, although it MUST support receiving fragments. (§7.5) | MUST | 7.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze receives nothing over TCP, so it cannot receive IKE fragments there; internal/component/ike/transport/udp.go |
| `RFC9329-9-1` | Implementations MUST favor using direct ESP or UDP encapsulation over TCP encapsulation whenever possible. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze uses direct ESP or UDP encapsulation only, but no test proves the preference; internal/component/ike/transport/udp.go |
| `RFC9329-2-2` | Since TCP encapsulation of IKE and IPsec packets adds overhead and has potential performance trade-offs compared to direct or UDP- encapsulated SAs (as described in Section 9), implementations SHOULD prefer ESP direct or UDP-encapsulated SAs over TCP-encapsulated SAs when possible. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-3-2` | Although a TCP stream may be able to send very long messages, implementations SHOULD limit message lengths to match the lengths used for UDP encapsulation of ESP messages. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-5.1-2` | If an Initiator implementation has no prior knowledge about the network it is on and the status of UDP on that network, it SHOULD always attempt to negotiate IKE over UDP first. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-10` | In order to close an IKE session, either the Initiator or Responder SHOULD gracefully tear down IKE SAs with DELETE payloads. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-11` | Once the SA has been deleted, the TCP Originator SHOULD close the TCP connection if it does not intend to use the connection for another IKE session to the TCP Responder. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-13` | If a DELETE payload has not been sent, both sides SHOULD maintain the state for their SAs for the standard lifetime or timeout period. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-14` | If either the TCP Originator or TCP Responder detects corruption on a connection that was started with a valid stream prefix, it SHOULD close the TCP connection. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-15` | Implementations SHOULD NOT tear down a connection if only a few consecutive ESP packets have unknown SPIs since the SPI databases may be momentarily out of sync. (§6.1) | SHOULD NOT | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-16` | A TCP Originator SHOULD only open one TCP connection per IKE SA, over which it sends all of the corresponding IKE and ESP messages. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-17` | As with TCP Originators, a TCP Responder SHOULD send packets for an IKE SA and its Child SAs over only one TCP connection at any given time. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-18` | It SHOULD choose the TCP connection on which it last received a valid and decryptable IKE or ESP message. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-19` | Since a connection may be broken and a new connection re-established by the TCP Originator without the TCP Responder being aware, a TCP Responder SHOULD accept receiving IKE and ESP messages on both old and new connections until the old connection is closed by the TCP Originator. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.2-2` | The exchange Initiator SHOULD NOT retransmit request message (*); if no response is received within some reasonable period of time, the IKE SA is deleted. (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.3-2` | The exchange Responder SHOULD NOT send an IKEv2 Cookie request without an accompanied Puzzle; implementations might choose to have exceptions to this for cases like mitigating TCP sequence number attacks. (§6.3) | SHOULD NOT | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.3-3` | If the Responder chooses to send a Cookie request (possibly along with Puzzle request), then the TCP connection that the IKE_SA_INIT request message was received over SHOULD be closed after the Responder sends its reply and no repeated requests are received within some short period of time to keep the Responder stateless (see Section 6.3.1). (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.4-1` | Thus, in the case of TCP encapsulation, an Initiator SHOULD NOT wait for additional messages in case it receives an error notification from the Responder in the IKE_SA_INIT exchange. (§6.4) | SHOULD NOT | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.4-2` | In the IKE_SA_INIT exchange, if the Responder returns an error notification that implies a recovery action from the Initiator (such as INVALID_KE_PAYLOAD or INVALID_MAJOR_VERSION, see Section 2.21.1 of [RFC7296]), then the Responder SHOULD NOT close the TCP connection immediately in anticipation of the fact that the Initiator will repeat the request with corrected parameters. (§6.4) | SHOULD NOT | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.5-1` | IKE_SA_INIT packets sent on a TCP connection SHOULD include these payloads with the same content as when sending over UDP and SHOULD use the applicable TCP ports when creating and checking the SHA-1 digests. (§6.5) | SHOULD | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.6-2` | In general, TCP port mappings are maintained by NATs longer than UDP port mappings, so IPsec ESP NAT-keepalive packets [RFC3948] SHOULD NOT be sent when using TCP encapsulation. (§6.6) | SHOULD NOT | 6.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.1-3` | When a MOBIKE-enabled Initiator changes networks, the INFORMATIONAL exchange with the UPDATE_SA_ADDRESSES notification SHOULD be initiated first over UDP before attempting over TCP. (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.1-4` | If the TCP transport was used for the previous network connection, the old TCP connection SHOULD be closed by the Initiator once MOBIKE finishes migration to a new connection (either TCP or UDP). (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.1-5` | In the case of TCP, the NO_NATS_ALLOWED notification SHOULD be ignored because TCP generally has no problems with NAT boxes. (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.1-6` | In the case of TCP encapsulation, this check has little value since a TCP handshake proves the routability of the TCP Originator's address; thus, the Return Routability Check SHOULD NOT be performed. (§7.1) | SHOULD NOT | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.3-1` | Since network conditions may change while the client is inactive, the fact that TCP encapsulation was used in an old SA SHOULD NOT affect which transport is used during session resumption. (§7.3) | SHOULD NOT | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.4-1` | This document makes the following recommendation: if support for High Availability in IKEv2 is negotiated and TCP transport is used, a client that is a TCP Originator SHOULD periodically send IKEv2 messages (e.g., by initiating liveness check exchange) whenever there is no IKEv2 or ESP traffic. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-7.5-2` | If an implementation supports both MOBIKE and IKE fragmentation, it SHOULD negotiate IKE fragmentation over a TCP-encapsulated session in case the session switches to UDP encapsulation on another network. (§7.5) | SHOULD | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-8-1` | If the IKE implementation has its own minimal implementation of TCP, it SHOULD still use common TCP behaviors to avoid being dropped by middleboxes. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-9.3-1` | Individual packets SHOULD NOT use different markings than the rest of the connection since packets with different priorities may be routed differently and cause unnecessary delays in the connection. (§9.3) | SHOULD NOT | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-9.4-1` | A TCP connection used for IKE encapsulation SHOULD negotiate its MSS in order to avoid unnecessary fragmentation of packets. (§9.4) | SHOULD | 9.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-9.5-1` | Implementations SHOULD follow the ECN compatibility mode for tunnel ingress as described in [RFC6040]. (§9.5) | SHOULD | 9.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-10-2` | Implementations SHOULD limit the attempts to resync, because if the injection attack is ongoing, then there is a high probability that the resync process will not succeed or will quickly come under attack again. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-10-3` | Therefore, implementations SHOULD make sure that IKE session state persists even if the underlying TCP connection is torn down. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-11-1` | This port SHOULD be used for TCP-encapsulated IKE and ESP as described in this document. (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-A-1` | The security of the IKE session is entirely derived from the IKE negotiation and key establishment and not from the TLS session (which, in this context, is only used for encapsulation purposes); therefore, when TLS is used on the TCP connection, both the TCP Originator and the TCP Responder SHOULD allow the NULL cipher to be selected for performance reasons. (§A) | SHOULD | A | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-A-2` | It is RECOMMENDED to follow [RFC9325] when selecting parameters for TLS. (§A) | RECOMMENDED | A | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-B.1-1` | During the TLS handshake, the server SHOULD NOT request the client's certificate since authentication is handled as part of IKE negotiation. (§B.1) | SHOULD NOT | B.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-B.3-1` | The client SHOULD attempt TLS session resumption if it has previously established a session with the server. (§B.3) | SHOULD | B.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-B.3-2` | If MOBIKE is being used, the Initiator SHOULD send an UPDATE_SA_ADDRESSES message. (§B.3) | SHOULD | B.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-5-3` | Initiators MAY use TCP encapsulation for any IKE session to a peer that is configured to support TCP encapsulation, although it is recommended that Initiators only use TCP encapsulation when traffic over UDP is blocked. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-12` | If the TCP connection is no longer associated with any active IKE SA, the TCP Responder MAY close the connection to clean up IKE resources if the TCP Originator didn't close it within some reasonable period of time (e.g., a few seconds). (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.1-20` | A TCP Responder MAY close a TCP connection that it perceives as idle and extraneous (one previously used for IKE and ESP messages that has been replaced by a new connection). (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.5-2` | Implementations MAY use the information that a NAT is present to influence keepalive timer values. (§6.5) | MAY | 6.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-6.7-2` | Note that, depending on the configuration of TCP and TLS on the connection, TCP keep-alives [RFC1122] and TLS keep-alives [RFC6520] MAY be used. (§6.7) | MAY | 6.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9329-10-1` | To avoid performance degradation caused by closing and re-creating TCP connections, implementations MAY alternatively try to resync after they receive unknown SPIs by searching the TCP stream for a 64-bit binary vector consisting of a known SPI in the first 32 bits and a valid Sequence Number for this SPI in the second 32 bits. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9329-1.2-1`](#rfc9329-1.2-1) If an IKE SA is rekeyed one or more times, the TCP Originator MUST remain the peer that originally initiated the first IKE SA. (§1.2) | {gap}, no test | Ze has no TCP connection and no TCP Originator role; internal/component/ike/transport/udp.go::newTransport opens UDP only; internal/component/ike/transport/udp.go |
| [`RFC9329-2-1`](#rfc9329-2-1) Compliant implementations MUST support TCP encapsulation on TCP port 4500, which is reserved for IPsec NAT traversal. (§2) | {gap}, no test | Ze does not listen on or connect to TCP port 4500; internal/component/ike/transport/udp.go::newTransport binds a UDP socket and no TCP transport exists under internal/component/ike/; internal/component/ike/transport/udp.go |
| [`RFC9329-3-1`](#rfc9329-3-1) Empty messages (where the Length field equals 2) MUST be silently ignored by receiver. (§3) | {gap}, no test | Ze parses no Length-framed TCP stream; internal/component/ike/transport/udp.go |
| [`RFC9329-3.1-1`](#rfc9329-3.1-1) The value in the Length field MUST NOT be 0 or 1. (§3.1) | {gap}, no test | Ze writes no TCP-encapsulated IKE message; internal/component/ike/transport/udp.go |
| [`RFC9329-3.1-2`](#rfc9329-3.1-2) The receiver MUST treat these values as fatal errors and MUST close the TCP connection. (§3.1) | {gap}, no test | Ze reads no TCP-encapsulated IKE message and holds no TCP connection to close; internal/component/ike/transport/udp.go |
| [`RFC9329-3.2-1`](#rfc9329-3.2-1) The Security Parameter Index (SPI) field [RFC7296] in the ESP header MUST NOT be a zero value. (§3.2) | {gap}, no test | Ze sends no ESP packet over TCP, so no TCP-encapsulated ESP header exists to carry an SPI; internal/component/ike/transport/udp.go |
| [`RFC9329-3.2-2`](#rfc9329-3.2-2) The value in the Length field MUST NOT be 0 or 1. (§3.2) | {gap}, no test | Ze writes no TCP-encapsulated ESP packet; internal/component/ike/transport/udp.go |
| [`RFC9329-3.2-3`](#rfc9329-3.2-3) The receiver MUST treat these values as fatal errors and MUST close TCP connection. (§3.2) | {gap}, no test | Ze reads no TCP-encapsulated ESP packet and holds no TCP connection to close; internal/component/ike/transport/udp.go |
| [`RFC9329-4-1`](#rfc9329-4-1) Each stream of bytes used for IKE and IPsec encapsulation MUST begin with a fixed sequence of 6 bytes as a magic value, containing the characters "IKETCP" as ASCII values. (§4) | {gap}, no test | Ze sends and checks no IKETCP stream prefix; internal/component/ike/transport/udp.go |
| [`RFC9329-5-1`](#rfc9329-5-1) If a Responder is configured to accept and is allowed to use TCP encapsulation, it MUST listen on the configured port(s) in case any peers will initiate new IKE sessions. (§5) | {gap}, no test | Ze has no TCP encapsulation configuration and no TCP listener; internal/component/ike/transport/udp.go::newTransport; internal/component/ike/transport/udp.go |
| [`RFC9329-5-2`](#rfc9329-5-2) If TCP encapsulation is being used for a specific IKE SA, all IKE messages for that IKE SA and ESP packets for its Child SAs MUST be sent over a TCP connection until the SA is deleted or IKEv2 Mobility and Multihoming (MOBIKE) is used to change the SA endpoints and/or the encapsulation protocol. (§5) | {gap}, no test | no IKE SA of Ze's ever uses TCP encapsulation; internal/component/ike/transport/udp.go |
| [`RFC9329-5.1-1`](#rfc9329-5.1-1) When switching from UDP to TCP, a new IKE_SA_INIT exchange MUST be initiated with the Initiator's new SPI and with recalculated content of NAT_DETECTION_*_IP notifications. (§5.1) | {gap}, no test | Ze never falls back from UDP to TCP; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-1`](#rfc9329-6.1-1) The first bytes sent on the TCP stream MUST be the stream prefix value (Section 4). (§6.1) | {gap}, no test | Ze opens no TCP stream; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-2`](#rfc9329-6.1-2) The TCP Responder MUST wait for the entire stream prefix to be received on the stream before trying to parse out any IKE or ESP messages. (§6.1) | {gap}, no test | Ze accepts no TCP stream; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-3`](#rfc9329-6.1-3) Since new TCP connections may use different IP addresses and/or ports due to NAT mappings or local address or port allocations changing, the TCP Responder MUST allow packets for existing SAs to be received from new source IP addresses and ports. (§6.1) | {gap}, no test | Ze accepts no TCP connection for an existing SA; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-4`](#rfc9329-6.1-4) Note that the IPv6 Flow-ID header MUST remain constant when a new TCP connection is created to avoid ECMP load balancing. (§6.1) | {gap}, no test | Ze creates no TCP connection, so no Flow Label is chosen for one; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-5`](#rfc9329-6.1-5) A peer MUST discard a partially received message due to a broken connection. (§6.1) | {gap}, no test | Ze reassembles no message from a TCP stream; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-6`](#rfc9329-6.1-6) Whenever the TCP Originator opens a new TCP connection to be used for an existing IKE SA, it MUST send the stream prefix first, before any IKE or ESP messages. (§6.1) | {gap}, no test | Ze opens no TCP connection for an IKE SA; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-7`](#rfc9329-6.1-7) Multiple IKE SAs MUST NOT share a single TCP connection, unless one is a rekey of an existing IKE SA, in which case there will temporarily be two IKE SAs on the same TCP connection. (§6.1) | {gap}, no test | unproven rather than known divergent. Ze carries no IKE SA over TCP, so none share a connection, but no test proves it; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-8`](#rfc9329-6.1-8) The TCP Responder MUST NOT accept any messages for the existing IKE session on a new incoming connection, unless that connection begins with the stream prefix. (§6.1) | {gap}, no test | unproven rather than known divergent. Ze accepts no incoming TCP connection at all, but no test proves it; internal/component/ike/transport/udp.go |
| [`RFC9329-6.1-9`](#rfc9329-6.1-9) If there is instead a syntax issue within an IKE message, an implementation MUST send the INVALID_SYNTAX notify payload and tear down the IKE SA as usual, rather than tearing down the TCP connection directly. (§6.1) | {gap}, no test | Ze receives no IKE message over TCP, so the TCP-specific choice between INVALID_SYNTAX and closing the connection never arises; internal/component/ike/transport/udp.go |
| [`RFC9329-6.2-1`](#rfc9329-6.2-1) If a new TCP connection for the IKE SA is established while the exchange Initiator is waiting for a response, the Initiator MUST retransmit its request over this connection and continue to wait for a response. (§6.2) | {gap}, no test | Ze has no TCP connection on which to retransmit; internal/component/ike/transport/udp.go |
| [`RFC9329-6.3-1`](#rfc9329-6.3-1) Note that the Responder MUST NOT include the Initiator's TCP port into the Cookie calculation (*) since the Cookie can be returned over a new TCP connection with a different port. (§6.3) | {gap}, no test | Ze computes no cookie for a TCP-encapsulated IKE_SA_INIT, because none arrives over TCP; internal/component/ike/transport/udp.go |
| [`RFC9329-6.6-1`](#rfc9329-6.6-1) Any implementation using TCP encapsulation MUST silently drop incoming NAT-keepalive packets and not treat them as errors. (§6.6) | {gap}, no test | Ze receives no TCP-encapsulated stream, so it has no TCP NAT-keepalive handling; internal/component/ike/transport/udp.go |
| [`RFC9329-6.7-1`](#rfc9329-6.7-1) These MUST NOT be used as indications of IKE peer liveness, for which purpose the standard IKEv2 mechanism of exchanging (usually empty) INFORMATIONAL messages is used (see Section 1.4 of [RFC7296]). (§6.7) | {gap}, no test | unproven rather than known divergent. Ze uses no TCP or TLS keep-alive, so none stands in for IKE liveness, but no test proves it; internal/component/ike/transport/udp.go |
| [`RFC9329-7.1-1`](#rfc9329-7.1-1) Implementations that implement both MOBIKE and TCP encapsulation within the same connection configuration MUST support dynamically enabling and disabling TCP encapsulation as interfaces change. (§7.1) | {gap}, no test | Ze implements MOBIKE but not TCP encapsulation, so MOBIKE never switches an SA to or from TCP; internal/component/ike/transport/udp.go |
| [`RFC9329-7.1-2`](#rfc9329-7.1-2) A new INFORMATIONAL exchange MUST NOT be started in this situation. (§7.1) | {gap}, no test | the sentence governs a MOBIKE update retried over a new TCP connection, and Ze has no TCP path; internal/component/ike/transport/udp.go |
| [`RFC9329-7.2-1`](#rfc9329-7.2-1) In other words, with the next security gateway, the client MUST first try UDP and then fall back to TCP while establishing a new IKE SA, regardless of the transport of the SA the redirect notification was received over (unless the client's configuration instructs it to instantly use TCP for the gateway it is redirected to). (§7.2) | {gap}, no test | Ze implements neither the RFC 5685 redirect nor a TCP fallback; internal/component/ike/transport/udp.go |
| [`RFC9329-7.5-1`](#rfc9329-7.5-1) Even if fragmentation is negotiated, an implementation SHOULD NOT send fragments when going over a TCP connection, although it MUST support receiving fragments. (§7.5) | {gap}, no test | Ze receives nothing over TCP, so it cannot receive IKE fragments there; internal/component/ike/transport/udp.go |
| [`RFC9329-9-1`](#rfc9329-9-1) Implementations MUST favor using direct ESP or UDP encapsulation over TCP encapsulation whenever possible. (§9) | {gap}, no test | unproven rather than known divergent. Ze uses direct ESP or UDP encapsulation only, but no test proves the preference; internal/component/ike/transport/udp.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9329-1.2-1`](#rfc9329-1.2-1)

If an IKE SA is rekeyed one or more times, the TCP Originator MUST remain the peer that originally initiated the first IKE SA. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-1.2-1, so no unit is bound to it.

### [`RFC9329-2-1`](#rfc9329-2-1)

Compliant implementations MUST support TCP encapsulation on TCP port 4500, which is reserved for IPsec NAT traversal. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-2-1, so no unit is bound to it.

### [`RFC9329-3-1`](#rfc9329-3-1)

Empty messages (where the Length field equals 2) MUST be silently ignored by receiver. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3-1, so no unit is bound to it.

### [`RFC9329-3.1-1`](#rfc9329-3.1-1)

The value in the Length field MUST NOT be 0 or 1. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3.1-1, so no unit is bound to it.

### [`RFC9329-3.1-2`](#rfc9329-3.1-2)

The receiver MUST treat these values as fatal errors and MUST close the TCP connection. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3.1-2, so no unit is bound to it.

### [`RFC9329-3.2-1`](#rfc9329-3.2-1)

The Security Parameter Index (SPI) field [RFC7296] in the ESP header MUST NOT be a zero value. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3.2-1, so no unit is bound to it.

### [`RFC9329-3.2-2`](#rfc9329-3.2-2)

The value in the Length field MUST NOT be 0 or 1. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3.2-2, so no unit is bound to it.

### [`RFC9329-3.2-3`](#rfc9329-3.2-3)

The receiver MUST treat these values as fatal errors and MUST close TCP connection. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-3.2-3, so no unit is bound to it.

### [`RFC9329-4-1`](#rfc9329-4-1)

Each stream of bytes used for IKE and IPsec encapsulation MUST begin with a fixed sequence of 6 bytes as a magic value, containing the characters "IKETCP" as ASCII values. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-4-1, so no unit is bound to it.

### [`RFC9329-5-1`](#rfc9329-5-1)

If a Responder is configured to accept and is allowed to use TCP encapsulation, it MUST listen on the configured port(s) in case any peers will initiate new IKE sessions. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-5-1, so no unit is bound to it.

### [`RFC9329-5-2`](#rfc9329-5-2)

If TCP encapsulation is being used for a specific IKE SA, all IKE messages for that IKE SA and ESP packets for its Child SAs MUST be sent over a TCP connection until the SA is deleted or IKEv2 Mobility and Multihoming (MOBIKE) is used to change the SA endpoints and/or the encapsulation protocol. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-5-2, so no unit is bound to it.

### [`RFC9329-5.1-1`](#rfc9329-5.1-1)

When switching from UDP to TCP, a new IKE_SA_INIT exchange MUST be initiated with the Initiator's new SPI and with recalculated content of NAT_DETECTION_*_IP notifications. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-5.1-1, so no unit is bound to it.

### [`RFC9329-6.1-1`](#rfc9329-6.1-1)

The first bytes sent on the TCP stream MUST be the stream prefix value (Section 4). (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-1, so no unit is bound to it.

### [`RFC9329-6.1-2`](#rfc9329-6.1-2)

The TCP Responder MUST wait for the entire stream prefix to be received on the stream before trying to parse out any IKE or ESP messages. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-2, so no unit is bound to it.

### [`RFC9329-6.1-3`](#rfc9329-6.1-3)

Since new TCP connections may use different IP addresses and/or ports due to NAT mappings or local address or port allocations changing, the TCP Responder MUST allow packets for existing SAs to be received from new source IP addresses and ports. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-3, so no unit is bound to it.

### [`RFC9329-6.1-4`](#rfc9329-6.1-4)

Note that the IPv6 Flow-ID header MUST remain constant when a new TCP connection is created to avoid ECMP load balancing. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-4, so no unit is bound to it.

### [`RFC9329-6.1-5`](#rfc9329-6.1-5)

A peer MUST discard a partially received message due to a broken connection. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-5, so no unit is bound to it.

### [`RFC9329-6.1-6`](#rfc9329-6.1-6)

Whenever the TCP Originator opens a new TCP connection to be used for an existing IKE SA, it MUST send the stream prefix first, before any IKE or ESP messages. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-6, so no unit is bound to it.

### [`RFC9329-6.1-7`](#rfc9329-6.1-7)

Multiple IKE SAs MUST NOT share a single TCP connection, unless one is a rekey of an existing IKE SA, in which case there will temporarily be two IKE SAs on the same TCP connection. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-7, so no unit is bound to it.

### [`RFC9329-6.1-8`](#rfc9329-6.1-8)

The TCP Responder MUST NOT accept any messages for the existing IKE session on a new incoming connection, unless that connection begins with the stream prefix. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-8, so no unit is bound to it.

### [`RFC9329-6.1-9`](#rfc9329-6.1-9)

If there is instead a syntax issue within an IKE message, an implementation MUST send the INVALID_SYNTAX notify payload and tear down the IKE SA as usual, rather than tearing down the TCP connection directly. (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.1-9, so no unit is bound to it.

### [`RFC9329-6.2-1`](#rfc9329-6.2-1)

If a new TCP connection for the IKE SA is established while the exchange Initiator is waiting for a response, the Initiator MUST retransmit its request over this connection and continue to wait for a response. (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.2-1, so no unit is bound to it.

### [`RFC9329-6.3-1`](#rfc9329-6.3-1)

Note that the Responder MUST NOT include the Initiator's TCP port into the Cookie calculation (*) since the Cookie can be returned over a new TCP connection with a different port. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.3-1, so no unit is bound to it.

### [`RFC9329-6.6-1`](#rfc9329-6.6-1)

Any implementation using TCP encapsulation MUST silently drop incoming NAT-keepalive packets and not treat them as errors. (§6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.6-1, so no unit is bound to it.

### [`RFC9329-6.7-1`](#rfc9329-6.7-1)

These MUST NOT be used as indications of IKE peer liveness, for which purpose the standard IKEv2 mechanism of exchanging (usually empty) INFORMATIONAL messages is used (see Section 1.4 of [RFC7296]). (§6.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-6.7-1, so no unit is bound to it.

### [`RFC9329-7.1-1`](#rfc9329-7.1-1)

Implementations that implement both MOBIKE and TCP encapsulation within the same connection configuration MUST support dynamically enabling and disabling TCP encapsulation as interfaces change. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-7.1-1, so no unit is bound to it.

### [`RFC9329-7.1-2`](#rfc9329-7.1-2)

A new INFORMATIONAL exchange MUST NOT be started in this situation. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-7.1-2, so no unit is bound to it.

### [`RFC9329-7.2-1`](#rfc9329-7.2-1)

In other words, with the next security gateway, the client MUST first try UDP and then fall back to TCP while establishing a new IKE SA, regardless of the transport of the SA the redirect notification was received over (unless the client's configuration instructs it to instantly use TCP for the gateway it is redirected to). (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-7.2-1, so no unit is bound to it.

### [`RFC9329-7.5-1`](#rfc9329-7.5-1)

Even if fragmentation is negotiated, an implementation SHOULD NOT send fragments when going over a TCP connection, although it MUST support receiving fragments. (§7.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-7.5-1, so no unit is bound to it.

### [`RFC9329-9-1`](#rfc9329-9-1)

Implementations MUST favor using direct ESP or UDP encapsulation over TCP encapsulation whenever possible. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9329-9-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc9329.txt |
| Source fingerprint | 3d8cc0d5a6d18adb |
| Record | rfc/extraction/rfc9329.json |
| Mapped sentences | 30 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 1 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 9 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `6.3` | not stated | 1 | walked | not stated |
| `6.3.1` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 1 | walked | not stated |
| `6.7` | not stated | 1 | walked | not stated |
| `6.8` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 2 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 0 | walked | not stated |
| `9.4` | not stated | 0 | walked | not stated |
| `9.5` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `B.1` | not stated | 0 | walked | not stated |
| `B.2` | not stated | 0 | walked | not stated |
| `B.3` | not stated | 0 | walked | not stated |
| `B.4` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 9329 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 9329, so its obligations are stated where they were written.
