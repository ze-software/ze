# RFC 8210 - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1

Partial. Every requirement this repository extracted from RFC 8210, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 41.7% | 25 of 60 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.3% | 5 of 60 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 60 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 5.1% | 3 of 59 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 60 | of 84 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 25 | of 60 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 41.7% | 25 of 60 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 60 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 60 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 8.3% | 5 of 60 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 30 | of 60 gated MUSTs judged | 10 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 60 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 84 |
| Gated MUST-level | 60 |
| Not applicable, so out of scope | 25 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 61 |
| Tagged units | 59 |
| Recorded audit verdicts | 30 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc8210.md` |
| Requirement shard | `rfc/requirements/rfc8210.md` |
| RFC text | `rfc/full/rfc8210.txt` |

## Enrolment

Enrolled: RPKI to Router Protocol, Version 1

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Ze implements the RTR router client, not a cache server. It polls caches in preference order, replaces complete VRP and ASPA sets at End of Data, and expires those sets independently of connection progress. Native mutual TLS uses a named PKI CA and client identity, a DNS-ID cache reference, and no CN or plaintext fallback. Unprotected TCP requires an explicit trusted-network selection. Candidate PKI is isolated from live configuration
- transport changes retain data only for its original lease. Producers: internal/component/bgp/plugins/rpki/{rtr_session.go,rtr_tls.go,rtr_expire.go,rpki_config_verify.go,rpki_reload.go}.


**What the ledger says remains**

Five MUST rows carry {gap}. Session ID mismatch handling, received supported-version mismatches, and Router Key handling remain incomplete. The requirement rows below retain the specific remaining scope and verification obligations. Cache-server-only obligations do not describe the router's local staging or validation behavior.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 25 | one part of the gated population |
| Annotated instead of tested | 35 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **60** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (25):** [`RFC8210-5-1`](#rfc8210-5-1), [`RFC8210-5.1-1`](#rfc8210-5.1-1), [`RFC8210-5.1-2`](#rfc8210-5.1-2), [`RFC8210-5.11-1`](#rfc8210-5.11-1), [`RFC8210-6-2`](#rfc8210-6-2), [`RFC8210-7-3`](#rfc8210-7-3), [`RFC8210-5.2-1`](#rfc8210-5.2-1), [`RFC8210-4-1`](#rfc8210-4-1), [`RFC8210-8.3-1`](#rfc8210-8.3-1), [`RFC8210-12-1`](#rfc8210-12-1), [`RFC8210-9-3`](#rfc8210-9-3), [`RFC8210-9.2-1`](#rfc8210-9.2-1), [`RFC8210-9.2-2`](#rfc8210-9.2-2), [`RFC8210-9.2-4`](#rfc8210-9.2-4), [`RFC8210-9.2-5`](#rfc8210-9.2-5), [`RFC8210-10-1`](#rfc8210-10-1), [`RFC8210-7-7`](#rfc8210-7-7), [`RFC8210-7-8`](#rfc8210-7-8), [`RFC8210-9.2-8`](#rfc8210-9.2-8), [`RFC8210-10-2`](#rfc8210-10-2), [`RFC8210-9.2-10`](#rfc8210-9.2-10), [`RFC8210-8.4-1`](#rfc8210-8.4-1), [`RFC8210-3-1`](#rfc8210-3-1), [`RFC8210-8.1-3`](#rfc8210-8.1-3), [`RFC8210-9.2-11`](#rfc8210-9.2-11)

**Annotated instead of tested (35):** [`RFC8210-5.1-3`](#rfc8210-5.1-3), [`RFC8210-5.1-4`](#rfc8210-5.1-4), [`RFC8210-5.1-5`](#rfc8210-5.1-5), [`RFC8210-5.3-1`](#rfc8210-5.3-1), [`RFC8210-5.3-2`](#rfc8210-5.3-2), [`RFC8210-5.5-1`](#rfc8210-5.5-1), [`RFC8210-5.6-1`](#rfc8210-5.6-1), [`RFC8210-5.10-1`](#rfc8210-5.10-1), [`RFC8210-5.10-2`](#rfc8210-5.10-2), [`RFC8210-5.8-1`](#rfc8210-5.8-1), [`RFC8210-5.11-2`](#rfc8210-5.11-2), [`RFC8210-5.11-3`](#rfc8210-5.11-3), [`RFC8210-5.11-4`](#rfc8210-5.11-4), [`RFC8210-6-1`](#rfc8210-6-1), [`RFC8210-7-1`](#rfc8210-7-1), [`RFC8210-7-2`](#rfc8210-7-2), [`RFC8210-7-4`](#rfc8210-7-4), [`RFC8210-8.1-1`](#rfc8210-8.1-1), [`RFC8210-8.2-1`](#rfc8210-8.2-1), [`RFC8210-9-1`](#rfc8210-9-1), [`RFC8210-9-2`](#rfc8210-9-2), [`RFC8210-9.1-1`](#rfc8210-9.1-1), [`RFC8210-9.1-2`](#rfc8210-9.1-2), [`RFC8210-9.2-3`](#rfc8210-9.2-3), [`RFC8210-9.2-6`](#rfc8210-9.2-6), [`RFC8210-9.3-1`](#rfc8210-9.3-1), [`RFC8210-9.4-1`](#rfc8210-9.4-1), [`RFC8210-4-2`](#rfc8210-4-2), [`RFC8210-4-3`](#rfc8210-4-3), [`RFC8210-9.3-2`](#rfc8210-9.3-2), [`RFC8210-9.4-2`](#rfc8210-9.4-2), [`RFC8210-9.4-3`](#rfc8210-9.4-3), [`RFC8210-9.2-9`](#rfc8210-9.2-9), [`RFC8210-8.1-2`](#rfc8210-8.1-2), [`RFC8210-2-1`](#rfc8210-2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8210-5-1` | Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L80). **positive:** `unit/verify` [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L19). **positive:** `unit/verify` [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L55). **negative:** `unit/verify` [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L39) |
| `RFC8210-5.1-1` | The remaining bits in the Flags field are reserved for future use. In protocol version 1, they MUST be zero on transmission and MUST be ignored on receipt. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L333). **negative:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L342). **negative:** `unit/verify` [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L81) |
| `RFC8210-5.1-2` | Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the Prefix element. This MUST NOT be less than the Prefix Length element. (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L59). **negative:** `unit/verify` [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L132) |
| `RFC8210-5.1-3` | If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data") (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePDU adopts the Cache Response Session ID without comparing it with the prior session; the Error Report encoder exists, but this mismatch does not reach it (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-5.1-4` | If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Cache Response Session ID changes are not detected, so they do not trigger the required data flush (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-5.1-5` | routers MUST treat sessions with different Protocol Version fields as separate sessions even if they do happen to have the same Session ID. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** downgrade after Error Report 4 resets the serial base, but other received supported-version mismatches are not separated or rejected; handlePDU rejects versions outside the supported range without comparing every payload against the negotiated version (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-5.3-1` | When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side response obligation -- Ze's router writes queries and Error Reports, not cache payload responses (internal/component/bgp/plugins/rpki/rtr_pdu.go) |
| `RFC8210-5.3-2` | That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side response obligation -- Ze receives payload PDUs and emits no change set (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-5.5-1` | When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) and has no Prefix PDU writer, so it never sets this field on transmission |
| `RFC8210-5.6-1` | The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission guarantee -- ze only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| `RFC8210-5.10-1` | The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission guarantee -- ze has no Router Key PDU writer; the only Router Key code is the discard arm in handlePDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379), so it emits no Router Key PDU whose uniqueness it would have to ensure |
| `RFC8210-5.10-2` | For this reason, implementations MUST compare Subject Public Key values as well as SKIs when detecting duplicate PDUs. (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs no Router Key duplicate detection -- handlePDU discards every Router Key PDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379) and stores neither SKI nor Subject Public Key, so no Subject Public Key comparison happens anywhere |
| `RFC8210-5.8-1` | The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads End of Data for its serial and timing parameters only (internal/component/bgp/plugins/rpki/rtr_session.go:291-306) and emits neither Cache Response nor End of Data PDUs whose Session ID and version it would have to match |
| `RFC8210-5.11-1` | An Error Report PDU MUST NOT be sent for an Error Report PDU. (§5.11) | MUST NOT | 5.11 | **positive:** `unit/verify` [`TestRTRErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L186). **negative:** `unit/verify` [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L148) |
| `RFC8210-5.11-2` | If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the router's readLoop reports malformed ASPA and unsupported-version PDUs, always with their associated offending PDU; it generates no generic, PDU-independent error report (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-5.11-3` | If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** writeErrorReport emits no optional diagnostic text and has no text input; its final field is a zero text length (internal/component/bgp/plugins/rpki/rtr_pdu.go) |
| `RFC8210-5.11-4` | The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (§5.11) | MUST | 5.11 | **positive:** `unit/verify` [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L149). **negative:** no negative test. **{single-polarity}:** ze never emits diagnostic text: internal/component/bgp/plugins/rpki/rtr_pdu.go::writeErrorReport takes no text argument and always writes a zero Length of Error Text, so no input makes ze send a report with text to contrast against |
| `RFC8210-6-1` | Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side timing obligation -- Ze receives End of Data and emits no timing parameters (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-6-2` | Expire Interval: This parameter tells the router how long it can continue to use the current version of the data while unable to perform a successful subsequent query. The router MUST NOT retain the data past the time indicated by this parameter. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L13). **negative:** `unit/verify` [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L14). **negative:** `unit/verify` [`TestRTRSerialDeltaCannotRenewAnElapsedLease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L93) |
| `RFC8210-7-1` | A router MUST start each transport connection by issuing either a Reset Query or a Serial Query. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L513). **negative:** no negative test. **{single-polarity}:** connectAndSync writes a Reset Query or a Serial Query as the first bytes of every connection (internal/component/bgp/plugins/rpki/rtr_session.go:165-176) before entering readLoop, and no received input can make the router open a connection without a query, so there is no negative case |
| `RFC8210-7-2` | If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side obligation -- ze runs no RTR cache server; RTRSession dials out to a configured cache (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync) and receives no queries to downgrade or reject |
| `RFC8210-7-3` | The cache may reply with a version 0 response. In this case, the router MUST either downgrade to version 0 or terminate the connection. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L226). **negative:** `unit/verify` [`TestRFC8210V1CacheResponseIsNotTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_rfc8210_version_test.go#L23) |
| `RFC8210-5.2-1` | If the router receives a Serial Notify PDU during the initial startup period where the router and cache are still negotiating to agree on a protocol version, the router MUST simply ignore the Serial Notify PDU, even if the Serial Notify PDU is for an unexpected protocol version. (§5.2, §7) | MUST | 5.2 | **positive:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L424). **negative:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L450) |
| `RFC8210-7-4` | If either party receives a PDU for a different Protocol Version once the above negotiation completes, that party MUST drop the session (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePDU rejects versions outside the supported range but does not compare a supported received version against the negotiated s.version (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-4-1` | The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L31). **negative:** `unit/verify` [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L32) |
| `RFC8210-8.1-1` | To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query periodically. (§8.1, §8.2) | MUST | 8.1 | **positive:** `unit/verify` [`TestRunPollsOnRefreshAfterSuccessRetryAfterFailure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_poll_test.go#L25). **negative:** no negative test. **{single-polarity}:** this is a periodic sender obligation. cacheGroup.Run selects the cache-supplied refresh interval after success and retry interval after failure; TestRunPollsOnRefreshAfterSuccessRetryAfterFailure observes actual Reset and Serial Queries rather than timer fields |
| `RFC8210-8.3-1` | Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L304). **negative:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L320) |
| `RFC8210-12-1` | Errors which are considered fatal MUST cause the session to be dropped. (§12) | MUST | 12 | **positive:** `unit/verify` [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L83). **negative:** `unit/verify` [`TestIsFatalError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L211) |
| `RFC8210-8.2-1` | The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it (internal/component/bgp/plugins/rpki/rtr_session.go:359-361) and has no Serial Notify writer |
| `RFC8210-9-1` | Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 14. (§9) | MUST | 9 | **positive:** `unit/verify` [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L88). **negative:** no negative test. **{single-polarity}:** Ze offers unprotected TCP with explicit trusted-network true and defaults that transport to port 323 (internal/component/bgp/plugins/rpki/rpki_config.go; internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-9-2` | If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** operator deployment obligation -- trusted-network true explicitly permits TCP but cannot establish that the actual network topology is trusted and controlled (internal/component/bgp/plugins/rpki/rpki_config.go) |
| `RFC8210-9-3` | If available to the operator, caches and routers MUST use one of the following more protected protocols: (§9) | MUST | 9 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L34). **negative:** `unit/verify` [`TestRTRTLSNoPlaintextFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L182) |
| `RFC8210-9.1-1` | Cache servers supporting SSH transport MUST accept RSA authentication (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server SSH obligation -- Ze's selected RTR role is router client, with native TCP and TLS transports but no RTR SSH endpoint (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-9.1-2` | User authentication MUST be supported (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** SSH-transport obligation -- the RTR client implements TCP and TLS, not SSH (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-9.2-1` | Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L35). **negative:** `unit/verify` [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L136) |
| `RFC8210-9.2-2` | Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L36). **negative:** `unit/verify` [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L137) |
| `RFC8210-9.2-3` | when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side TLS obligation -- ze runs no RTR TLS server (it dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting client IP against iPAddress identities |
| `RFC8210-9.2-4` | Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid MITM attacks. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L37). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L94) |
| `RFC8210-9.2-5` | rpki-rtr implementations which use TLS MUST NOT use Common Name (CN-ID) identifiers (§9.2) | MUST NOT | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L38). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L95) |
| `RFC8210-9.2-6` | the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** server-certificate obligation -- the selected RTR role is router client only; startTLS verifies the remote cache certificate but Ze serves no RTR TLS endpoint (internal/component/bgp/plugins/rpki/rtr_tls.go) |
| `RFC8210-9.3-1` | If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- the RTR client implements TCP and TLS, not RTR TCP-MD5 (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-9.4-1` | Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- the RTR client implements TCP and TLS, not RTR TCP-AO (internal/component/bgp/plugins/rpki/rtr_session.go) |
| `RFC8210-10-1` | If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation of BGP announcements. (§10) | MUST NOT | 10 | **positive:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L154). **negative:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L162) |
| `RFC8210-4-2` | As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** server-clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache server and holds no clock this binds |
| `RFC8210-4-3` | Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no Serial Number ordering comparison -- it stores the serial the cache reports (internal/component/bgp/plugins/rpki/rtr_session.go:297) and tests only serial == 0 to choose between a Reset Query and a Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:166); an equality test against zero is unaffected by wrap-around, and grep over the package finds no other serial comparison |
| `RFC8210-5.1-6` | To reduce the risk of confusion, cache servers SHOULD NOT use the same Session ID across multiple protocol versions (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.6-2` | Should the router client receive an IPvX PDU with a {Prefix, Len, Max-Len, ASN} identical to one it already has active, it SHOULD raise a Duplicate Announcement Received error. (§5.6, §5.10) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-5` | If an erroneous Error Report PDU is received, the session SHOULD be dropped. (§5.11) | SHOULD | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-3` | Refresh Interval: This parameter tells the router how long to wait before next attempting to poll the cache and between subsequent attempts, using a Serial Query or Reset Query PDU. The router SHOULD NOT poll the cache sooner than indicated by this parameter. (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-4` | Retry Interval: This parameter tells the router how long to wait before retrying a failed Serial Query or Reset Query. The router SHOULD NOT retry sooner than indicated by this parameter. (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-5` | Caches SHOULD NOT send Serial Notify PDUs before version negotiation completes. (§7) | SHOULD NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.3-2` | When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-4` | To reduce exposure to dropped but non-terminated sessions, both caches and routers SHOULD enable keep-alives when available in the chosen transport protocol. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-5` | Caches and routers SHOULD use TCP-AO transport [RFC5925] over the rpki-rtr port. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-3` | Cache servers supporting SSH transport MUST accept RSA authentication and SHOULD accept Elliptic Curve Digital Signature Algorithm (ECDSA) authentication. (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-4` | Client routers SHOULD verify the public key of the cache to avoid MITM attacks. (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.2-7` | DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*". (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.2-2` | Upon receipt of a Serial Notify PDU, the router MAY issue an immediate Serial Query (Section 5.3) or Reset Query (Section 5.4) without waiting for the Refresh Interval timer (see Section 6) to expire. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-6` | If the error is associated with a PDU of excessive length, i.e., too long to be any legal PDU other than another Error Report, or a possibly corrupt length, the Erroneous PDU field MAY be truncated. (§5.11) | MAY | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-6` | The cache may terminate the connection, perhaps with a version 0 Error Report PDU. In this case, the router MAY retry the connection using protocol version 0. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-5` | if the router needs to downgrade to a lower protocol version number, it MAY send the first Serial Query or Reset Query immediately. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-6` | Caches and routers MAY use Secure Shell version 2 (SSHv2) transport [RFC4252] using the normal SSH port. For an example, see Section 9.1. o Caches and routers MAY use TCP MD5 transport [RFC2385] using the rpki-rtr port. Note that TCP MD5 has been obsoleted by TCP-AO [RFC5925]. o Caches and routers MAY use TCP over IPsec transport [RFC4301] using the rpki-rtr port. o Caches and routers MAY use Transport Layer Security (TLS) transport [RFC5246] using port rpki-rtr-tls (324); see Section 14. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-7` | If either party receives a PDU containing an unrecognized Protocol Version (neither 0 nor 1) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU unless the received PDU is itself an Error Report PDU. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L224). **negative:** `unit/verify` [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L225) |
| `RFC8210-7-8` | The router MUST ignore any Serial Notify PDUs it might receive from the cache during this initial startup period, regardless of the Protocol Version field in the Serial Notify PDU. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L427). **negative:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L454) |
| `RFC8210-9.2-8` | o The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache. (§9.2) | MUST | 9.2 | **positive:** `unit/verify` [`TestRTRTLSDNSAddressIsTheReference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L221). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L97) |
| `RFC8210-10-2` | A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L125). **negative:** `unit/verify` [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L126) |
| `RFC8210-9.3-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| `RFC8210-9.4-2` | Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| `RFC8210-9.4-3` | The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |
| `RFC8210-9.2-9` | Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** issuer obligation -- Ze's PKI store imports certificates; the RTR router client does not issue cache-server certificates (internal/component/pki/config.go; internal/component/bgp/plugins/rpki/rtr_tls.go) |
| `RFC8210-9.2-10` | a CN field may be present in the server certificate's subject name but MUST NOT be used for authentication within the rules described in [RFC6125]. (§9.2) | MUST NOT | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L39). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L96) |
| `RFC8210-8.1-2` | When a transport connection is first established, the router MUST send either a Reset Query or a Serial Query. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L516). **negative:** no negative test. **{single-polarity}:** the first bytes connectAndSync writes on a freshly established transport are a Reset Query when the serial is 0 and a Serial Query otherwise (internal/component/bgp/plugins/rpki/rtr_session.go:165-176); no received input can produce a connection that carries no opening query, so there is no negative case |
| `RFC8210-8.4-1` | When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (§8.4) | MUST | 8.4 | **positive:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L337). **negative:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L338) |
| `RFC8210-2-1` | While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- the cache holds a fetch in progress and decides when to send it; ze is the router, receives payload PDUs and serves no cache (internal/component/bgp/plugins/rpki/rtr_session.go::handlePDU) |
| `RFC8210-3-1` | A Relying Party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any cache(s) it uses. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L33). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L93) |
| `RFC8210-8.1-3` | In all other cases, the router lacks the necessary data for fast resynchronization and therefore MUST fall back to a Reset Query. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/resync_rfc8210_test.go#L67). **negative:** `unit/verify` [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/resync_rfc8210_test.go#L81) |
| `RFC8210-9.2-11` | o Support for the DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations which use TLS. (§9.2) | REQUIRED | 9.2 | **positive:** `unit/verify` [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L32). **negative:** `unit/verify` [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L92) |
| `RFC8210-7-9` | unless the PDU containing the unexpected Protocol Version was itself an Error Report PDU, the party dropping the session SHOULD send an Error Report with an error code of 8 ("Unexpected Protocol Version"). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-10-3` | The client SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a different cache or established a new connection to the previous cache. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-10-4` | If a client loses connectivity to a cache it is using or otherwise decides to switch to a new cache, it SHOULD retain the data from the previous cache until it has a full set of data from one or more other caches. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.3-3` | Key rollover with TCP MD5 is problematic. Cache servers SHOULD support [RFC4808]. (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.4-2` | When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-11-1` | The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa, before any data are exchanged. (§13) | SHOULD | 13 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.2-2` | The cache server SHOULD send a Notify PDU with its current Serial Number when the cache's serial changes (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8210-5.1-3`](#rfc8210-5.1-3) If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data") (§5.1) | {gap}, no test | handlePDU adopts the Cache Response Session ID without comparing it with the prior session; the Error Report encoder exists, but this mismatch does not reach it (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-5.1-4`](#rfc8210-5.1-4) If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache. (§5.1) | {gap}, no test | Cache Response Session ID changes are not detected, so they do not trigger the required data flush (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-5.1-5`](#rfc8210-5.1-5) routers MUST treat sessions with different Protocol Version fields as separate sessions even if they do happen to have the same Session ID. (§5.1) | {gap}, no test | downgrade after Error Report 4 resets the serial base, but other received supported-version mismatches are not separated or rejected; handlePDU rejects versions outside the supported range without comparing every payload against the negotiated version (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-5.3-1`](#rfc8210-5.3-1) When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side response obligation -- Ze's router writes queries and Error Reports, not cache payload responses (internal/component/bgp/plugins/rpki/rtr_pdu.go) |
| [`RFC8210-5.3-2`](#rfc8210-5.3-2) That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side response obligation -- Ze receives payload PDUs and emits no change set (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-5.5-1`](#rfc8210-5.5-1) When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) and has no Prefix PDU writer, so it never sets this field on transmission |
| [`RFC8210-5.6-1`](#rfc8210-5.6-1) The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission guarantee -- ze only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| [`RFC8210-5.10-1`](#rfc8210-5.10-1) The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission guarantee -- ze has no Router Key PDU writer; the only Router Key code is the discard arm in handlePDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379), so it emits no Router Key PDU whose uniqueness it would have to ensure |
| [`RFC8210-5.10-2`](#rfc8210-5.10-2) For this reason, implementations MUST compare Subject Public Key values as well as SKIs when detecting duplicate PDUs. (§5.10) | {gap}, no test | ze performs no Router Key duplicate detection -- handlePDU discards every Router Key PDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379) and stores neither SKI nor Subject Public Key, so no Subject Public Key comparison happens anywhere |
| [`RFC8210-5.8-1`](#rfc8210-5.8-1) The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads End of Data for its serial and timing parameters only (internal/component/bgp/plugins/rpki/rtr_session.go:291-306) and emits neither Cache Response nor End of Data PDUs whose Session ID and version it would have to match |
| [`RFC8210-5.11-2`](#rfc8210-5.11-2) If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: the router's readLoop reports malformed ASPA and unsupported-version PDUs, always with their associated offending PDU; it generates no generic, PDU-independent error report (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-5.11-3`](#rfc8210-5.11-3) If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: writeErrorReport emits no optional diagnostic text and has no text input; its final field is a zero text length (internal/component/bgp/plugins/rpki/rtr_pdu.go) |
| [`RFC8210-6-1`](#rfc8210-6-1) Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side timing obligation -- Ze receives End of Data and emits no timing parameters (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-7-2`](#rfc8210-7-2) If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side obligation -- ze runs no RTR cache server; RTRSession dials out to a configured cache (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync) and receives no queries to downgrade or reject |
| [`RFC8210-7-4`](#rfc8210-7-4) If either party receives a PDU for a different Protocol Version once the above negotiation completes, that party MUST drop the session (§7) | {gap}, no test | handlePDU rejects versions outside the supported range but does not compare a supported received version against the negotiated s.version (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-8.2-1`](#rfc8210-8.2-1) The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it (internal/component/bgp/plugins/rpki/rtr_session.go:359-361) and has no Serial Notify writer |
| [`RFC8210-9-2`](#rfc8210-9-2) If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: operator deployment obligation -- trusted-network true explicitly permits TCP but cannot establish that the actual network topology is trusted and controlled (internal/component/bgp/plugins/rpki/rpki_config.go) |
| [`RFC8210-9.1-1`](#rfc8210-9.1-1) Cache servers supporting SSH transport MUST accept RSA authentication (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server SSH obligation -- Ze's selected RTR role is router client, with native TCP and TLS transports but no RTR SSH endpoint (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-9.1-2`](#rfc8210-9.1-2) User authentication MUST be supported (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: SSH-transport obligation -- the RTR client implements TCP and TLS, not SSH (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-9.2-3`](#rfc8210-9.2-3) when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side TLS obligation -- ze runs no RTR TLS server (it dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting client IP against iPAddress identities |
| [`RFC8210-9.2-6`](#rfc8210-9.2-6) the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: server-certificate obligation -- the selected RTR role is router client only; startTLS verifies the remote cache certificate but Ze serves no RTR TLS endpoint (internal/component/bgp/plugins/rpki/rtr_tls.go) |
| [`RFC8210-9.3-1`](#rfc8210-9.3-1) If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- the RTR client implements TCP and TLS, not RTR TCP-MD5 (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-9.4-1`](#rfc8210-9.4-1) Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- the RTR client implements TCP and TLS, not RTR TCP-AO (internal/component/bgp/plugins/rpki/rtr_session.go) |
| [`RFC8210-4-2`](#rfc8210-4-2) As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: server-clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache server and holds no clock this binds |
| [`RFC8210-4-3`](#rfc8210-4-3) Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no Serial Number ordering comparison -- it stores the serial the cache reports (internal/component/bgp/plugins/rpki/rtr_session.go:297) and tests only serial == 0 to choose between a Reset Query and a Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:166); an equality test against zero is unaffected by wrap-around, and grep over the package finds no other serial comparison |
| [`RFC8210-9.3-2`](#rfc8210-9.3-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| [`RFC8210-9.4-2`](#rfc8210-9.4-2) Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| [`RFC8210-9.4-3`](#rfc8210-9.4-3) The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |
| [`RFC8210-9.2-9`](#rfc8210-9.2-9) Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: issuer obligation -- Ze's PKI store imports certificates; the RTR router client does not issue cache-server certificates (internal/component/pki/config.go; internal/component/bgp/plugins/rpki/rtr_tls.go) |
| [`RFC8210-2-1`](#rfc8210-2-1) While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- the cache holds a fetch in progress and decides when to send it; ze is the router, receives payload PDUs and serves no cache (internal/component/bgp/plugins/rpki/rtr_session.go::handlePDU) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8210-5-1`](#rfc8210-5-1)

Reserved fields (marked "zero" in PDU diagrams) MUST be zero on transmission and MUST be ignored on receipt. (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Transmit clause proven: TestWriteResetQueryZeroesReservedOverGarbage asserts octets 2-3 of the Reset Query zero over a 0xFF buffer (the only reserved field in a PDU ze writes). Receipt clause proven only for the IPv4 and IPv6 Prefix PDUs (TestReservedPrefixFieldsPreserveValidation, dirty header and octet-11 bytes still authorize). The Cache Reset PDU's reserved octets 2-3 are never sent nonzero by any tagged unit (TestCacheResetTriggersResetQuery uses a zeroed buffer), so a handlePDU that refused a Cache Reset with nonzero reserved octets stays green

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L39) | unit/verify | unproven |
| positive | [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L80) | unit/verify | unproven |
| positive | [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L55) | unit/verify | unproven |
| positive | [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L19) | unit/verify | unproven |

### [`RFC8210-5.1-1`](#rfc8210-5.1-1)

The remaining bits in the Flags field are reserved for future use. In protocol version 1, they MUST be zero on transmission and MUST be ignored on receipt. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden on receipt: letting reserved Flags bits affect acceptance or the announce/withdraw decision. TestParsePrefixPDUFlagsHighBitsIgnored (rtr_pdu_test.go) asserts 0xFF is accepted (require.NoError) and read as announce, and 0xFE is accepted and read as withdraw (assert.False announce), so a parser that validated or read the high bits goes red; negative TestReservedPrefixFieldsPreserveValidation repeats the withdrawal case through handlePDU for IPv4 and IPv6. Transmit clause: Ze writes only Reset Query, Serial Query and Error Report PDUs (rtr_pdu.go), none of which carries a Flags field, so there is no producer that could violate it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReservedPrefixFieldsPreserveValidation`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L81) | unit/verify | unproven |
| negative | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L342) | unit/verify | unproven |
| positive | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L333) | unit/verify | unproven |

### [`RFC8210-5.1-2`](#rfc8210-5.1-2)

Max Length: An 8-bit unsigned integer denoting the longest prefix allowed by the Prefix element. This MUST NOT be less than the Prefix Length element. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Prefix PDU whose Max Length is less than its Prefix Length. TestParseIPv4PrefixMaxLenLessThanPrefixLen builds a PDU with prefix length 24 and max length 16 and asserts parseIPv4Prefix returns an error containing 'invalid' (the prefix length alone is valid, so the buffer isolates the rule); positive TestParseIPv4Prefix accepts max length 24 over prefix length 8. Both buffers carry rtrVersionMin (1), the RFC 8210 version.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L132) | unit/verify | unproven |
| positive | [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L59) | unit/verify | unproven |

### [`RFC8210-5.1-3`](#rfc8210-5.1-3)

If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data") (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-3, so no unit is bound to it.

### [`RFC8210-5.1-4`](#rfc8210-5.1-4)

If, at any time after the protocol version has been negotiated (Section 7), either the router or the cache finds that the value of the Session ID is not the same as the other's, the party which detects the mismatch MUST immediately terminate the session with an Error Report PDU with code 0 ("Corrupt Data"), and the router MUST flush all data learned from that cache. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-4, so no unit is bound to it.

### [`RFC8210-5.1-5`](#rfc8210-5.1-5)

routers MUST treat sessions with different Protocol Version fields as separate sessions even if they do happen to have the same Session ID. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-5, so no unit is bound to it.

### [`RFC8210-5.3-1`](#rfc8210-5.3-1)

When replying to a Serial Query, the cache MUST return the minimum set of changes needed to bring the router into sync with the cache. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.3-1, so no unit is bound to it.

### [`RFC8210-5.3-2`](#rfc8210-5.3-2)

That is, if a particular prefix or router key underwent multiple changes between the Serial Number specified by the router and the cache's current Serial Number, the cache MUST merge those changes to present the simplest possible view of those changes to the router. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.3-2, so no unit is bound to it.

### [`RFC8210-5.5-1`](#rfc8210-5.5-1)

When replying to a Reset Query (Section 5.4), the cache sends the set of all data records it has; in this case, the withdraw/announce field in the payload PDUs MUST have the value 1 (announce). (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.5-1, so no unit is bound to it.

### [`RFC8210-5.6-1`](#rfc8210-5.6-1)

The cache server MUST ensure that it has told the router client to have one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time. (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.6-1, so no unit is bound to it.

### [`RFC8210-5.10-1`](#rfc8210-5.10-1)

The cache server MUST ensure that it has told the router client to have one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any one point in time. (§5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.10-1, so no unit is bound to it.

### [`RFC8210-5.10-2`](#rfc8210-5.10-2)

For this reason, implementations MUST compare Subject Public Key values as well as SKIs when detecting duplicate PDUs. (§5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.10-2, so no unit is bound to it.

### [`RFC8210-5.8-1`](#rfc8210-5.8-1)

The Session ID and Protocol Version MUST be the same as that of the corresponding Cache Response which began the (possibly null) sequence of payload PDUs. (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.8-1, so no unit is bound to it.

### [`RFC8210-5.11-1`](#rfc8210-5.11-1)

An Error Report PDU MUST NOT be sent for an Error Report PDU. (§5.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering an Error Report with an Error Report. TestRTRErrorReportIsNotAnswered writes an Error Report with version 99 (which would otherwise draw Unsupported Protocol Version) into readLoop and asserts the cache side's one-byte io.ReadFull ends in io.EOF, so any reply byte goes red. Negative TestASPAProviderListErrorOnWire: a malformed ASPA PDU does draw an Error Report (report[1] == pduErrorRpt, code 9, the encapsulated PDU echoed), so a router that never sent Error Reports goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L148) | unit/verify | unproven |
| positive | [`TestRTRErrorReportIsNotAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L186) | unit/verify | unproven |

### [`RFC8210-5.11-2`](#rfc8210-5.11-2)

If the error is generic (e.g., "Internal Error") and not associated with the PDU to which it is responding, the Erroneous PDU field MUST be empty and the Length of Encapsulated PDU field MUST be zero. (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-2, so no unit is bound to it.

### [`RFC8210-5.11-3`](#rfc8210-5.11-3)

If error text is present, it MUST be a string in UTF-8 encoding (see [RFC3629]). (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-3, so no unit is bound to it.

### [`RFC8210-5.11-4`](#rfc8210-5.11-4)

The diagnostic text is optional; if not present, the Length of Error Text field MUST be zero. (§5.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a nonzero Length of Error Text on a report carrying no diagnostic text. TestASPAProviderListErrorOnWire reads the transmitted Error Report and asserts the uint32 after the encapsulated PDU is 0 (assert.Equal uint32(0)). Row carries {single-polarity: positive}: writeErrorReport takes no text, so no input produces a report with text to contrast.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestASPAProviderListErrorOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L149) | unit/verify | unproven |

### [`RFC8210-6-1`](#rfc8210-6-1)

Caches MUST set Expire Interval to a value larger than either Refresh Interval or Retry Interval. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-6-1, so no unit is bound to it.

### [`RFC8210-6-2`](#rfc8210-6-2)

Expire Interval: This parameter tells the router how long it can continue to use the current version of the data while unable to perform a successful subsequent query. The router MUST NOT retain the data past the time indicated by this parameter. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using VRPs or ASPAs after the Expire Interval measured from the last completed End of Data. TestRTRDataExpiryAndRenewal (synctest) asserts the payloads still validate at expire-1s (assertLive) and, after a renewal and an incomplete response, that at the lease deadline cache.Validate is NotFound and aspaCache.isProvider false ("expired payloads still authorize routes"), with both consumers notified; the negatives assert a completed EOD renews (assertLive after renewal), an incomplete response neither renews nor publishes, and TestRTRSerialDeltaCannotRenewAnElapsedLease asserts a serial EOD after the deadline returns errRtrDataExpired and publishes nothing even with the expiry timer stopped. The quote now carries the Expire Interval definition sentence, which names the parameter the MUST NOT refers to and adds no obligation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L14) | unit/verify | unproven |
| negative | [`TestRTRSerialDeltaCannotRenewAnElapsedLease`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L93) | unit/verify | unproven |
| positive | [`TestRTRDataExpiryAndRenewal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_expire_test.go#L13) | unit/verify | unproven |

### [`RFC8210-7-1`](#rfc8210-7-1)

A router MUST start each transport connection by issuing either a Reset Query or a Serial Query. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a transport connection on which the router does not speak first with a Reset or Serial Query. TestFirstPDUOnConnectionIsAQuery accepts the connection at a real listener and require.NoError on io.ReadFull of the first PDU (a silent router times out), then asserts buf[1] == pduResetQuery for a fresh session and pduSerialQuery for a resuming one. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L513) | unit/verify | unproven |

### [`RFC8210-7-2`](#rfc8210-7-2)

If a cache which supports version 1 receives a query from a router which specifies version 0, the cache MUST downgrade to protocol version 0 [RFC6810] or send a version 1 Error Report PDU with Error Code 4 ("Unsupported Protocol Version") and terminate the connection. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-2, so no unit is bound to it.

### [`RFC8210-7-3`](#rfc8210-7-3)

The cache may reply with a version 0 response. In this case, the router MUST either downgrade to version 0 or terminate the connection. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after a version 0 reply to the router's query, continuing at the router's own version (neither downgrading nor terminating). TestRTRUnknownNegotiationVersion "unsupported v0": the cache sends a version 0 Cache Response, and the test asserts syncOnce returns errRtrUnsupportedProtocol and the cache's trailing read reaches io.EOF (require.ErrorIs got.err io.EOF), so a router that kept the stream open or loaded the v0 reply goes red. Ze supports no version 0, so terminating is the branch it takes. Negative TestRFC8210V1CacheResponseIsNotTerminated: a reply at the router's own version keeps s.version, publishes its VRP (Validate Valid), and draws no trailing bytes (assert.Empty), so a check that terminated on every Cache Response goes red. The quote now carries the conditioning sentence (the cache replies with a version 0 response).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8210V1CacheResponseIsNotTerminated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_rfc8210_version_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L226) | unit/verify | unproven |

### [`RFC8210-5.2-1`](#rfc8210-5.2-1)

If the router receives a Serial Notify PDU during the initial startup period where the router and cache are still negotiating to agree on a protocol version, the router MUST simply ignore the Serial Notify PDU, even if the Serial Notify PDU is for an unexpected protocol version. (§5.2, §7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a startup Serial Notify erroring the session or being adopted, including one for an unexpected protocol version. TestSerialNotifyIgnoredDuringStartup asserts no error, done=false, serial/sessionID/state/version untouched, and the negative that a Cache Response in the same window is acted on; but its notify carries rtrVersionMax, the expected version. The clause 'even if the Serial Notify PDU is for an unexpected protocol version' has no assertion: removing the Serial Notify exemption from handlePDU's version check leaves both subtests green (same gap as RFC8210-7-8)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L450) | unit/verify | unproven |
| positive | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L424) | unit/verify | unproven |

### [`RFC8210-7-4`](#rfc8210-7-4)

If either party receives a PDU for a different Protocol Version once the above negotiation completes, that party MUST drop the session (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-4, so no unit is bound to it.

### [`RFC8210-4-1`](#rfc8210-4-1)

The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: selecting a cache other than the most preferred one that answers (config order, or reading every cache at once). TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers lists the standby (preference 200) first and the preferred (10) second; it waits for the preferred cache's accept and t.Fatal fires if the standby accepts within 500ms, so a group that followed config order or dialled all caches goes red. The failover subtest asserts that when the preferred cache refuses, the next in preference order (snaps[1].Synced) carries the load, and snapshots are held most preferred first. The 'or set of caches' alternative is a permission, not a forbidden behaviour.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L32) | unit/verify | unproven |
| positive | [`TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L31) | unit/verify | unproven |

### [`RFC8210-8.1-1`](#rfc8210-8.1-1)

To limit the length of time a cache must keep the data necessary to generate incremental updates, a router MUST send either a Serial Query or a Reset Query periodically. (§8.1, §8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a router that never re-polls. TestRunPollsOnRefreshAfterSuccessRetryAfterFailure (cacheGroup.Run against a real listener, refresh 1s, retry 7200s) waits for a second accept (waitAccept fails the test if none arrives) and asserts it comes within 30s, and that the queries are a Reset Query then a Serial Query. Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRunPollsOnRefreshAfterSuccessRetryAfterFailure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_poll_test.go#L25) | unit/verify | unproven |

### [`RFC8210-8.3-1`](#rfc8210-8.3-1)

Regardless of how this state arose, the cache replies with a Cache Reset to tell the router that it cannot honor the request. When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache. (§8.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: after a Cache Reset with no more-preferred cache, not issuing a Reset Query or not taking a whole new load. The tagged units in TestCacheResetTriggersResetQuery call handlePDU directly and assert only that s.serial becomes 0 and the sync ends with errRtrCacheResetReceivedWillDo; no tagged unit observes the Reset Query on the wire or the replacement of the old set by a full load, and the 'no more-preferred caches' condition is not built (single session, no cache group). The tag prose says ze runs every configured cache in parallel, which the preference-ordered cacheGroup (rtr_preference_test.go) contradicts. The widened quote adds the SHOULD sentence as context for the condition; this row judges only its MUST.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L320) | unit/verify | unproven |
| positive | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L304) | unit/verify | unproven |

### [`RFC8210-12-1`](#rfc8210-12-1)

Errors which are considered fatal MUST cause the session to be dropped. (§12)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive drives a fatal Error Report 4 to TCP closure; negative only calls isFatalError(errNoDataAvail) and never shows a non-fatal error keeps the session, so a session that dropped on every error still passes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsFatalError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L211) | unit/verify | unproven |
| positive | [`TestHandlePDUVersionDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L83) | unit/verify | unproven |

### [`RFC8210-8.2-1`](#rfc8210-8.2-1)

The cache MUST rate-limit Serial Notifies to no more frequently than one per minute. (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-8.2-1, so no unit is bound to it.

### [`RFC8210-9-1`](#rfc8210-9-1)

Caches and routers MUST implement unprotected transport over TCP using a port, rpki-rtr (323); see Section 14. (§9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the only tagged unit asserts the parsed default port is 323; no tagged unit drives an unprotected TCP RTR session from config

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L88) | unit/verify | unproven |

### [`RFC8210-9-2`](#rfc8210-9-2)

If unprotected TCP is the transport, the cache and routers MUST be on the same trusted and controlled network. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9-2, so no unit is bound to it.

### [`RFC8210-9-3`](#rfc8210-9-3)

If available to the operator, caches and routers MUST use one of the following more protected protocols: (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using an unprotected stream when the operator has configured a protected transport. TestRTRTLSAuthenticatedCache asserts the RTR exchange completes over TLS 1.2 and 1.3 (observed state.Version, queryType, published VRP and ASPA); negative TestRTRTLSNoPlaintextFallback offers a plaintext RTR server at the TLS address and assertRTRTLSRefused asserts syncOnce errors, zero RTR query bytes reach the peer on any connection, and nothing is published, so a plaintext fallback goes red. Availability is the operator's configuration; the refusal of a cache with neither tls nor trusted-network sits in untagged TestRTRTLSConfigRejectsUnsafeTransport and is RFC8210-3-1's concern.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSNoPlaintextFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L182) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L34) | unit/verify | unproven |

### [`RFC8210-9.1-1`](#rfc8210-9.1-1)

Cache servers supporting SSH transport MUST accept RSA authentication (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.1-1, so no unit is bound to it.

### [`RFC8210-9.1-2`](#rfc8210-9.1-2)

User authentication MUST be supported (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.1-2, so no unit is bound to it.

### [`RFC8210-9.2-1`](#rfc8210-9.2-1)

Client routers using TLS transport MUST present client-side certificates to authenticate themselves to the cache in order to allow the cache to manage the load by rejecting connections from unauthorized routers. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a TLS session in which the router presents no client certificate, or not the configured one. The fixture server uses RequireAndVerifyClientCert, and TestRTRTLSAuthenticatedCache asserts the cache received the configured leaf (bytes.Equal chain[0].Raw) and both intermediates, so a router sending no certificate fails the handshake (t.Fatalf on syncOnce) and one sending another goes red. Negative TestRTRTLSRejectsUnusableClientIdentity: missing CA, missing certificate, missing or mismatched key, wrong EKU and missing IP SAN all go through assertRTRTLSRefused, with a working alternate identity present so a silent substitution goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L136) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L35) | unit/verify | unproven |

### [`RFC8210-9.2-2`](#rfc8210-9.2-2)

Certificates used to authenticate client routers in this protocol MUST include a subjectAltName extension [RFC5280] containing one or more iPAddress identities (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating to the cache with a router certificate that carries no iPAddress SAN. Negative TestRTRTLSRejectsUnusableClientIdentity "missing IP SAN" installs a client-auth certificate without an IP SAN and asserts assertRTRTLSRefused (syncOnce error, zero query bytes); the fixture server does not check IP SANs, so the refusal is Ze's and a Ze that sent the certificate would sync and go red. Positive TestRTRTLSAuthenticatedCache presents the fixture router certificate (issued with an iPAddress SAN) and asserts it is the one the cache received; it does not assert the SAN content itself.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnusableClientIdentity`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L137) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L36) | unit/verify | unproven |

### [`RFC8210-9.2-3`](#rfc8210-9.2-3)

when authenticating the router's certificate, the cache MUST check the IP address of the TLS connection against these iPAddress identities (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-3, so no unit is bound to it.

### [`RFC8210-9.2-4`](#rfc8210-9.2-4)

Routers MUST also verify the cache's TLS server certificate, using subjectAltName dNSName identities as described in [RFC6125], to avoid MITM attacks. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a cache certificate without verifying it against the dNSName reference. Negative TestRTRTLSRejectsUnauthenticatedCaches "wrong DNS" (server-name other.rtr.test) and "CN only" (no dNSName SAN) both go through assertRTRTLSRefused: syncOnce error, zero query bytes, nothing published, so a router that skipped name verification goes red. Positive TestRTRTLSAuthenticatedCache: a trusted certificate whose dNSName matches syncs and publishes VRP and ASPA.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L94) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L37) | unit/verify | unproven |

### [`RFC8210-9.2-5`](#rfc8210-9.2-5)

rpki-rtr implementations which use TLS MUST NOT use Common Name (CN-ID) identifiers (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating a cache by its Common Name. Negative TestRTRTLSRejectsUnauthenticatedCaches "CN only" issues a server certificate whose CN matches the reference and has no DNS SAN, and assertRTRTLSRefused asserts it is refused, so a CN fallback goes red. Positive TestRTRTLSAuthenticatedCache: a certificate whose CN differs from the reference but whose DNS-ID matches authenticates, so a CN check that overrode DNS-ID goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L95) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L38) | unit/verify | unproven |

### [`RFC8210-9.2-6`](#rfc8210-9.2-6)

the DNS-ID identifier type MUST be present in rpki-rtr server certificates. (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-6, so no unit is bound to it.

### [`RFC8210-9.3-1`](#rfc8210-9.3-1)

If TCP MD5 is used, implementations MUST support key lengths of at least 80 printable ASCII bytes, per Section 4.5 of [RFC2385]. (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.3-1, so no unit is bound to it.

### [`RFC8210-9.4-1`](#rfc8210-9.4-1)

Implementations MUST support key lengths of at least 80 printable ASCII bytes. Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-1, so no unit is bound to it.

### [`RFC8210-10-1`](#rfc8210-10-1)

If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation of BGP announcements. (§10)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. two ApplyDelta batches on one ROACache stand in for two caches; no two sessions are driven, and the negative (origin mismatch is Invalid) violates RFC 6811 origin validation, not source distinction

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L162) | unit/verify | unproven |
| positive | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L154) | unit/verify | unproven |

### [`RFC8210-4-2`](#rfc8210-4-2)

As a cache server must evaluate certificates and ROAs (Route Origin Authorizations; see [RFC6480]), which are time dependent, servers' clocks MUST be correct to a tolerance of approximately an hour. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-4-2, so no unit is bound to it.

### [`RFC8210-4-3`](#rfc8210-4-3)

Note that the Serial Number comparison used to determine "since the given Serial Number" MUST take wrap-around into account; see [RFC1982]. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-4-3, so no unit is bound to it.

### [`RFC8210-7-7`](#rfc8210-7-7)

If either party receives a PDU containing an unrecognized Protocol Version (neither 0 nor 1) during this negotiation, it MUST either downgrade to a known version or terminate the connection, with an Error Report PDU unless the received PDU is itself an Error Report PDU. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: accepting a PDU with an unrecognized version, or terminating silently, or answering an unknown-version Error Report with an Error Report. TestRTRUnknownNegotiationVersion asserts versions 0 and 99 yield errRtrUnsupportedProtocol, an Error Report PDU with code 4 (got.report[1], got.report[2:4]) and EOF, and version 2 completes. The exception clause 'unless the received PDU is itself an Error Report PDU' has no assertion in the tagged unit (it is asserted only in TestRTRErrorReportIsNotAnswered, which carries RFC8210-5.11-1, not this id).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L225) | unit/verify | unproven |
| positive | [`TestRTRUnknownNegotiationVersion`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L224) | unit/verify | unproven |

### [`RFC8210-7-8`](#rfc8210-7-8)

The router MUST ignore any Serial Notify PDUs it might receive from the cache during this initial startup period, regardless of the Protocol Version field in the Serial Notify PDU. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a startup Serial Notify erroring the session or adopting its serial/Session ID, for ANY Protocol Version in the notify. TestSerialNotifyIgnoredDuringStartup asserts no error, done=false, serial and sessionID unchanged, but only for a notify at rtrVersionMax, called on handlePDU directly. The clause 'regardless of the Protocol Version field' is unasserted: deleting the hdr.Type != pduSerialNotify exemption from the version check in handlePDU (rtr_session.go) leaves both subtests green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L454) | unit/verify | unproven |
| positive | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L427) | unit/verify | unproven |

### [`RFC8210-9.2-8`](#rfc8210-9.2-8)

o The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: using a reference identifier other than the cache DNS name (the connected IP, or any name the certificate carries). Negative TestRTRTLSRejectsUnauthenticatedCaches: 'wrong DNS' (server-name other.rtr.test vs cert cache.rtr.test) and 'IP SAN only' (connect to 127.0.0.1, cert has only that IP SAN) both go red via assertRTRTLSRefused (syncOnce error, zero query bytes, nothing published). Positive TestRTRTLSDNSAddressIsTheReference: omitted server-name uses the configured cache DNS name, asserting state.ServerName == localhost and a published VRP.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L97) | unit/verify | unproven |
| positive | [`TestRTRTLSDNSAddressIsTheReference`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L221) | unit/verify | unproven |

### [`RFC8210-10-2`](#rfc8210-10-2)

A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data. (§10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an update from one cache applied to another cache's data (unmarked source). TestRTRCacheSwitchKeepsSerialBasesSeparate gives both caches the same Session ID and serial; after switching 2->3->2 it asserts the query sequence ends {2, pduResetQuery} (not a Serial Query against the other cache's base) and that only the selected cache's prefix and ASPA provider validate (roas.Validate / aspas.isProvider per candidate), so a merge or reuse goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L126) | unit/verify | unproven |
| positive | [`TestRTRCacheSwitchKeepsSerialBasesSeparate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_preference_test.go#L125) | unit/verify | unproven |

### [`RFC8210-9.3-2`](#rfc8210-9.3-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.3-2, so no unit is bound to it.

### [`RFC8210-9.4-2`](#rfc8210-9.4-2)

Implementations MUST also support hexadecimal sequences of at least 32 characters, i.e., 128 bits. (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-2, so no unit is bound to it.

### [`RFC8210-9.4-3`](#rfc8210-9.4-3)

The cryptographic algorithms and associated parameters described in [RFC5926] MUST be supported. (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-3, so no unit is bound to it.

### [`RFC8210-9.2-9`](#rfc8210-9.2-9)

Certification authorities which issue rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-9, so no unit is bound to it.

### [`RFC8210-9.2-10`](#rfc8210-9.2-10)

a CN field may be present in the server certificate's subject name but MUST NOT be used for authentication within the rules described in [RFC6125]. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: authenticating a cache by its CN. Negative TestRTRTLSRejectsUnauthenticatedCaches 'CN only' (CN cache.rtr.test, no DNS SAN) is refused via assertRTRTLSRefused. Positive TestRTRTLSAuthenticatedCache: a cert whose CN is not-the-cache-reference.invalid but whose DNS-ID matches authenticates, so a CN check is not used either way.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L96) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L39) | unit/verify | unproven |

### [`RFC8210-8.1-2`](#rfc8210-8.1-2)

When a transport connection is first established, the router MUST send either a Reset Query or a Serial Query. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a new transport on which the router sends nothing first, or something other than a Reset/Serial Query. TestFirstPDUOnConnectionIsAQuery requires io.ReadFull of the first PDU to succeed and asserts buf[1] == pduResetQuery (fresh) and == pduSerialQuery (resuming). Row carries {single-polarity: positive}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L516) | unit/verify | unproven |

### [`RFC8210-8.4-1`](#rfc8210-8.4-1)

When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order. If no other caches are available, the router MUST issue periodic Reset Queries until it gets a new usable load from the cache. (§8.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after No Data Available with no other cache, stopping, sending Serial Queries, or not repeating the Reset Query until a usable load arrives. TestNoDataAvailableKeepsResetQueryMode runs a single-session cacheGroup (no other cache), the cache answers two queries with Error Report code 2, and the test asserts three consecutive queries are pduResetQuery each within 4s at a 1s retry (a router that stopped hits t.Fatal, one that sent a Serial Query fails assert.Equal); the negative asserts that once a complete load is published the next query is pduSerialQuery, which pins the 'until it gets a new usable load' bound. The widened quote adds the SHOULD failover sentence only as the condition's context; this row judges its MUST.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L338) | unit/verify | unproven |
| positive | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L337) | unit/verify | unproven |

### [`RFC8210-2-1`](#rfc8210-2-1)

While a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-2-1, so no unit is bound to it.

### [`RFC8210-3-1`](#rfc8210-3-1)

A Relying Party, e.g., router or other client, MUST have a trust relationship with, and a trusted transport channel to, any cache(s) it uses. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: using a cache with no trust relationship or over an untrusted channel. TLS path is asserted: TestRTRTLSRejectsUnauthenticatedCaches 'untrusted issuer' and 'no client authentication' are refused (assertRTRTLSRefused), TestRTRTLSAuthenticatedCache succeeds with the named CA. The unprotected-TCP path, where the trusted channel is the operator's trusted-network declaration, has no assertion in a tagged unit: the refusal of a cache with neither tls nor trusted-network is in TestRTRTLSConfigRejectsUnsafeTransport, which does not carry this id.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L93) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L33) | unit/verify | unproven |

### [`RFC8210-8.1-3`](#rfc8210-8.1-3)

In all other cases, the router lacks the necessary data for fast resynchronization and therefore MUST fall back to a Reset Query. (§8.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a Serial Query when the router lacks the data for fast resynchronization, i.e. data expired, a different cache, or no remembered Session ID. TestRFC8210ResetQueryFallback asserts only the Cache Reset case (buf[1] == pduResetQuery) and the held-serial negative. The expired-data case (rtr_expire.go clears serial), the cache-switch case (startFullSync) and the version-downgrade case have no assertion in the tagged unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/resync_rfc8210_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRFC8210ResetQueryFallback`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/resync_rfc8210_test.go#L67) | unit/verify | revert, verified |

### [`RFC8210-9.2-11`](#rfc8210-9.2-11)

o Support for the DNS-ID identifier type (that is, the dNSName identity in the subjectAltName extension) is REQUIRED in rpki-rtr server and client implementations which use TLS. (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a TLS client that cannot authenticate the cache by DNS-ID (dNSName SAN). Positive TestRTRTLSAuthenticatedCache: a DNS-ID cert with a differing CN syncs and publishes VRP and ASPA. Negative TestRTRTLSRejectsUnauthenticatedCaches: wrong DNS, CN only, IP SAN only are refused (assertRTRTLSRefused). The server-implementation clause binds a role Ze does not fill: Ze has no RTR cache server (only rtr_session.go and rtr_pdu.go, both client-side).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRTRTLSRejectsUnauthenticatedCaches`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L92) | unit/verify | unproven |
| positive | [`TestRTRTLSAuthenticatedCache`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_tls_test.go#L32) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8210.txt |
| Source fingerprint | d55f8b529fb430bf |
| Record | rfc/extraction/rfc8210.json |
| Mapped sentences | 56 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 4 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `5.3` | not stated | 2 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `5.5` | not stated | 1 | walked | not stated |
| `5.6` | not stated | 1 | walked | not stated |
| `5.7` | not stated | 0 | walked | not stated |
| `5.8` | not stated | 1 | walked | not stated |
| `5.9` | not stated | 0 | walked | not stated |
| `5.10` | not stated | 2 | walked | not stated |
| `5.11` | not stated | 4 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 7 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 3 | walked | not stated |
| `8.2` | not stated | 2 | walked | not stated |
| `8.3` | not stated | 1 | walked | not stated |
| `8.4` | not stated | 1 | walked | not stated |
| `9` | not stated | 3 | walked | not stated |
| `9.1` | not stated | 2 | walked | not stated |
| `9.2` | not stated | 7 | walked | not stated |
| `9.3` | not stated | 2 | walked | not stated |
| `9.4` | not stated | 4 | walked | not stated |
| `10` | not stated | 2 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 1 | walked | not stated |
| `13` | not stated | 1 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |
| `15.1` | not stated | 0 | walked | not stated |
| `15.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the ignore-Serial-Notify-before-negotiation obligation already mapped at site 7:5, adding only the backwards-compatibility reason | Routers, however, MUST handle such notifications (by ignoring them) for backwards compatibility with caches serving protocol version 0. |
| `8.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the periodic Serial-or-Reset Query obligation already mapped at site 8.1:3, here for the old-withdraw case | To limit the length of time a cache must keep old withdraws, a router MUST send either a Serial Query or a Reset Query periodically. |
| `9.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the MAC-length half of the TCP-AO support obligation the row already states: "key lengths of at least 80 printable ASCII bytes and MAC lengths of at least 96 bits" | Message Authentication Code (MAC) lengths of at least 96 bits MUST be supported, per Section 5.1 of [RFC5925]. |
| `13:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Security Considerations restatement of the section 9 rule that an unprotected TCP transport puts the router and cache on the same trusted, controlled network | Protocols which provide integrity and authenticity SHOULD be used, and if they cannot, i.e., TCP is used as the transport, the router and cache MUST be on the same trusted, controlled network. |

## Superseded

No document obsoletes RFC 8210, so its obligations are stated where they were written.
