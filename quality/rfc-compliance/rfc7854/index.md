# RFC 7854 - BGP Monitoring Protocol (BMP)

Partial. Every requirement this repository extracted from RFC 7854, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 79.5% | 31 of 39 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.5% | 8 of 39 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 39 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 39 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 39 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 49.5% | 47 of 95 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 39 | of 48 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 39 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 39 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 39 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 39 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 43 | of 39 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 39 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 48 |
| Gated MUST-level | 39 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 95 |
| Tagged units | 95 |
| Recorded audit verdicts | 43 |
| Discrimination records | 47 |
| Summary | `rfc/short/rfc7854.md` |
| Requirement shard | `rfc/requirements/rfc7854.md` |
| RFC text | `rfc/full/rfc7854.txt` |

## Enrolment

Enrolled: Ze implements the BMP sender and receiver under internal/component/bgp/plugins/bmp. Wire-message and session carriers cover the common and per-peer headers, OPEN exchange, current per-peer replay and completion, identity, ordered information, termination and statistics. Requirement coverage and discrimination remain subject to the current gate result.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- BMP receiver and sender, peer lifecycle, current per-NLRI replay with per-family End-of-RIB, received and sent route monitoring, CLI and configuration. Initiation carries the administrative system identity
- receiver information strings retain order and duplicates. Sender shutdown carries Reason 0 and closes
- bounded transmit queues reset stalled sessions.


**What the ledger says remains**

Focused execution, producer discrimination and third-party pmacct acceptance of the current changes remain unrun. Loc-RIB monitoring is defined under RFC 9069 and Adj-RIB-Out under RFC 8671. Post-policy Adj-RIB-In monitoring, the optional view with the L flag set on received routes, is not offered: the route-monitoring-policy post-policy choice streams the Adj-RIB-Out.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 31 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **39** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (31):** [`RFC7854-x-1`](#rfc7854-x-1), [`RFC7854-x-2`](#rfc7854-x-2), [`RFC7854-x-3`](#rfc7854-x-3), [`RFC7854-x-5`](#rfc7854-x-5), [`RFC7854-x-8`](#rfc7854-x-8), [`RFC7854-4.9-1`](#rfc7854-4.9-1), [`RFC7854-4.5-2`](#rfc7854-4.5-2), [`RFC7854-x-18`](#rfc7854-x-18), [`RFC7854-3.2-1`](#rfc7854-3.2-1), [`RFC7854-3.2-2`](#rfc7854-3.2-2), [`RFC7854-3.3-1`](#rfc7854-3.3-1), [`RFC7854-4.1-1`](#rfc7854-4.1-1), [`RFC7854-4.2-1`](#rfc7854-4.2-1), [`RFC7854-4.3-2`](#rfc7854-4.3-2), [`RFC7854-4.4-1`](#rfc7854-4.4-1), [`RFC7854-4.4-2`](#rfc7854-4.4-2), [`RFC7854-4.4-3`](#rfc7854-4.4-3), [`RFC7854-4.5-1`](#rfc7854-4.5-1), [`RFC7854-4.5-3`](#rfc7854-4.5-3), [`RFC7854-4.5-4`](#rfc7854-4.5-4), [`RFC7854-4.7-1`](#rfc7854-4.7-1), [`RFC7854-4.7-2`](#rfc7854-4.7-2), [`RFC7854-4.7-3`](#rfc7854-4.7-3), [`RFC7854-4.8-1`](#rfc7854-4.8-1), [`RFC7854-4.8-2`](#rfc7854-4.8-2), [`RFC7854-4.8-3`](#rfc7854-4.8-3), [`RFC7854-4.10-1`](#rfc7854-4.10-1), [`RFC7854-5-1`](#rfc7854-5-1), [`RFC7854-5-2`](#rfc7854-5-2), [`RFC7854-5-4`](#rfc7854-5-4), [`RFC7854-8.2-1`](#rfc7854-8.2-1)

**Annotated (including scoped evidence) (8):** [`RFC7854-x-4`](#rfc7854-x-4), [`RFC7854-x-6`](#rfc7854-x-6), [`RFC7854-x-7`](#rfc7854-x-7), [`RFC7854-x-9`](#rfc7854-x-9), [`RFC7854-x-10`](#rfc7854-x-10), [`RFC7854-x-12`](#rfc7854-x-12), [`RFC7854-4.5-5`](#rfc7854-4.5-5), [`RFC7854-5-6`](#rfc7854-5-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7854-x-1` | This is set to '3' for all messages defined in this specification. (§4.1, Common Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L18). **positive:** `unit/verify` [`TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L68). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L37). **negative:** `unit/verify` [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_test.go#L77) |
| `RFC7854-x-2` | Version (1 byte): Indicates the BMP version. This is set to '3' for all messages defined in this specification. ('1' and '2' were used by draft versions of this document.) Version 0 is reserved and MUST NOT be sent. o Message Length (4 bytes): Length of the message in bytes (including headers, data, and encapsulated messages, if any). o Message Type (1 byte): This identifies the type of the BMP message. (§4.1, Common Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestBMPCommonHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L83). **positive:** `unit/verify` [`TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L70). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L30) |
| `RFC7854-x-3` | The per-peer header follows the common header for most BMP messages. (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L285). **positive:** `unit/verify` [`TestRFC7854PerPeerHeaderFollowsTheCommonHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L106). **negative:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L287). **negative:** `unit/verify` [`TestRFC7854PerPeerHeaderFollowsTheCommonHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L110) |
| `RFC7854-x-4` | If a 16-bit AS number is stored in this field [RFC6793], it should be padded with zeroes in the 16 most significant bits. (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116). **positive:** `unit/verify` [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173). **positive:** `unit/verify` [`TestRFC7854PeerASIsFourOctetsWithTheShortASZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L148). **negative:** no negative test. **{single-polarity}:** the Peer AS is unconditionally a 4-octet field on every encode and decode, so there is no shorter-AS variant to reject and no negative case to construct |
| `RFC7854-x-5` | It is 4 bytes long if an IPv4 address is carried in this field (with the 12 most significant bytes zero-filled) and 16 bytes long if an IPv6 address is carried in this field. (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L379). **negative:** `unit/verify` [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L393) |
| `RFC7854-x-6` | An initiation message MUST be sent as the first message after the TCP session comes up. (§4.3, Initiation Message) | MUST | 4.3 | **positive:** `unit/verify` [`TestBMPSenderConnects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L19). **negative:** no negative test. **{single-polarity}:** the sender always emits Initiation as the first message on a fresh connection, so there is no valid session in which another message precedes it to reject |
| `RFC7854-x-7` | The sysDescr and sysName Information TLVs MUST be sent, any others are optional. (§4.3, Initiation Message) | MUST | 4.3 | **positive:** `unit/verify` [`TestBMPSenderInitiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L84). **negative:** no negative test. **{single-polarity}:** the Initiation the sender builds always includes the sysName TLV, so there is no valid Initiation omitting it to assert against |
| `RFC7854-x-8` | Sent OPEN Message: The full OPEN message transmitted by the monitored router to its peer. o Received OPEN Message: The full OPEN message received by the monitored router from its peer. (§4.10, Peer Up Notification) | MUST | 4.10 | **positive:** `unit/verify` [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L211). **positive:** `unit/verify` [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L136). **negative:** `unit/verify` [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerup_openless_test.go#L40) |
| `RFC7854-x-9` | Following the common BMP header and per-peer header is a BGP Update PDU. (§4.6, Route Monitoring) | MUST | 4.6 | **positive:** `unit/verify` [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L279). **negative:** no negative test. **{single-polarity}:** Route Monitoring is only ever constructed around a complete BGP UPDATE PDU, so there is no valid Route Monitoring lacking one to reject |
| `RFC7854-x-10` | Reason indicates why the session was closed. (§4.9, Peer Down Notification) | MUST | 4.9 | **positive:** `unit/verify` [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L248). **positive:** `unit/verify` [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L269). **negative:** no negative test. **{single-polarity}:** Peer Down is always written with a reason byte, so there is no valid Peer Down without one to assert against |
| `RFC7854-x-11` | The router MAY send a Termination message prior to closing the session. (§3.3, Lifecycle of a BMP Session) | MAY | 3.3 | **positive:** `unit/verify` [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L363). **positive:** `unit/verify` [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_queue_test.go#L561). **negative:** no negative test. **{single-polarity}:** Termination is produced unconditionally when the session is torn down, so there is no valid shutdown that omits it to reject |
| `RFC7854-x-12` | No BMP message is ever sent from the monitoring station to the monitored router. (§3.2, Connection Establishment and Termination) | MUST | 3.2 | **positive:** `unit/verify` [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_test.go#L135). **negative:** no negative test. **{single-polarity}:** the receiver loop (internal/component/bgp/plugins/bmp/bmp.go:441-492) issues only reads and the sender hold-loop (sender.go:207-237) reads only to detect close, so neither role writes toward the monitored router on a valid session and there is no reject case to construct |
| `RFC7854-4.9-1` | Reason 1: The local system closed the session. Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that would have been sent to the peer. o Reason 2: The local system closed the session. No notification message was sent. Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session (see Section 8.1 of [RFC4271]). Two bytes both set to 0 are used to indicate that no relevant Event code is defined. o Reason 3: The remote system closed the session with a notification message. Following the Reason is a BGP PDU containing the BGP NOTIFICATION message as received from the peer. (§4.9, Peer Down Notification) | MUST | 4.9 | **positive:** `unit/verify` [`TestRFC7854PeerDownCarriesTheDataItsReasonRequires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go#L88). **negative:** `unit/verify` [`TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go#L148) |
| `RFC7854-4.5-2` | Likewise, the monitoring station MUST close the TCP session after receiving a termination message. (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_close_test.go#L9). **negative:** `unit/verify` [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_absent_test.go#L9) |
| `RFC7854-x-13` | Exponential backoff with a default initial backoff of 30 seconds and a maximum of 720 seconds is suggested. (§3.2, Connection Establishment and Termination) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-14` | Exponential backoff with a default initial backoff of 30 seconds and a maximum of 720 seconds is suggested. (§3.2, Connection Establishment and Termination) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-15` | It MAY periodically send Stats Reports or even new Initiation messages, according to configuration. (§3.3, Lifecycle of a BMP Session) | MAY | 3.3 | **positive:** `unit/verify` [`TestRFC7854StatisticsTimeoutSendsPeriodicReports`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L422). **negative:** `unit/verify` [`TestRFC7854StatisticsTimeoutZeroSendsNoReport`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L475) |
| `RFC7854-x-16` | It subsequently sends a Peer Up message over the BMP session for each of its monitored BGP peers that is in the Established state. (§3.3, Lifecycle of a BMP Session) | SHOULD | 3.3 | **positive:** `unit/verify` [`TestRFC7854SessionPeerUpOnlyForEstablishedPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L95). **negative:** `unit/verify` [`TestRFC7854SessionPeerUpOnlyForEstablishedPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L97) |
| `RFC7854-x-17` | It subsequently sends a Peer Up message over the BMP session for each of its monitored BGP peers that is in the Established state. It follows by sending the contents of its Adj- RIBs-In (pre-policy, post-policy, or both, see Section 5) encapsulated in Route Monitoring messages. (§3.3, Lifecycle of a BMP Session) | SHOULD | 3.3 | **positive:** `unit/verify` [`TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L128). **negative:** `unit/verify` [`TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L131) |
| `RFC7854-x-18` | The sysDescr and sysName Information TLVs MUST be sent, any others are optional. (§4.3, Initiation Message) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC7854InitiationCarriesSysDescr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L29). **positive:** `unit/verify` [`TestRFC7854InitiationCarriesSysName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L19). **positive:** `unit/verify` [`TestRFC7854InitiationEmptyIdentityValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L40). **negative:** `unit/verify` [`TestBMPInitiationMissingSysNameEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L28). **negative:** `unit/verify` [`TestRFC7854InitiationMissingSysDescrEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L15) |
| `RFC7854-x-19` | Information: Information about the peer, using the Information TLV (Section 4.4) format. Only the string type is defined in this context; it may be repeated. Inclusion of the Information field is OPTIONAL. (§4.10, Peer Up Notification) | MAY | 4.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-20` | Route Mirroring messages are used for verbatim duplication of messages as received. A possible use for mirroring is exact mirroring of one or more monitored BGP sessions, without state compression. (§4.7, Route Mirroring) | MAY | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-3.2-1` | Retries MUST be subject to some variety of backoff. (§3.2, Connection Establishment and Termination) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7854RetryBackoffDoubles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L64). **negative:** `unit/verify` [`TestRFC7854RetryNotBeforeBackoff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L114) |
| `RFC7854-3.2-2` | The router MUST also restrict the rate at which sessions may be established. (§3.2, Connection Establishment and Termination) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L173). **negative:** `unit/verify` [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L174) |
| `RFC7854-3.3-1` | Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer (§3.3, Lifecycle of a BMP Session, and restated in the Route Monitoring section) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC7854AdjReplayEndsEachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L67). **negative:** `unit/verify` [`TestRFC7854AdjReplayCompletionIsSessionScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L95). **negative:** `unit/verify` [`TestRFC7854IncompleteAdjReplayCannotClaimCompletion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L198) |
| `RFC7854-4.1-1` | A BMP implementation MUST ignore unrecognized message types upon receipt. (§4.1, Common Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC7854UnrecognizedMessageTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L252). **negative:** `unit/verify` [`TestRFC7854MalformedKnownTypeStillEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L261) |
| `RFC7854-4.2-1` | The remaining bits are reserved for future use. They MUST be transmitted as 0 and their values MUST be ignored on receipt. (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7854ReservedFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L337). **negative:** `unit/verify` [`TestRFC7854ReservedFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L357) |
| `RFC7854-4.3-1` | The string TLV MAY be included multiple times. (§4.3, Initiation Message) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-4.3-2` | The initiation message consists of the common BMP header followed by two or more Information TLVs (Section 4.4) containing information about the monitored router. (§4.3, Initiation Message) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC7854InitiationIsTheCommonHeaderThenTwoOrMoreTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L70). **negative:** `unit/verify` [`TestRFC7854InitiationWithFewerThanTwoTLVsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L99) |
| `RFC7854-4.4-1` | If multiple strings are included, their ordering MUST be preserved when they are reported. (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L21). **positive:** `unit/verify` [`TestRFC7854PeerUpStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L44). **negative:** `unit/verify` [`TestRFC7854InitiationStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L32). **negative:** `unit/verify` [`TestRFC7854PeerUpStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L55) |
| `RFC7854-4.4-2` | Type = 1: sysDescr. The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysDescr MIB-II [RFC1213] object. (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationDescribesRunningSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L205). **negative:** `unit/verify` [`TestRFC7854InitiationDescriptionChangesWithBuild`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L227) |
| `RFC7854-4.4-3` | Type = 2: sysName. The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysName MIB-II [RFC1213] object. (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationUsesConfiguredSystemName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L168). **negative:** `unit/verify` [`TestRFC7854InitiationSystemNameChanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L188) |
| `RFC7854-4.5-1` | Once the router has sent a termination message, it MUST close the TCP session without sending any further messages. (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L384). **negative:** `unit/verify` [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L385) |
| `RFC7854-4.5-3` | Type = 1: Reason. The Information field contains a 2-byte code indicating the reason that the connection was terminated. Some reasons may have further TLVs associated with them. Inclusion of this TLV is REQUIRED. (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC7854SenderTerminationCarriesReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L54). **negative:** `unit/verify` [`TestRFC7854TerminationRequiresTwoByteReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L86) |
| `RFC7854-4.5-4` | The termination message consists of the common BMP header followed by one or more TLVs containing information about the reason for the termination (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC7854TerminationIsTheCommonHeaderThenTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L113). **negative:** `unit/verify` [`TestRFC7854TerminationWithNoTLVIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L149) |
| `RFC7854-4.5-5` | Although use of this message is RECOMMENDED, a monitoring station must always be prepared for the session to terminate with no message. (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_absent_test.go#L74). **positive:** `unit/verify` [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_queue_test.go#L568). **negative:** no negative test. **{single-polarity}:** a router that closes with no Termination message sends no non-compliant input, because the sentence permits exactly that close, so the station has nothing to refuse. TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination proves the station clause and TestSenderStopSendsTerminationToCollector proves the RECOMMENDED clause on the router side |
| `RFC7854-4.7-1` | If the BGP Message TLV occurs in the Route Mirroring message, it MUST occur last in the list of TLVs. (§4.7, Route Mirroring) | MUST | 4.7 | **positive:** `unit/verify` [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L120). **positive:** `unit/verify` [`TestRFC7854RouteMirroringBGPMessageTLVLast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L421). **negative:** `unit/verify` [`TestRFC7854MirrorBGPMessageNotLastEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L134) |
| `RFC7854-4.7-2` | Code = 0: Errored PDU. The contained message was found to have some error that made it unusable, causing it to be treated-as- withdraw [RFC7606]. A BGP Message TLV MUST also occur in the TLV list. (§4.7, Route Mirroring) | MUST | 4.7 | **positive:** `unit/verify` [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L121). **negative:** `unit/verify` [`TestRFC7854ErroredMirrorWithoutPDUEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L146) |
| `RFC7854-4.7-3` | Following the common BMP header and per-peer header is a set of TLVs that contain information about a message or set of messages. (§4.7, Route Mirroring) | MUST | 4.7 | **positive:** `unit/verify` [`TestRFC7854RouteMirroringIsThePerPeerHeaderThenTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L176). **negative:** `unit/verify` [`TestRFC7854RouteMirroringWhoseBodyIsNotTLVsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L209) |
| `RFC7854-4.8-1` | A BMP implementation MUST ignore unrecognized stat types on receipt, and likewise MUST ignore unexpected data in the Stat Data field. (§4.8, Stats Reports) | MUST | 4.8 | **positive:** `unit/verify` [`TestRFC7854UnrecognizedStatTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L278). **negative:** `unit/verify` [`TestRFC7854TruncatedStatEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L290) |
| `RFC7854-4.8-2` | However, if an SR message is transmitted, at least one statistic MUST be carried in it. (§4.8, Stats Reports) | MUST | 4.8 | **positive:** `unit/verify` [`TestRFC7854StatsReportCarriesAStatistic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L451). **negative:** `unit/verify` [`TestRFC7854SenderRefusesEmptyStatistics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L175) |
| `RFC7854-4.8-3` | Following the common BMP header and per-peer header is a 4-byte field that indicates the number of counters in the stats message where each counter is encoded as a TLV. (§4.8, Stats Reports) | MUST | 4.8 | **positive:** `unit/verify` [`TestRFC7854StatsReportCountNamesTheCountersThatFollow`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L238). **negative:** `unit/verify` [`TestRFC7854StatsReportCountDisagreeingWithItsCountersIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L270) |
| `RFC7854-4.10-1` | Following the common BMP header and per-peer header is the following (§4.10, Peer Up Notification) | MUST | 4.10 | **positive:** `unit/verify` [`TestRFC7854PeerUpIsThePerPeerHeaderThenTheFixedFieldsAndBothOPENs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L294). **negative:** `unit/verify` [`TestRFC7854PeerUpShorterThanItsFixedFieldsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L339) |
| `RFC7854-5-1` | Pre-policy routes MUST have their L flag clear in the BMP header (see Section 4), post-policy routes MUST have their L flag set. (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L478). **negative:** `unit/verify` [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L503) |
| `RFC7854-5-2` | If the implementation is able to provide information about when routes were received, it MAY provide such information in the BMP Timestamp field. Otherwise, the BMP Timestamp field MUST be set to 0, indicating that time is not available. (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854UnknownRouteTimeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L122). **negative:** `unit/verify` [`TestRFC7854KnownRouteTimeSurvivesReplay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L142) |
| `RFC7854-5-4` | The withdraw MUST have its L flag set to correspond to that of any previous announcement (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L479). **negative:** `unit/verify` [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L504) |
| `RFC7854-5-6` | When a change occurs to a route, such as an attribute change, the router must update the monitoring station with the new attribute. (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854ChangedAttributesCanReturnToEarlierValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L247). **negative:** no negative test. **{single-polarity}:** every received UPDATE that changes a route is compliant input to the router, so the sender holds no refusal path. TestRFC7854ChangedAttributesCanReturnToEarlierValue asserts one Route Monitoring message for each ORIGIN change, including the return to an earlier value |
| `RFC7854-8.2-1` | Since in this case no transport session actually exists, the Local and Remote Port fields of the Peer Up message MUST be set to 0. (§8.2, Peer Up Notification) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC7854LocRIBPeerUpPortsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L528). **negative:** `unit/verify` [`TestRFC7854BGPPeerUpCarriesTransportPorts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L542) |

## Gaps and untested MUSTs

RFC 7854 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7854-x-1`](#rfc7854-x-1)

This is set to '3' for all messages defined in this specification. (§4.1, Common Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. rfc7854_wire_header_test.go::TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength builds all seven types through the writers the sender calls (writeRouteMonitoring, writeStatisticsReport, writePeerDown, writePeerUp, writeInitiation, writeTermination, writeRouteMirroring) into a 0xA5-patterned buffer and requires octet 0 == 3 on each, so a writer that emits another version or leaves the octet unset goes red. The sentence binds the sender and no input can make it send another version; the negatives (header_test decode table #2, session_test TestBMPMalformedHeaderDrops) are the receiver refusing a version-2 header, the same rule read at the other end. The old decode-table positive stays tagged and proves only decode.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L37) | unit/verify | unproven |
| negative | [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_test.go#L77) | unit/verify | unproven |
| positive | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L18) | unit/verify | unproven |
| positive | [`TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L68) | unit/verify | revert, verified |

### [`RFC7854-x-2`](#rfc7854-x-2)

Version (1 byte): Indicates the BMP version. This is set to '3' for all messages defined in this specification. ('1' and '2' were used by draft versions of this document.) Version 0 is reserved and MUST NOT be sent. o Message Length (4 bytes): Length of the message in bytes (including headers, data, and encapsulated messages, if any). o Message Type (1 byte): This identifies the type of the BMP message. (§4.1, Common Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength reads the transmitted octets of all seven types: octet 0 == 3 (so never the reserved 0), octets 1-4 == the whole message computed independently from the case's own inputs (common header + per-peer header where present + data) and equal to the octets the writer produced, and octet 5 == the type. Each clause of the quoted definition is asserted on a sent message. The tagged negative (header_test, a 3-octet buffer refused) is thin, but the clauses are sender field definitions with no input to refuse; the positive discriminates every one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L30) | unit/verify | unproven |
| positive | [`TestBMPCommonHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L83) | unit/verify | unproven |
| positive | [`TestRFC7854EveryTransmittedMessageCarriesVersion3AndItsTotalLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L70) | unit/verify | revert, verified |

### [`RFC7854-x-3`](#rfc7854-x-3)

The per-peer header follows the common header for most BMP messages. (§4.2, Per-Peer Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. rfc7854_wire_header_test.go::TestRFC7854PerPeerHeaderFollowsTheCommonHeader writes all seven types and decodes a per-peer header at octet 6 of Route Monitoring, Statistics Report, Peer Down, Peer Up and Route Mirroring, requiring it to equal the header the writer was given (4-octet AS 4200000001); for Initiation and Termination it requires the first TLV (type 1, length 2) at octet 6 and a message too short to hold a per-peer header, which is the 'most' of the sentence. The older TestHasPeerHeader tags stay: hasPeerHeader has no non-test caller, so those two tags prove a helper table only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L287) | unit/verify | unproven |
| negative | [`TestRFC7854PerPeerHeaderFollowsTheCommonHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L285) | unit/verify | unproven |
| positive | [`TestRFC7854PerPeerHeaderFollowsTheCommonHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L106) | unit/verify | revert, verified |

### [`RFC7854-x-4`](#rfc7854-x-4)

If a 16-bit AS number is stored in this field [RFC6793], it should be padded with zeroes in the 16 most significant bits. (§4.2, Per-Peer Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. rfc7854_wire_header_test.go::TestRFC7854PeerASIsFourOctetsWithTheShortASZeroPadded writes a Peer Down (writePeerHeader) into a patterned buffer and reads per-peer octets 26-29 raw: 65001 must be 00 00 FD E9 (the 16 most significant bits zero) and 4200000001 must be FA 56 EA 01, so a 2-octet field, a lost padding or a misplaced AS goes red. {single-polarity: positive} holds: the field is a fixed 4-octet encode with no shorter variant to refuse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116) | unit/verify | unproven |
| positive | [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173) | unit/verify | unproven |
| positive | [`TestRFC7854PeerASIsFourOctetsWithTheShortASZeroPadded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_wire_header_test.go#L148) | unit/verify | revert, verified |

### [`RFC7854-x-5`](#rfc7854-x-5)

It is 4 bytes long if an IPv4 address is carried in this field (with the 12 most significant bytes zero-filled) and 16 bytes long if an IPv6 address is carried in this field. (§4.2, Per-Peer Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. IPv4 lands in octets 12..15 with 0..11 zero; the IPv6 negative keeps its full address

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L393) | unit/verify | unproven |
| positive | [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L379) | unit/verify | unproven |

### [`RFC7854-x-6`](#rfc7854-x-6)

An initiation message MUST be sent as the first message after the TCP session comes up. (§4.3, Initiation Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. collector reads the first message off a live TCP connection and asserts type Initiation; single-polarity marker

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderConnects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L19) | unit/verify | unproven |

### [`RFC7854-x-7`](#rfc7854-x-7)

The sysDescr and sysName Information TLVs MUST be sent, any others are optional. (§4.3, Initiation Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. live Initiation carries sysName equal to the system identity and a sysDescr; single-polarity marker

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderInitiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L84) | unit/verify | unproven |

### [`RFC7854-x-8`](#rfc7854-x-8)

Sent OPEN Message: The full OPEN message transmitted by the monitored router to its peer. o Received OPEN Message: The full OPEN message received by the monitored router from its peer. (§4.10, Peer Up Notification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Peer Up echoes both cached OPENs byte for byte (sender and event paths); with no cached OPENs no Peer Up is sent at all

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerup_openless_test.go#L40) | unit/verify | unproven |
| positive | [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L136) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L211) | unit/verify | unproven |

### [`RFC7854-x-9`](#rfc7854-x-9)

Following the common BMP header and per-peer header is a BGP Update PDU. (§4.6, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Route Monitoring wraps the body in a full RFC 4271 header of type UPDATE; single-polarity marker

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L279) | unit/verify | unproven |

### [`RFC7854-x-10`](#rfc7854-x-10)

Reason indicates why the session was closed. (§4.9, Peer Down Notification)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestBMPSenderPeerDown round-trips a Reason byte the caller chose (5), so it cannot fail on a wrong cause; TestHandleSenderStatePeerDown pins reason 2 ('no notification message was sent') for an event whose close reason is 'notification'. No unit ties a Reason to the cause that closed the session, and the producer peerDownFor (bmp_events.go) never emits reason 4, so an unexpected transport termination (which Section 4.9 assigns to reason 4) is reported as a local close

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L269) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L248) | unit/verify | unproven |

### [`RFC7854-x-11`](#rfc7854-x-11)

The router MAY send a Termination message prior to closing the session. (§3.3, Lifecycle of a BMP Session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29: the stale unit TestSenderStopSendsTerminationToCollector changed only by an added RFC7854-4.5-5 tag comment; body unchanged. It still requires the collector to read a Termination through stop() racing holdConnection, and the writeTerminationLocked break recorded under RFC7854-4.5-5 on this same unit was observed red. sender_test.go::TestBMPSenderTermination covers the encoder, no record. {single-polarity: positive} holds for a MAY that ze always exercises.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_queue_test.go#L561) | unit/verify | unproven |
| positive | [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_test.go#L363) | unit/verify | unproven |

### [`RFC7854-x-12`](#rfc7854-x-12)

No BMP message is ever sent from the monitoring station to the monitored router. (§3.2, Connection Establishment and Termination)

Audit verdict: enforced (the tests do what the requirement demands), fresh. receiver loop driven with Initiation and Termination writes zero bytes toward the router end; single-polarity marker

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_test.go#L135) | unit/verify | unproven |

### [`RFC7854-4.9-1`](#rfc7854-4.9-1)

Reason 1: The local system closed the session. Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that would have been sent to the peer. o Reason 2: The local system closed the session. No notification message was sent. Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session (see Section 8.1 of [RFC4271]). Two bytes both set to 0 are used to indicate that no relevant Event code is defined. o Reason 3: The remote system closed the session with a notification message. Following the Reason is a BGP PDU containing the BGP NOTIFICATION message as received from the peer. (§4.9, Peer Down Notification)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. reasons 1 and 3 carry the exact NOTIFICATION PDU and reason 5 carries no Data, but the reason 2 clause ('the code corresponding to the FSM Event that caused the system to close the session') is unproven: the producer writes the constant fsmEventNone for every reason 2, so the subtest's two-zero-octet assertion cannot fail on a wrong event code, and the subtest drives 'connection lost', an unexpected transport termination that Section 4.9 assigns to reason 4, not reason 2

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC7854PeerDownCarriesTheDataItsReasonRequires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_peerdown_data_test.go#L88) | unit/verify | unproven |

### [`RFC7854-4.5-2`](#rfc7854-4.5-2)

Likewise, the monitoring station MUST close the TCP session after receiving a termination message. (§4.5, Termination Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. receiver closes after reading a Termination with the router end held open (well under the read deadline) and drops the router; without a Termination two valid messages leave the session open

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_absent_test.go#L9) | unit/verify | unproven |
| positive | [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_close_test.go#L9) | unit/verify | unproven |

### [`RFC7854-x-15`](#rfc7854-x-15)

It MAY periodically send Stats Reports or even new Initiation messages, according to configuration. (§3.3, Lifecycle of a BMP Session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. statistics-timeout 1 through the reload rail yields two reports about one interval apart; statistics-timeout 0 yields none

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854StatisticsTimeoutZeroSendsNoReport`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L475) | unit/verify | revert, verified |
| positive | [`TestRFC7854StatisticsTimeoutSendsPeriodicReports`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L422) | unit/verify | revert, verified |

### [`RFC7854-x-16`](#rfc7854-x-16)

It subsequently sends a Peer Up message over the BMP session for each of its monitored BGP peers that is in the Established state. (§3.3, Lifecycle of a BMP Session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. The wrong tags on TestConcurrentDumpsStayAddressedToTheirOwnCollector are gone. rfc7854_session_opening_test.go::TestRFC7854SessionPeerUpOnlyForEstablishedPeers primes a new collector (primeSender) over one Established peer and one peer that exchanged OPENs and sent an UPDATE but never reported Up; it counts Peer Ups per address on the collector's bytes: exactly 1 for the Established peer, 0 for the other, and the first message is the Peer Up. A producer that announced every peer with cached OPENs, or none, goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SessionPeerUpOnlyForEstablishedPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L97) | unit/verify | revert, verified |
| positive | [`TestRFC7854SessionPeerUpOnlyForEstablishedPeers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L95) | unit/verify | revert, verified |

### [`RFC7854-x-17`](#rfc7854-x-17)

It subsequently sends a Peer Up message over the BMP session for each of its monitored BGP peers that is in the Established state. It follows by sending the contents of its Adj- RIBs-In (pre-policy, post-policy, or both, see Section 5) encapsulated in Route Monitoring messages. (§3.3, Lifecycle of a BMP Session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. rfc7854_session_opening_test.go::TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp reads the same two-peer session in order: no Route Monitoring before the Peer Up, the Established peer's received route (exact UPDATE body announceBody(1)) arrives as a pre-policy Route Monitoring (L clear, the default pre-policy selection), and nothing is dumped for the peer that never reached Established. Order, content and scope of the Adj-RIB-In dump are each asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestRFC7854SessionDumpsAdjRIBInAfterThePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L128) | unit/verify | revert, verified |

### [`RFC7854-x-18`](#rfc7854-x-18)

The sysDescr and sysName Information TLVs MUST be sent, any others are optional. (§4.3, Initiation Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. Both clauses now hold both polarities. Sender: rfc7854_test.go TestRFC7854InitiationCarriesSysDescr and the new rfc7854_session_opening_test.go::TestRFC7854InitiationCarriesSysName decode the Initiation sendInitiation writes and require exactly one sysName (type 2) and a sysDescr (type 1). Receiver: TestRFC7854InitiationMissingSysDescrEndsSession and TestBMPInitiationMissingSysNameEndsSession (tag added) each send an Initiation missing one TLV and require the session closed (decodeInitiation checks each independently); TestRFC7854InitiationEmptyIdentityValuesAccepted keeps presence, not length, as the test.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPInitiationMissingSysNameEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestRFC7854InitiationMissingSysDescrEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L15) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationEmptyIdentityValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L40) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationCarriesSysName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_session_opening_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC7854InitiationCarriesSysDescr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L29) | unit/verify | revert, verified |

### [`RFC7854-3.2-1`](#rfc7854-3.2-1)

Retries MUST be subject to some variety of backoff. (§3.2, Connection Establishment and Termination)

Audit verdict: enforced (the tests do what the requirement demands), fresh. nextReconnectWait doubles and caps at reconnectMax; a refused dial is not retried before the base wait even when the port reopens early

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854RetryNotBeforeBackoff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC7854RetryBackoffDoubles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L64) | unit/verify | revert, verified |

### [`RFC7854-3.2-2`](#rfc7854-3.2-2)

The router MUST also restrict the rate at which sessions may be established. (§3.2, Connection Establishment and Termination)

Audit verdict: enforced (the tests do what the requirement demands), fresh. after the collector closes a session the sender reconnects, and not before the base wait since the first session

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L173) | unit/verify | revert, verified |

### [`RFC7854-3.3-1`](#rfc7854-3.3-1)

Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer (§3.3, Lifecycle of a BMP Session, and restated in the Route Monitoring section)

Audit verdict: enforced (the tests do what the requirement demands), fresh. after reconnect Peer Up precedes the surviving route and each family gets EOR after its routes; an incremental UPDATE and another collector's reconnect produce no EOR, and an incomplete snapshot never emits EOR

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854AdjReplayCompletionIsSessionScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L95) | unit/verify | unproven |
| negative | [`TestRFC7854IncompleteAdjReplayCannotClaimCompletion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC7854AdjReplayEndsEachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L67) | unit/verify | unproven |

### [`RFC7854-4.1-1`](#rfc7854-4.1-1)

A BMP implementation MUST ignore unrecognized message types upon receipt. (§4.1, Common Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. type 200 is skipped and the following Termination is consumed; a malformed Peer Up still ends the session

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854MalformedKnownTypeStillEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestRFC7854UnrecognizedMessageTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L252) | unit/verify | revert, verified |

### [`RFC7854-4.2-1`](#rfc7854-4.2-1)

The remaining bits are reserved for future use. They MUST be transmitted as 0 and their values MUST be ignored on receipt. (§4.2, Per-Peer Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. transmitted Route Monitoring on both directions has reserved bits 0; a Peer Up with every reserved bit set is accepted and its defined flags decode unchanged

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854ReservedFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L357) | unit/verify | revert, verified |
| positive | [`TestRFC7854ReservedFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L337) | unit/verify | revert, verified |

### [`RFC7854-4.3-2`](#rfc7854-4.3-2)

The initiation message consists of the common BMP header followed by two or more Information TLVs (Section 4.4) containing information about the monitored router. (§4.3, Initiation Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. Positive rfc7854_message_layout_test.go::TestRFC7854InitiationIsTheCommonHeaderThenTwoOrMoreTLVs reads the octets sender.go::sendInitiation writes off a pipe, checks type 4 and walks TLVs by hand from octet 6 to exactly the end, requiring >= 2. Negative (R1(a)) ::TestRFC7854InitiationWithFewerThanTwoTLVsIsRefused: a one-TLV Initiation is refused by the receiver. The refusal comes from decodeInitiation's sysName-presence check (RFC7854-x-18), not a count check, but every one-TLV Initiation lacks sysName or sysDescr, so no one-TLV input is accepted; the buffer is otherwise valid.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationWithFewerThanTwoTLVsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC7854InitiationIsTheCommonHeaderThenTwoOrMoreTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L70) | unit/verify | revert, verified |

### [`RFC7854-4.4-1`](#rfc7854-4.4-1)

If multiple strings are included, their ordering MUST be preserved when they are reported. (§4.4, Information TLV)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Initiation and Peer Up strings reach the command output in transmitted order, with duplicates kept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L32) | unit/verify | unproven |
| negative | [`TestRFC7854PeerUpStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L55) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L21) | unit/verify | unproven |
| positive | [`TestRFC7854PeerUpStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L44) | unit/verify | unproven |

### [`RFC7854-4.4-2`](#rfc7854-4.4-2)

Type = 1: sysDescr. The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysDescr MIB-II [RFC1213] object. (§4.4, Information TLV)

Audit verdict: enforced (the tests do what the requirement demands), fresh. sysDescr carries the running build and uname sysname, release and machine, the MIB-II sysDescr content; it changes when the build identity changes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationDescriptionChangesWithBuild`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L227) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationDescribesRunningSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L205) | unit/verify | unproven |

### [`RFC7854-4.4-3`](#rfc7854-4.4-3)

Type = 2: sysName. The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysName MIB-II [RFC1213] object. (§4.4, Information TLV)

Audit verdict: enforced (the tests do what the requirement demands), fresh. sysName is the configured host.domain FQDN, the MIB-II sysName convention, survives rollback, and follows a configuration change

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationSystemNameChanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationUsesConfiguredSystemName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L168) | unit/verify | unproven |

### [`RFC7854-4.5-1`](#rfc7854-4.5-1)

Once the router has sent a termination message, it MUST close the TCP session without sending any further messages. (§4.5, Termination Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. after stop() the collector reads the Termination then EOF; a Peer Up afterwards is refused with errNotConnected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L384) | unit/verify | revert, verified |

### [`RFC7854-4.5-3`](#rfc7854-4.5-3)

Type = 1: Reason. The Information field contains a 2-byte code indicating the reason that the connection was terminated. Some reasons may have further TLVs associated with them. Inclusion of this TLV is REQUIRED. (§4.5, Termination Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. live shutdown Termination carries exactly one two-octet Reason; absent, empty, short and long Reason TLVs fail decode and end the session

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TerminationRequiresTwoByteReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L86) | unit/verify | unproven |
| positive | [`TestRFC7854SenderTerminationCarriesReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L54) | unit/verify | unproven |

### [`RFC7854-4.5-4`](#rfc7854-4.5-4)

The termination message consists of the common BMP header followed by one or more TLVs containing information about the reason for the termination (§4.5, Termination Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. Positive rfc7854_message_layout_test.go::TestRFC7854TerminationIsTheCommonHeaderThenTLVs reads the octets the sender's Termination path writes, walks TLVs from octet 6 to exactly the end, requires at least one and exactly one Reason TLV. Negative ::TestRFC7854TerminationWithNoTLVIsRefused: a 6-octet header-only Termination is refused (decodeTermination's missing-Reason branch, which any zero-TLV body reaches).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TerminationWithNoTLVIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC7854TerminationIsTheCommonHeaderThenTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L113) | unit/verify | revert, verified |

### [`RFC7854-4.5-5`](#rfc7854-4.5-5)

Although use of this message is RECOMMENDED, a monitoring station must always be prepared for the session to terminate with no message. (§4.5, Termination Message)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Station clause: termination_absent_test.go::TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination sends Initiation + Peer Up, confirms one router and one peer held, closes the pipe with no Termination, and requires handleSession to return with zero routers and zero peers (record: removeRouter break observed red). RECOMMENDED clause: sender_queue_test.go::TestSenderStopSendsTerminationToCollector reads a Termination on the collector side through stop() racing holdConnection (record: writeTerminationLocked break observed red). {single-polarity: positive} holds: the sentence permits the no-message close, so no non-compliant router input exists for the station to refuse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_sender_queue_test.go#L568) | unit/verify | revert, verified |
| positive | [`TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_termination_absent_test.go#L74) | unit/verify | revert, verified |

### [`RFC7854-4.7-1`](#rfc7854-4.7-1)

If the BGP Message TLV occurs in the Route Mirroring message, it MUST occur last in the list of TLVs. (§4.7, Route Mirroring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. transmitted Route Mirroring ends with the BGP Message TLV; Information then BGP Message is accepted, BGP Message then Information ends the session

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854MirrorBGPMessageNotLastEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L134) | unit/verify | unproven |
| positive | [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L120) | unit/verify | unproven |
| positive | [`TestRFC7854RouteMirroringBGPMessageTLVLast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L421) | unit/verify | revert, verified |

### [`RFC7854-4.7-2`](#rfc7854-4.7-2)

Code = 0: Errored PDU. The contained message was found to have some error that made it unusable, causing it to be treated-as- withdraw [RFC7606]. A BGP Message TLV MUST also occur in the TLV list. (§4.7, Route Mirroring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Errored PDU with a BGP Message TLV is accepted; Errored PDU without one ends the session

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854ErroredMirrorWithoutPDUEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L146) | unit/verify | unproven |
| positive | [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L121) | unit/verify | unproven |

### [`RFC7854-4.7-3`](#rfc7854-4.7-3)

Following the common BMP header and per-peer header is a set of TLVs that contain information about a message or set of messages. (§4.7, Route Mirroring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. Positive rfc7854_message_layout_test.go::TestRFC7854RouteMirroringIsThePerPeerHeaderThenTLVs: writeRouteMirroring output decodes a per-peer header equal to the input at octet 6, then TLVs walked by hand from octet 48 to exactly the end, same types and values in order. Negative ::TestRFC7854RouteMirroringWhoseBodyIsNotTLVsIsRefused: the valid message is first accepted, then a 3-octet tail that is no TLV (length fixed up) is refused, so only the not-TLVs body differs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854RouteMirroringWhoseBodyIsNotTLVsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L209) | unit/verify | revert, verified |
| positive | [`TestRFC7854RouteMirroringIsThePerPeerHeaderThenTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L176) | unit/verify | revert, verified |

### [`RFC7854-4.8-1`](#rfc7854-4.8-1)

A BMP implementation MUST ignore unrecognized stat types on receipt, and likewise MUST ignore unexpected data in the Stat Data field. (§4.8, Stats Reports)

Audit verdict: enforced (the tests do what the requirement demands), fresh. unknown stat type 60000 and a 3-octet counter are consumed and the session survives; a stat length past the report end still ends it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TruncatedStatEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestRFC7854UnrecognizedStatTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L278) | unit/verify | revert, verified |

### [`RFC7854-4.8-2`](#rfc7854-4.8-2)

However, if an SR message is transmitted, at least one statistic MUST be carried in it. (§4.8, Stats Reports)

Audit verdict: enforced (the tests do what the requirement demands), fresh. a transmitted report carries one statistic; an empty report is refused before it reaches the collector

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SenderRefusesEmptyStatistics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L175) | unit/verify | unproven |
| positive | [`TestRFC7854StatsReportCarriesAStatistic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L451) | unit/verify | revert, verified |

### [`RFC7854-4.8-3`](#rfc7854-4.8-3)

Following the common BMP header and per-peer header is a 4-byte field that indicates the number of counters in the stats message where each counter is encoded as a TLV. (§4.8, Stats Reports)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. Positive rfc7854_message_layout_test.go::TestRFC7854StatsReportCountNamesTheCountersThatFollow: the 4-octet count at octet 48 equals 3 and exactly three counter TLVs, walked by hand, run to the end with the input types and values. Negative ::TestRFC7854StatsReportCountDisagreeingWithItsCountersIsRefused: the valid report is first accepted, then count+1 (truncated entry) and count-1 (trailing data) are each refused by decodeStatisticsReport.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854StatsReportCountDisagreeingWithItsCountersIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC7854StatsReportCountNamesTheCountersThatFollow`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L238) | unit/verify | revert, verified |

### [`RFC7854-4.10-1`](#rfc7854-4.10-1)

Following the common BMP header and per-peer header is the following (§4.10, Peer Up Notification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29. The row's sentence is the lead-in to the §4.10 figure, the only statement of the Peer Up layout, so the obligation judged is that figure: Local Address (16), Local Port, Remote Port, Sent OPEN, Received OPEN after the per-peer header (Information TLVs are RFC7854-x-19, the OPEN contents RFC7854-x-8). Positive rfc7854_message_layout_test.go::TestRFC7854PeerUpIsThePerPeerHeaderThenTheFixedFieldsAndBothOPENs reads each field raw at its offset from writePeerUp output and the exact total length. Negative ::TestRFC7854PeerUpShorterThanItsFixedFieldsIsRefused: a valid Peer Up is accepted, the same cut 10 octets into the fixed fields is refused (decodePeerUp short check).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854PeerUpShorterThanItsFixedFieldsIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L339) | unit/verify | revert, verified |
| positive | [`TestRFC7854PeerUpIsThePerPeerHeaderThenTheFixedFieldsAndBothOPENs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_message_layout_test.go#L294) | unit/verify | revert, verified |

### [`RFC7854-5-1`](#rfc7854-5-1)

Pre-policy routes MUST have their L flag clear in the BMP header (see Section 4), post-policy routes MUST have their L flag set. (§5, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (the negative unit gained the RFC7854-5-4 tag line only; assertions unchanged). rfc7854_test.go::TestRFC7854LFlagFollowsPolicy: received announce and withdraw leave with L clear, sent with L set. ::TestRFC7854LFlagNotDecidedByBody: one body streamed on both sides leaves L clear received and L set sent, so the flag follows the direction (policy stage) and not the message. Ze sends no post-policy Adj-RIB-In; its L-set stream is the RFC 8671 Adj-RIB-Out (O and L set, peerHeaderFromEvent).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L503) | unit/verify | revert, verified |
| positive | [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L478) | unit/verify | revert, verified |

### [`RFC7854-5-2`](#rfc7854-5-2)

If the implementation is able to provide information about when routes were received, it MAY provide such information in the BMP Timestamp field. Otherwise, the BMP Timestamp field MUST be set to 0, indicating that time is not available. (§5, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. replayed routes and EORs with unknown receipt time carry 0.0; a known receipt time survives replay exactly

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854KnownRouteTimeSurvivesReplay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC7854UnknownRouteTimeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L122) | unit/verify | unproven |

### [`RFC7854-5-4`](#rfc7854-5-4)

The withdraw MUST have its L flag set to correspond to that of any previous announcement (§5, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. First verdict 2026-09-29 (first clause of the §5 sentence; the twice-sent-withdraw clause left with retired RFC7854-5-3). Producer peerHeaderFromEvent sets L only on the sent direction, so a route's announcement and its withdraw take their L from the same direction. Positive rfc7854_test.go::TestRFC7854LFlagFollowsPolicy: a received withdraw carries L clear like a received announce, a sent withdraw L set like a sent announce. Negative ::TestRFC7854LFlagNotDecidedByBody: the same withdraw body on both sides gets L clear and L set, so a withdraw cannot carry the other side's flag. Both observed red under the peerHeaderFromEvent break. The test does not pair one announce with its own withdraw by prefix; the direction is the only input L reads, so no pairing can diverge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L504) | unit/verify | revert, verified |
| positive | [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L479) | unit/verify | revert, verified |

### [`RFC7854-5-6`](#rfc7854-5-6)

When a change occurs to a route, such as an attribute change, the router must update the monitoring station with the new attribute. (§5, Route Monitoring)

Audit verdict: enforced (the tests do what the requirement demands), fresh. rfc7854_replay_test.go::TestRFC7854ChangedAttributesCanReturnToEarlierValue receives one prefix three times with ORIGIN IGP, INCOMPLETE, IGP and requires exactly three Route Monitoring messages, each body byte-equal to the UPDATE carrying that ORIGIN, so a sender that suppresses or dedupes the return to an earlier value fails (record: handleSenderUpdate break observed red). One attribute (ORIGIN) samples 'such as an attribute change'. {single-polarity: positive} holds: every route change is compliant input to the router, which has no refusal path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7854ChangedAttributesCanReturnToEarlierValue`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L247) | unit/verify | revert, verified |

### [`RFC7854-8.2-1`](#rfc7854-8.2-1)

Since in this case no transport session actually exists, the Local and Remote Port fields of the Peer Up message MUST be set to 0. (§8.2, Peer Up Notification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Loc-RIB instance Peer Up carries ports 0/0; a BGP peer's Peer Up carries its real 179/40000

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854BGPPeerUpCarriesTransportPorts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L542) | unit/verify | revert, verified |
| positive | [`TestRFC7854LocRIBPeerUpPortsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L528) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-29 |
| Register | prose |
| Source | rfc/full/rfc7854.txt |
| Source fingerprint | b32c6bf7d6b5d67a |
| Record | rfc/extraction/rfc7854.json |
| Mapped sentences | 24 |
| Declined as scope | 15 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 2 | walked | not stated |
| `3.3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 2 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 2 | walked | not stated |
| `4.4` | not stated | 3 | walked | not stated |
| `4.5` | not stated | 4 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 2 | walked | not stated |
| `4.8` | not stated | 3 | walked | not stated |
| `4.9` | not stated | 0 | walked | not stated |
| `4.10` | not stated | 0 | walked | not stated |
| `5` | not stated | 6 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 2 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 1 | walked | not stated |
| `10.2` | not stated | 1 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `10.4` | not stated | 1 | walked | not stated |
| `10.5` | not stated | 1 | walked | not stated |
| `10.6` | not stated | 1 | walked | not stated |
| `10.7` | not stated | 1 | walked | not stated |
| `10.8` | not stated | 1 | walked | not stated |
| `10.9` | not stated | 1 | walked | not stated |
| `10.10` | not stated | 1 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Front-matter boilerplate the extractor did not strip: the IETF Trust Legal Provisions notice binds whoever extracts Code Components from the RFC document, not a BMP router or monitoring station. The lowercase must is the license condition of the copyright notice. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `3.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the obligation section 4.3 states at the site this walk maps to RFC7854-x-6: 'An initiation message MUST be sent as the first message after the TCP session comes up.' Section 3.3 states it as the first step of the session lifecycle; section 4.3 states it as the rule of the message itself. | It MUST begin by sending an Initiation message. |
| `4.8:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence is the 64-bit Gauge definition that section 4.8 adopts from Section 4 of RFC 2856 (the section says the stats types are defined after the examples in Section 3.2.3.3 of RFC 1155 and Section 4 of RFC 2856). The lowercase shall describes the semantics of that data type as RFC 2856 defines it; section 4.8 states its own obligations on stats reports at the sites this walk maps to RFC7854-4.8-1 and RFC7854-4.8-2. | 64-bit Gauge: A non-negative integer that may increase or decrease, but shall never exceed a maximum value, nor fall below a minimum one. |
| `5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the per-peer End-of-RIB obligation section 3.3 states at the site this walk maps to RFC7854-3.3-1: 'Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer'. Section 5 adds only where the marker is defined (Section 2 of RFC 4724) and that it carries the BMP encapsulation header. | When the initial dump is completed for a given peer, this MUST be indicated by sending an End-of-RIB marker for that peer (as specified in Section 2 of [RFC4724], plus the BMP encapsulation header). |
| `5:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The prose scanner matched the ordinary English required in (Route mirroring (Section 6) is provided if this is required.), which describes when an operator uses Route Mirroring, and the lowercase should attempt to generate updates promptly, which the same paragraph relaxes: it is acceptable that the router generate the final state of that prefix. Neither states an obligation on the router; RFC 7854 invokes RFC 2119 for the capitalized keywords only. | (Route mirroring (Section 6) is provided if this is required.) While the router should attempt to generate updates promptly, there is a finite time that could elapse between reception of an update, the generation an RM message, and its transmission to the monitoring station. |
| `8.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The prose scanner matched the ordinary English Some consideration is required, which introduces section 8.2 and states no obligation. The section obligation on locally originated routes is the next site, mapped to RFC7854-8.2-1. | Some consideration is required for routes originated into BGP by the local router, whether as a result of redistribution from another protocol or for some other reason. |
| `10.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Type values 0 through 127 MUST be assigned using the "Standards Action" policy, and values 128 through 250 using the "Specification Required" policy defined in [RFC5226]. |
| `10.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Peer Type values 0 through 127 MUST be assigned using the "Standards Action" policy, and values 128 through 250 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Stat Type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.7:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.9:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |
| `10.10:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds IANA, the registry the section addresses: the sentence sets the allocation policy ('Standards Action', 'Specification Required' under RFC 5226) IANA applies when it assigns a value from this registry. Ze holds no code that acts as IANA and could hold none: the role is the registry itself, not a party on the BMP wire. Ze's BMP encoder and decoder (internal/component/bgp/plugins/bmp) read and write the values the registry already holds; nothing in the repository allocates one. | Information type values 0 through 32767 MUST be assigned using the "Standards Action" policy, and values 32768 through 65530 using the "Specification Required" policy, defined in [RFC5226]. |

## Superseded

No document obsoletes RFC 7854, so its obligations are stated where they were written.
