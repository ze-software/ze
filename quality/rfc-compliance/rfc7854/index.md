# RFC 7854 - BGP Monitoring Protocol (BMP)

Partial. Every requirement this repository extracted from RFC 7854, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 81.2% | 26 of 32 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 18.8% | 6 of 32 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 32 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 32 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 31.5% | 23 of 73 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 32 | of 41 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 32 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 32 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 32 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 32 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 32 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 41 |
| Gated MUST-level | 32 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 73 |
| Tagged units | 73 |
| Recorded audit verdicts | 0 |
| Discrimination records | 23 |
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


**What the ledger says remains:**

Focused execution, producer discrimination and third-party pmacct acceptance of the current changes remain unrun. Loc-RIB monitoring is defined under RFC 9069 and Adj-RIB-Out under RFC 8671.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 26 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **32** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (26):** [`RFC7854-x-1`](#rfc7854-x-1), [`RFC7854-x-2`](#rfc7854-x-2), [`RFC7854-x-3`](#rfc7854-x-3), [`RFC7854-x-5`](#rfc7854-x-5), [`RFC7854-x-8`](#rfc7854-x-8), [`RFC7854-4.9-1`](#rfc7854-4.9-1), [`RFC7854-4.5-2`](#rfc7854-4.5-2), [`RFC7854-x-18`](#rfc7854-x-18), [`RFC7854-3.2-1`](#rfc7854-3.2-1), [`RFC7854-3.2-2`](#rfc7854-3.2-2), [`RFC7854-3.3-1`](#rfc7854-3.3-1), [`RFC7854-4.1-1`](#rfc7854-4.1-1), [`RFC7854-4.2-1`](#rfc7854-4.2-1), [`RFC7854-4.4-1`](#rfc7854-4.4-1), [`RFC7854-4.4-2`](#rfc7854-4.4-2), [`RFC7854-4.4-3`](#rfc7854-4.4-3), [`RFC7854-4.5-1`](#rfc7854-4.5-1), [`RFC7854-4.5-3`](#rfc7854-4.5-3), [`RFC7854-4.7-1`](#rfc7854-4.7-1), [`RFC7854-4.7-2`](#rfc7854-4.7-2), [`RFC7854-4.8-1`](#rfc7854-4.8-1), [`RFC7854-4.8-2`](#rfc7854-4.8-2), [`RFC7854-5-1`](#rfc7854-5-1), [`RFC7854-5-2`](#rfc7854-5-2), [`RFC7854-5-3`](#rfc7854-5-3), [`RFC7854-8.2-1`](#rfc7854-8.2-1)

**Annotated instead of tested (6):** [`RFC7854-x-4`](#rfc7854-x-4), [`RFC7854-x-6`](#rfc7854-x-6), [`RFC7854-x-7`](#rfc7854-x-7), [`RFC7854-x-9`](#rfc7854-x-9), [`RFC7854-x-10`](#rfc7854-x-10), [`RFC7854-x-12`](#rfc7854-x-12)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7854-x-1` | Common Header Version field must be 3 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L18). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L37). **negative:** `unit/verify` [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L77) |
| `RFC7854-x-2` | Common Header must be 6 bytes: Version (1) + Message Length (4) + Message Type (1) (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPCommonHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L83). **negative:** `unit/verify` [`TestBMPCommonHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L30) |
| `RFC7854-x-3` | Per-Peer Header must be present for message types 0-3 and 6 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L285). **negative:** `unit/verify` [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L287) |
| `RFC7854-x-4` | Peer AS field must always be encoded as 4-byte AS number (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116). **positive:** `unit/verify` [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173). **negative:** no negative test. **{single-polarity}:** the Peer AS is unconditionally a 4-octet field on every encode and decode, so there is no shorter-AS variant to reject and no negative case to construct |
| `RFC7854-x-5` | IPv4 Peer Address occupies the least significant 4 bytes of the 16-byte field, "with the 12 most significant bytes zero-filled" (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L379). **negative:** `unit/verify` [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L393) |
| `RFC7854-x-6` | Initiation message must be sent immediately after TCP connection establishment (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderConnects`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L19). **negative:** no negative test. **{single-polarity}:** the sender always emits Initiation as the first message on a fresh connection, so there is no valid session in which another message precedes it to reject |
| `RFC7854-x-7` | Initiation message must include sysName TLV (type 2) (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderInitiation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L84). **negative:** no negative test. **{single-polarity}:** the Initiation the sender builds always includes the sysName TLV, so there is no valid Initiation omitting it to assert against |
| `RFC7854-x-8` | Peer Up message must include both sent and received OPEN messages (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L211). **positive:** `unit/verify` [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L136). **negative:** `unit/verify` [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerup_openless_test.go#L40) |
| `RFC7854-x-9` | Route Monitoring messages must contain a BGP UPDATE message (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L279). **negative:** no negative test. **{single-polarity}:** Route Monitoring is only ever constructed around a complete BGP UPDATE PDU, so there is no valid Route Monitoring lacking one to reject |
| `RFC7854-x-10` | Peer Down must include the reason code (1 byte) (Message Types) | MUST | x | **positive:** `unit/verify` [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L248). **positive:** `unit/verify` [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L269). **negative:** no negative test. **{single-polarity}:** Peer Down is always written with a reason byte, so there is no valid Peer Down without one to assert against |
| `RFC7854-x-11` | Termination message may be sent before the BMP session is closed: "The router MAY send a Termination message prior to closing the session." (Session Lifecycle) | MAY | x | **positive:** `unit/verify` [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L363). **positive:** `unit/verify` [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_queue_test.go#L561). **negative:** no negative test. **{single-polarity}:** Termination is produced unconditionally when the session is torn down, so there is no valid shutdown that omits it to reject |
| `RFC7854-x-12` | BMP is unidirectional: router to collector only (Session Lifecycle) | MUST | x | **positive:** `unit/verify` [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L135). **negative:** no negative test. **{single-polarity}:** the receiver loop (internal/component/bgp/plugins/bmp/bmp.go:441-492) issues only reads and the sender hold-loop (sender.go:207-237) reads only to detect close, so neither role writes toward the monitored router on a valid session and there is no reject case to construct |
| `RFC7854-4.9-1` | Peer Down must carry the Data field when the Reason is 1, 2 or 3: the BGP NOTIFICATION PDU for reason 1 and reason 3, and the 2-byte FSM event code for reason 2 (§4.9, Peer Down Notification) | MUST | 4.9 | **positive:** `unit/verify` [`TestRFC7854PeerDownCarriesTheDataItsReasonRequires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerdown_data_test.go#L88). **negative:** `unit/verify` [`TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerdown_data_test.go#L148) |
| `RFC7854-4.5-2` | Likewise, the monitoring station MUST close the TCP session after receiving a termination message. (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_close_test.go#L9). **negative:** `unit/verify` [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_absent_test.go#L9) |
| `RFC7854-x-13` | Minimum 30 seconds between reconnection attempts (Reconnection) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-14` | Maximum 720 seconds between reconnection attempts with exponential backoff (Reconnection) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-15` | Statistics Reports may be sent periodically: "It MAY periodically send Stats Reports or even new Initiation messages, according to configuration." The Stats Reports text imposes no timing either: "This specification does not impose any timing restrictions on when and on what event these reports have to be transmitted." (Session Lifecycle) | MAY | x | **positive:** `unit/verify` [`TestRFC7854StatisticsTimeoutSendsPeriodicReports`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L422). **negative:** `unit/verify` [`TestRFC7854StatisticsTimeoutZeroSendsNoReport`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L475) |
| `RFC7854-x-16` | Peer Up message should be sent for each established peer (Session Lifecycle) | SHOULD | x | **positive:** `unit/verify` [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L463). **negative:** no negative test |
| `RFC7854-x-17` | Initial RIB dump via Route Monitoring should follow Peer Up (Session Lifecycle) | SHOULD | x | **positive:** `unit/verify` [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L466). **negative:** no negative test |
| `RFC7854-x-18` | Initiation message must include the sysDescr TLV (type 1): "The sysDescr and sysName Information TLVs MUST be sent, any others are optional." (Message Types) | MUST | x | **positive:** `unit/verify` [`TestRFC7854InitiationCarriesSysDescr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L29). **positive:** `unit/verify` [`TestRFC7854InitiationEmptyIdentityValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L39). **negative:** `unit/verify` [`TestRFC7854InitiationMissingSysDescrEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L15) |
| `RFC7854-x-19` | Peer Up message may include optional TLVs (Message Types) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-x-20` | Route Mirroring messages may be used to mirror BGP messages verbatim (Message Types) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-3.2-1` | Retries of a failed connection must be subject to some variety of backoff: "Retries MUST be subject to some variety of backoff." (§3.2, Connection Establishment and Termination) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7854RetryBackoffDoubles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L64). **negative:** `unit/verify` [`TestRFC7854RetryNotBeforeBackoff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L114) |
| `RFC7854-3.2-2` | The router must restrict the rate at which BMP sessions may be established: "The router MUST also restrict the rate at which sessions may be established." (§3.2, Connection Establishment and Termination) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L173). **negative:** `unit/verify` [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L174) |
| `RFC7854-3.3-1` | Once all the routes for a given peer have been sent, an End-of-RIB message must be sent for that peer: "Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer" (§3.3, Lifecycle of a BMP Session, and restated in the Route Monitoring section) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFC7854AdjReplayEndsEachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L67). **negative:** `unit/verify` [`TestRFC7854AdjReplayCompletionIsSessionScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L95). **negative:** `unit/verify` [`TestRFC7854IncompleteAdjReplayCannotClaimCompletion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L198) |
| `RFC7854-4.1-1` | Unrecognized message types must be ignored on receipt: "A BMP implementation MUST ignore unrecognized message types upon receipt." (§4.1, Common Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC7854UnrecognizedMessageTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L252). **negative:** `unit/verify` [`TestRFC7854MalformedKnownTypeStillEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L261) |
| `RFC7854-4.2-1` | The reserved per-peer flag bits must be transmitted as 0 and their values ignored on receipt: "They MUST be transmitted as 0 and their values MUST be ignored on receipt." (§4.2, Per-Peer Header) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7854ReservedFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L337). **negative:** `unit/verify` [`TestRFC7854ReservedFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L357) |
| `RFC7854-4.3-1` | The string TLV may be included multiple times in an Initiation message: "The string TLV MAY be included multiple times." (§4.3, Initiation Message) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7854-4.4-1` | When multiple strings are included in a string TLV, their ordering must be preserved when they are reported: "If multiple strings are included, their ordering MUST be preserved when they are reported." (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L21). **positive:** `unit/verify` [`TestRFC7854PeerUpStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L44). **negative:** `unit/verify` [`TestRFC7854InitiationStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L32). **negative:** `unit/verify` [`TestRFC7854PeerUpStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L55) |
| `RFC7854-4.4-2` | The sysDescr TLV Information field must equal the MIB-II sysDescr object: "The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysDescr MIB-II [RFC1213] object." (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationDescribesRunningSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L205). **negative:** `unit/verify` [`TestRFC7854InitiationDescriptionChangesWithBuild`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L227) |
| `RFC7854-4.4-3` | The sysName TLV Information field must equal the MIB-II sysName object: "The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysName MIB-II [RFC1213] object." (§4.4, Information TLV) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC7854InitiationUsesConfiguredSystemName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L168). **negative:** `unit/verify` [`TestRFC7854InitiationSystemNameChanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L188) |
| `RFC7854-4.5-1` | After sending a termination message the router must close the TCP session and send no further messages: "Once the router has sent a termination message, it MUST close the TCP session without sending any further messages." (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L384). **negative:** `unit/verify` [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L385) |
| `RFC7854-4.5-3` | A Termination message must carry the Reason TLV (type 1): "Inclusion of this TLV is REQUIRED." (§4.5, Termination Message) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC7854SenderTerminationCarriesReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L53). **negative:** `unit/verify` [`TestRFC7854TerminationRequiresTwoByteReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L85) |
| `RFC7854-4.7-1` | A BGP Message TLV in a Route Mirroring message must occur last in the list of TLVs: "If the BGP Message TLV occurs in the Route Mirroring message, it MUST occur last in the list of TLVs." (§4.7, Route Mirroring) | MUST | 4.7 | **positive:** `unit/verify` [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L119). **positive:** `unit/verify` [`TestRFC7854RouteMirroringBGPMessageTLVLast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L421). **negative:** `unit/verify` [`TestRFC7854MirrorBGPMessageNotLastEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L133) |
| `RFC7854-4.7-2` | A Route Mirroring message carrying Information code 0 (Errored PDU) must also carry a BGP Message TLV: "A BGP Message TLV MUST also occur in the TLV list." (§4.7, Route Mirroring) | MUST | 4.7 | **positive:** `unit/verify` [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L120). **negative:** `unit/verify` [`TestRFC7854ErroredMirrorWithoutPDUEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L145) |
| `RFC7854-4.8-1` | Unrecognized stat types and unexpected Stat Data must be ignored on receipt: "A BMP implementation MUST ignore unrecognized stat types on receipt, and likewise MUST ignore unexpected data in the Stat Data field." (§4.8, Stats Reports) | MUST | 4.8 | **positive:** `unit/verify` [`TestRFC7854UnrecognizedStatTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L278). **negative:** `unit/verify` [`TestRFC7854TruncatedStatEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L290) |
| `RFC7854-4.8-2` | A transmitted Stats Report must carry at least one statistic: "However, if an SR message is transmitted, at least one statistic MUST be carried in it." (§4.8, Stats Reports) | MUST | 4.8 | **positive:** `unit/verify` [`TestRFC7854StatsReportCarriesAStatistic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L451). **negative:** `unit/verify` [`TestRFC7854SenderRefusesEmptyStatistics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L174) |
| `RFC7854-5-1` | Pre-policy routes must have the L flag clear and post-policy routes must have it set: "Pre-policy routes MUST have their L flag clear in the BMP header (see Section 4), post-policy routes MUST have their L flag set." (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L478). **negative:** `unit/verify` [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L503) |
| `RFC7854-5-2` | Where the time a route was installed is not available, the BMP Timestamp field must be set to 0: "Otherwise, the BMP Timestamp field MUST be set to 0, indicating that time is not available." (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854UnknownRouteTimeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L122). **negative:** `unit/verify` [`TestRFC7854KnownRouteTimeSurvivesReplay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L142) |
| `RFC7854-5-3` | A withdraw must carry the L flag of the announcement it withdraws, and must be sent twice where the route was announced both pre-policy and post-policy: "The withdraw MUST have its L flag set to correspond to that of any previous announcement; if the route in question was previously announced with L flag both clear and set, the withdraw MUST similarly be sent twice, with L flag clear and set." (§5, Route Monitoring) | MUST | 5 | **positive:** `unit/verify` [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L479). **negative:** `unit/verify` [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L504) |
| `RFC7854-8.2-1` | Where no transport session exists, the Local Port and Remote Port fields of a Peer Up message must be set to 0: "Since in this case no transport session actually exists, the Local and Remote Port fields of the Peer Up message MUST be set to 0." (§8.2, Peer Up Notification) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC7854LocRIBPeerUpPortsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L528). **negative:** `unit/verify` [`TestRFC7854BGPPeerUpCarriesTransportPorts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L542) |

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
| negative | [`TestBMPMalformedHeaderDrops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L77) | unit/verify | unproven |
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
| negative | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L287) | unit/verify | unproven |
| positive | [`TestHasPeerHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L285) | unit/verify | unproven |

### [`RFC7854-x-4`](#rfc7854-x-4)

Peer AS field must always be encoded as 4-byte AS number (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPPeerHeaderDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L116) | unit/verify | unproven |
| positive | [`TestBMPPeerHeaderEncode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/header_test.go#L173) | unit/verify | unproven |

### [`RFC7854-x-5`](#rfc7854-x-5)

IPv4 Peer Address occupies the least significant 4 bytes of the 16-byte field, "with the 12 most significant bytes zero-filled" (§4.2, Per-Peer Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPIntoIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L393) | unit/verify | unproven |
| positive | [`TestParseIPIntoIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L379) | unit/verify | unproven |

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
| negative | [`TestPeerUpOnCacheMissNeverReachesTheCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerup_openless_test.go#L40) | unit/verify | unproven |
| positive | [`TestHandleSenderStatePeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L136) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerUp`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L211) | unit/verify | unproven |

### [`RFC7854-x-9`](#rfc7854-x-9)

Route Monitoring messages must contain a BGP UPDATE message (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPSenderRouteMonitoring`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L279) | unit/verify | unproven |

### [`RFC7854-x-10`](#rfc7854-x-10)

Peer Down must include the reason code (1 byte) (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestHandleSenderStatePeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/event_test.go#L269) | unit/verify | unproven |
| positive | [`TestBMPSenderPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L248) | unit/verify | unproven |

### [`RFC7854-x-11`](#rfc7854-x-11)

Termination message may be sent before the BMP session is closed: "The router MAY send a Termination message prior to closing the session." (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSenderStopSendsTerminationToCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_queue_test.go#L561) | unit/verify | unproven |
| positive | [`TestBMPSenderTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/sender_test.go#L363) | unit/verify | unproven |

### [`RFC7854-x-12`](#rfc7854-x-12)

BMP is unidirectional: router to collector only (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBMPReceiverUnidirectional`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/session_test.go#L135) | unit/verify | unproven |

### [`RFC7854-4.9-1`](#rfc7854-4.9-1)

Peer Down must carry the Data field when the Reason is 1, 2 or 3: the BGP NOTIFICATION PDU for reason 1 and reason 3, and the 2-byte FSM event code for reason 2 (§4.9, Peer Down Notification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854PeerDownOmitsDataWhereTheReasonHasNone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerdown_data_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC7854PeerDownCarriesTheDataItsReasonRequires`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/peerdown_data_test.go#L88) | unit/verify | unproven |

### [`RFC7854-4.5-2`](#rfc7854-4.5-2)

Likewise, the monitoring station MUST close the TCP session after receiving a termination message. (§4.5, Termination Message)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBMPReceiverKeepsSessionWithoutTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_absent_test.go#L9) | unit/verify | unproven |
| positive | [`TestBMPReceiverClosesAfterTermination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/termination_close_test.go#L9) | unit/verify | unproven |

### [`RFC7854-x-15`](#rfc7854-x-15)

Statistics Reports may be sent periodically: "It MAY periodically send Stats Reports or even new Initiation messages, according to configuration." The Stats Reports text imposes no timing either: "This specification does not impose any timing restrictions on when and on what event these reports have to be transmitted." (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854StatisticsTimeoutZeroSendsNoReport`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L475) | unit/verify | revert, verified |
| positive | [`TestRFC7854StatisticsTimeoutSendsPeriodicReports`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/statistics_test.go#L422) | unit/verify | revert, verified |

### [`RFC7854-x-16`](#rfc7854-x-16)

Peer Up message should be sent for each established peer (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L463) | unit/verify | unproven |

### [`RFC7854-x-17`](#rfc7854-x-17)

Initial RIB dump via Route Monitoring should follow Peer Up (Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestConcurrentDumpsStayAddressedToTheirOwnCollector`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/bmp_reconnect_test.go#L466) | unit/verify | unproven |

### [`RFC7854-x-18`](#rfc7854-x-18)

Initiation message must include the sysDescr TLV (type 1): "The sysDescr and sysName Information TLVs MUST be sent, any others are optional." (Message Types)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationMissingSysDescrEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L15) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationEmptyIdentityValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L39) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationCarriesSysDescr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L29) | unit/verify | revert, verified |

### [`RFC7854-3.2-1`](#rfc7854-3.2-1)

Retries of a failed connection must be subject to some variety of backoff: "Retries MUST be subject to some variety of backoff." (§3.2, Connection Establishment and Termination)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854RetryNotBeforeBackoff`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC7854RetryBackoffDoubles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L64) | unit/verify | revert, verified |

### [`RFC7854-3.2-2`](#rfc7854-3.2-2)

The router must restrict the rate at which BMP sessions may be established: "The router MUST also restrict the rate at which sessions may be established." (§3.2, Connection Establishment and Termination)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC7854SessionEstablishmentRateLimited`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L173) | unit/verify | revert, verified |

### [`RFC7854-3.3-1`](#rfc7854-3.3-1)

Once all the routes for a given peer have been sent, an End-of-RIB message must be sent for that peer: "Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer" (§3.3, Lifecycle of a BMP Session, and restated in the Route Monitoring section)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854AdjReplayCompletionIsSessionScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L95) | unit/verify | unproven |
| negative | [`TestRFC7854IncompleteAdjReplayCannotClaimCompletion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC7854AdjReplayEndsEachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L67) | unit/verify | unproven |

### [`RFC7854-4.1-1`](#rfc7854-4.1-1)

Unrecognized message types must be ignored on receipt: "A BMP implementation MUST ignore unrecognized message types upon receipt." (§4.1, Common Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854MalformedKnownTypeStillEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestRFC7854UnrecognizedMessageTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L252) | unit/verify | revert, verified |

### [`RFC7854-4.2-1`](#rfc7854-4.2-1)

The reserved per-peer flag bits must be transmitted as 0 and their values ignored on receipt: "They MUST be transmitted as 0 and their values MUST be ignored on receipt." (§4.2, Per-Peer Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854ReservedFlagsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L357) | unit/verify | revert, verified |
| positive | [`TestRFC7854ReservedFlagsTransmittedAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L337) | unit/verify | revert, verified |

### [`RFC7854-4.4-1`](#rfc7854-4.4-1)

When multiple strings are included in a string TLV, their ordering must be preserved when they are reported: "If multiple strings are included, their ordering MUST be preserved when they are reported." (§4.4, Information TLV)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L32) | unit/verify | unproven |
| negative | [`TestRFC7854PeerUpStringsKeepDuplicates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L55) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L21) | unit/verify | unproven |
| positive | [`TestRFC7854PeerUpStringsReportedInOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L44) | unit/verify | unproven |

### [`RFC7854-4.4-2`](#rfc7854-4.4-2)

The sysDescr TLV Information field must equal the MIB-II sysDescr object: "The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysDescr MIB-II [RFC1213] object." (§4.4, Information TLV)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationDescriptionChangesWithBuild`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L227) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationDescribesRunningSystem`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L205) | unit/verify | unproven |

### [`RFC7854-4.4-3`](#rfc7854-4.4-3)

The sysName TLV Information field must equal the MIB-II sysName object: "The Information field contains an ASCII string whose value MUST be set to be equal to the value of the sysName MIB-II [RFC1213] object." (§4.4, Information TLV)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854InitiationSystemNameChanges`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L188) | unit/verify | unproven |
| positive | [`TestRFC7854InitiationUsesConfiguredSystemName`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_identity_test.go#L168) | unit/verify | unproven |

### [`RFC7854-4.5-1`](#rfc7854-4.5-1)

After sending a termination message the router must close the TCP session and send no further messages: "Once the router has sent a termination message, it MUST close the TCP session without sending any further messages." (§4.5, Termination Message)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L385) | unit/verify | revert, verified |
| positive | [`TestRFC7854TerminationThenCloseAndSilence`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L384) | unit/verify | revert, verified |

### [`RFC7854-4.5-3`](#rfc7854-4.5-3)

A Termination message must carry the Reason TLV (type 1): "Inclusion of this TLV is REQUIRED." (§4.5, Termination Message)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TerminationRequiresTwoByteReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L85) | unit/verify | unproven |
| positive | [`TestRFC7854SenderTerminationCarriesReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L53) | unit/verify | unproven |

### [`RFC7854-4.7-1`](#rfc7854-4.7-1)

A BGP Message TLV in a Route Mirroring message must occur last in the list of TLVs: "If the BGP Message TLV occurs in the Route Mirroring message, it MUST occur last in the list of TLVs." (§4.7, Route Mirroring)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854MirrorBGPMessageNotLastEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L133) | unit/verify | unproven |
| positive | [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L119) | unit/verify | unproven |
| positive | [`TestRFC7854RouteMirroringBGPMessageTLVLast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L421) | unit/verify | revert, verified |

### [`RFC7854-4.7-2`](#rfc7854-4.7-2)

A Route Mirroring message carrying Information code 0 (Errored PDU) must also carry a BGP Message TLV: "A BGP Message TLV MUST also occur in the TLV list." (§4.7, Route Mirroring)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854ErroredMirrorWithoutPDUEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L145) | unit/verify | unproven |
| positive | [`TestRFC7854ErroredMirrorWithPDUAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L120) | unit/verify | unproven |

### [`RFC7854-4.8-1`](#rfc7854-4.8-1)

Unrecognized stat types and unexpected Stat Data must be ignored on receipt: "A BMP implementation MUST ignore unrecognized stat types on receipt, and likewise MUST ignore unexpected data in the Stat Data field." (§4.8, Stats Reports)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854TruncatedStatEndsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestRFC7854UnrecognizedStatTypeIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L278) | unit/verify | revert, verified |

### [`RFC7854-4.8-2`](#rfc7854-4.8-2)

A transmitted Stats Report must carry at least one statistic: "However, if an SR message is transmitted, at least one statistic MUST be carried in it." (§4.8, Stats Reports)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854SenderRefusesEmptyStatistics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_codec_test.go#L174) | unit/verify | unproven |
| positive | [`TestRFC7854StatsReportCarriesAStatistic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L451) | unit/verify | revert, verified |

### [`RFC7854-5-1`](#rfc7854-5-1)

Pre-policy routes must have the L flag clear and post-policy routes must have it set: "Pre-policy routes MUST have their L flag clear in the BMP header (see Section 4), post-policy routes MUST have their L flag set." (§5, Route Monitoring)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L503) | unit/verify | revert, verified |
| positive | [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L478) | unit/verify | revert, verified |

### [`RFC7854-5-2`](#rfc7854-5-2)

Where the time a route was installed is not available, the BMP Timestamp field must be set to 0: "Otherwise, the BMP Timestamp field MUST be set to 0, indicating that time is not available." (§5, Route Monitoring)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854KnownRouteTimeSurvivesReplay`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC7854UnknownRouteTimeIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_replay_test.go#L122) | unit/verify | unproven |

### [`RFC7854-5-3`](#rfc7854-5-3)

A withdraw must carry the L flag of the announcement it withdraws, and must be sent twice where the route was announced both pre-policy and post-policy: "The withdraw MUST have its L flag set to correspond to that of any previous announcement; if the route in question was previously announced with L flag both clear and set, the withdraw MUST similarly be sent twice, with L flag clear and set." (§5, Route Monitoring)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854LFlagNotDecidedByBody`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L504) | unit/verify | revert, verified |
| positive | [`TestRFC7854LFlagFollowsPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L479) | unit/verify | revert, verified |

### [`RFC7854-8.2-1`](#rfc7854-8.2-1)

Where no transport session exists, the Local Port and Remote Port fields of a Peer Up message must be set to 0: "Since in this case no transport session actually exists, the Local and Remote Port fields of the Peer Up message MUST be set to 0." (§8.2, Peer Up Notification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7854BGPPeerUpCarriesTransportPorts`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L542) | unit/verify | revert, verified |
| positive | [`TestRFC7854LocRIBPeerUpPortsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/bmp/rfc7854_test.go#L528) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7854.txt |
| Source fingerprint | b32c6bf7d6b5d67a |
| Record | rfc/extraction/rfc7854.json |
| Mapped sentences | 22 |
| Declined as scope | 11 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
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
| `4.5` | not stated | 3 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 2 | walked | not stated |
| `4.8` | not stated | 2 | walked | not stated |
| `4.9` | not stated | 0 | walked | not stated |
| `4.10` | not stated | 0 | walked | not stated |
| `5` | not stated | 4 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 1 | walked | not stated |
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
| `3.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the obligation section 4.3 states at the site this walk maps to RFC7854-x-6: 'An initiation message MUST be sent as the first message after the TCP session comes up.' Section 3.3 states it as the first step of the session lifecycle; section 4.3 states it as the rule of the message itself. | It MUST begin by sending an Initiation message. |
| `5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the per-peer End-of-RIB obligation section 3.3 states at the site this walk maps to RFC7854-3.3-1: 'Once it has sent all the routes for a given peer, it MUST send an End-of-RIB message for that peer'. Section 5 adds only where the marker is defined (Section 2 of RFC 4724) and that it carries the BMP encapsulation header. | When the initial dump is completed for a given peer, this MUST be indicated by sending an End-of-RIB marker for that peer (as specified in Section 2 of [RFC4724], plus the BMP encapsulation header). |
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
