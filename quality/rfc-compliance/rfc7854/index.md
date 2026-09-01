# RFC 7854 - BGP Monitoring Protocol (BMP)

Partial. Every requirement this repository extracted from RFC 7854, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 46.2% | 6 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 53.8% | 7 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 27 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 21 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 21 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 27 |
| Tagged units | 27 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7854.md` |
| Requirement shard | `rfc/requirements/rfc7854.md` |
| RFC text | `rfc/full/rfc7854.txt` |

## Enrolment

Enrolled: BGP Monitoring Protocol (BMP) base: twelve MUST-level requirements, all met in internal/component/bgp/plugins/bmp. Five carry positive+negative tags: x-1 (the common-header version is 3, other versions rejected), x-2 (the common header is 6 octets), x-3 (the per-peer header is present for the peer-scoped message types), x-5 (an IPv4 peer address is IPv4-mapped in the 16-octet field), and x-8 (a Peer Up carries the sent and received OPENs). Seven are {single-polarity: positive}: x-4 (the Peer AS is 4 octets), x-6 (Initiation is sent first), x-7 (Initiation carries the sysName TLV), x-9 (Route Monitoring wraps a BGP UPDATE PDU), x-10 (Peer Down carries a Reason), x-11 (Termination is sent on shutdown), and x-12 (monitoring is unidirectional: the BMP receiver writes nothing back over a valid session, proven by a new net.Pipe test).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- BMP receiver and sender, peer lifecycle, route monitoring messages, CLI and config. Each collector connection is a complete BMP session: Initiation first, then a Peer Up for every BGP peer that is already established, and a Termination before the session is closed (sent by the teardown path that closes the socket, so a collector actually receives it). Messages reach the socket through a per-session byte-bounded transmit queue drained by that session's own goroutine, so a collector that stops reading never blocks BGP
- when the bound is reached the session is reset with a bare TCP close and no Termination.


**What the ledger says remains:**

Loc-RIB route monitoring is provided under RFC 9069.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC7854-x-1`](#rfc7854-x-1), [`RFC7854-x-2`](#rfc7854-x-2), [`RFC7854-x-3`](#rfc7854-x-3), [`RFC7854-x-5`](#rfc7854-x-5), [`RFC7854-x-8`](#rfc7854-x-8), [`RFC7854-4.5-2`](#rfc7854-4.5-2)

**Annotated instead of tested (7):** [`RFC7854-x-4`](#rfc7854-x-4), [`RFC7854-x-6`](#rfc7854-x-6), [`RFC7854-x-7`](#rfc7854-x-7), [`RFC7854-x-9`](#rfc7854-x-9), [`RFC7854-x-10`](#rfc7854-x-10), [`RFC7854-x-11`](#rfc7854-x-11), [`RFC7854-x-12`](#rfc7854-x-12)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7854-x-1` | Common Header Version field must be 3 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L18). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L37). **negative:** `unit/verify` [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L73) |
| `RFC7854-x-2` | Common Header must be 6 bytes: Version (1) + Message Length (4) + Message Type (1) (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPCommonHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L83). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L30) |
| `RFC7854-x-3` | Per-Peer Header must be present for message types 0-3 and 6 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L312). **negative:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L314) |
| `RFC7854-x-4` | Peer AS field must always be encoded as 4-byte AS number (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116). **positive:** `unit/verify` [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173). **negative:** no negative test. **{single-polarity}:** the Peer AS is unconditionally a 4-octet field on every encode and decode, so there is no shorter-AS variant to reject and no negative case to construct |
| `RFC7854-x-5` | IPv4 Peer Address must be encoded as IPv4-mapped IPv6 (::ffff:x.x.x.x) in 16-byte field (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L352). **negative:** `unit/verify` [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L367) |
| `RFC7854-x-6` | Initiation message must be sent immediately after TCP connection establishment (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderConnects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L19). **negative:** no negative test. **{single-polarity}:** the sender always emits Initiation as the first message on a fresh connection, so there is no valid session in which another message precedes it to reject |
| `RFC7854-x-7` | Initiation message must include sysName TLV (type 2) (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderInitiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L84). **negative:** no negative test. **{single-polarity}:** the Initiation the sender builds always includes the sysName TLV, so there is no valid Initiation omitting it to assert against |
| `RFC7854-x-8` | Peer Up message must include both sent and received OPEN messages (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L207). **positive:** `unit/verify` [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L127). **negative:** `unit/verify` [`TestBMPPeerUpSkippedOnCacheMiss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L260). **negative:** `unit/verify` [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerup_openless_test.go#L47) |
| `RFC7854-x-9` | Route Monitoring messages must contain a BGP UPDATE message (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L275). **negative:** no negative test. **{single-polarity}:** Route Monitoring is only ever constructed around a complete BGP UPDATE PDU, so there is no valid Route Monitoring lacking one to reject |
| `RFC7854-x-10` | Peer Down must include the reason code (1 byte) (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L244). **positive:** `unit/verify` [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L289). **negative:** no negative test. **{single-polarity}:** Peer Down is always written with a reason byte, so there is no valid Peer Down without one to assert against |
| `RFC7854-x-11` | Termination message must be sent when BMP session is being closed (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L359). **positive:** `unit/verify` [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_queue_test.go#L560). **negative:** no negative test. **{single-polarity}:** Termination is produced unconditionally when the session is torn down, so there is no valid shutdown that omits it to reject |
| `RFC7854-x-12` | BMP is unidirectional: router to collector only (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L130). **negative:** no negative test. **{single-polarity}:** the receiver loop (internal/component/bgp/plugins/bmp/bmp.go:441-492) issues only reads and the sender hold-loop (sender.go:207-237) reads only to detect close, so neither role writes toward the monitored router on a valid session and there is no reject case to construct |
| `RFC7854-4.5-2` | The monitoring station must close the TCP session after receiving a termination message (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_close_test.go#L9). **negative:** `unit/verify` [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_absent_test.go#L9) |
| `RFC7854-x-13` | Minimum 30 seconds between reconnection attempts (Reconnection) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-14` | Maximum 720 seconds between reconnection attempts with exponential backoff (Reconnection) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-15` | Statistics Reports should be sent periodically (Session Lifecycle) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-16` | Peer Up message should be sent for each established peer (Session Lifecycle) | SHOULD | x | **positive:** `unit/verify` [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L457). **negative:** no negative test |
| `RFC7854-x-17` | Initial RIB dump via Route Monitoring should follow Peer Up (Session Lifecycle) | SHOULD | x | **positive:** `unit/verify` [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L460). **negative:** no negative test |
| `RFC7854-x-18` | Initiation message may include sysDescr TLV (type 1) and free-form string TLV (type 0) (Message Types) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-19` | Peer Up message may include optional TLVs (Message Types) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-20` | Route Mirroring messages may be used to mirror BGP messages verbatim (Message Types) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7854 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7854-x-1`](#rfc7854-x-1)

Common Header Version field must be 3 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L37) | unit/verify | unproven |
| negative | [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L73) | unit/verify | unproven |
| positive | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L18) | unit/verify | unproven |

### [`RFC7854-x-2`](#rfc7854-x-2)

Common Header must be 6 bytes: Version (1) + Message Length (4) + Message Type (1) (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L30) | unit/verify | unproven |
| positive | [`TestBMPCommonHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L83) | unit/verify | unproven |

### [`RFC7854-x-3`](#rfc7854-x-3)

Per-Peer Header must be present for message types 0-3 and 6 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L314) | unit/verify | unproven |
| positive | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L312) | unit/verify | unproven |

### [`RFC7854-x-4`](#rfc7854-x-4)

Peer AS field must always be encoded as 4-byte AS number (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116) | unit/verify | unproven |
| positive | [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173) | unit/verify | unproven |

### [`RFC7854-x-5`](#rfc7854-x-5)

IPv4 Peer Address must be encoded as IPv4-mapped IPv6 (::ffff:x.x.x.x) in 16-byte field (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L367) | unit/verify | unproven |
| positive | [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L352) | unit/verify | unproven |

### [`RFC7854-x-6`](#rfc7854-x-6)

Initiation message must be sent immediately after TCP connection establishment (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderConnects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L19) | unit/verify | unproven |

### [`RFC7854-x-7`](#rfc7854-x-7)

Initiation message must include sysName TLV (type 2) (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderInitiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L84) | unit/verify | unproven |

### [`RFC7854-x-8`](#rfc7854-x-8)

Peer Up message must include both sent and received OPEN messages (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPPeerUpSkippedOnCacheMiss`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L260) | unit/verify | unproven |
| negative | [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerup_openless_test.go#L47) | unit/verify | unproven |
| positive | [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L127) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L207) | unit/verify | unproven |

### [`RFC7854-x-9`](#rfc7854-x-9)

Route Monitoring messages must contain a BGP UPDATE message (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L275) | unit/verify | unproven |

### [`RFC7854-x-10`](#rfc7854-x-10)

Peer Down must include the reason code (1 byte) (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L289) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L244) | unit/verify | unproven |

### [`RFC7854-x-11`](#rfc7854-x-11)

Termination message must be sent when BMP session is being closed (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_queue_test.go#L560) | unit/verify | unproven |
| positive | [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L359) | unit/verify | unproven |

### [`RFC7854-x-12`](#rfc7854-x-12)

BMP is unidirectional: router to collector only (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L130) | unit/verify | unproven |

### [`RFC7854-4.5-2`](#rfc7854-4.5-2)

The monitoring station must close the TCP session after receiving a termination message (§4.5, Termination Message)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_absent_test.go#L9) | unit/verify | unproven |
| positive | [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_close_test.go#L9) | unit/verify | unproven |

### [`RFC7854-x-16`](#rfc7854-x-16)

Peer Up message should be sent for each established peer (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L457) | unit/verify | unproven |

### [`RFC7854-x-17`](#rfc7854-x-17)

Initial RIB dump via Route Monitoring should follow Peer Up (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L460) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 7854, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7854, so its obligations are stated where they were written.
