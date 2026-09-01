# RFC 8210 - The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1

Partial. Every requirement this repository extracted from RFC 8210, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 8 of 24 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 4 of 24 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 24 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 21 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 56 | of 80 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 32 | of 56 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 50.0% | 12 of 24 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 24 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 80 |
| Gated MUST-level | 56 |
| Obligations that bind Ze | 24 |
| Not applicable, so out of scope | 32 |
| Declared gaps | 12 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 23 |
| Tagged units | 21 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8210.md` |
| Requirement shard | `rfc/requirements/rfc8210.md` |
| RFC text | `rfc/full/rfc8210.txt` |

## Enrolment

Enrolled: RPKI to Router Protocol, Version 1

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

ze is the RTR router (client). It opens every connection with a Reset or Serial Query ([`internal/component/bgp/plugins/rpki/rtr_session.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session.go)), speaks version 2 with a downgrade to version 1 (rtr_pdu.go, rtr_session.go), parses IPv4/IPv6 Prefix, Cache Response, the 24-byte v1 End of Data with its Refresh/Retry/Expire parameters, Cache Reset, Serial Notify and Error Report PDUs, enforces Max Length >= Prefix Length and ignores reserved Flags bits on receipt (rtr_pdu.go), applies each delta atomically at End of Data (rtr_session.go), ignores Serial Notify during startup, re-issues a Reset Query on Cache Reset and on No Data Available, and validates origins against the merged VRP cache (validate.go).

**What the ledger says remains**

Twelve MUST-level gaps, each annotated in [`rfc/short/rfc8210.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8210.md): [`RFC8210-5.1-3`](#rfc8210-5.1-3), [`RFC8210-5.1-4`](#rfc8210-5.1-4) -- no Session ID mismatch detection, no Error Report code 0, no cache flush (rtr_session.go adopts the Session ID unconditionally); [`RFC8210-5.1-5`](#rfc8210-5.1-5) -- a version downgrade reuses the previous version's Session ID and serial (rtr_session.go); [`RFC8210-7-3`](#rfc8210-7-3), [`RFC8210-7-4`](#rfc8210-7-4), [`RFC8210-7-7`](#rfc8210-7-7) -- handlePDU never reads hdr.Version (rtr_session.go), so no received-version check, downgrade or session drop happens; [`RFC8210-12-1`](#rfc8210-12-1) -- Error Code 4 is excluded from isFatalError (rtr_pdu.go) and at rtrVersionMin the session survives it (rtr_session.go); [`RFC8210-5.10-2`](#rfc8210-5.10-2) -- Router Key PDUs are discarded (rtr_session.go), so no Subject Public Key comparison exists; [`RFC8210-6-2`](#rfc8210-6-2) -- VRPs outlive the Expire Interval (ROACache.Clear, roa_cache.go, has no production caller); [`RFC8210-4-1`](#rfc8210-4-1) -- no most-preferred-cache selection (rpki.go runs every configured cache concurrently); [`RFC8210-10-2`](#rfc8210-10-2) -- VRPs are not marked by source cache (rpki.go, roa_cache.go); [`RFC8210-9-3`](#rfc8210-9-3) -- no protected RTR transport (rtr_session.go dials only unprotected TCP).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 48 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **56** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC8210-5-1`](#rfc8210-5-1), [`RFC8210-5.1-1`](#rfc8210-5.1-1), [`RFC8210-5.1-2`](#rfc8210-5.1-2), [`RFC8210-5.2-1`](#rfc8210-5.2-1), [`RFC8210-8.3-1`](#rfc8210-8.3-1), [`RFC8210-10-1`](#rfc8210-10-1), [`RFC8210-7-8`](#rfc8210-7-8), [`RFC8210-8.4-1`](#rfc8210-8.4-1)

**Annotated instead of tested (48):** [`RFC8210-5.1-3`](#rfc8210-5.1-3), [`RFC8210-5.1-4`](#rfc8210-5.1-4), [`RFC8210-5.1-5`](#rfc8210-5.1-5), [`RFC8210-5.3-1`](#rfc8210-5.3-1), [`RFC8210-5.3-2`](#rfc8210-5.3-2), [`RFC8210-5.5-1`](#rfc8210-5.5-1), [`RFC8210-5.6-1`](#rfc8210-5.6-1), [`RFC8210-5.10-1`](#rfc8210-5.10-1), [`RFC8210-5.10-2`](#rfc8210-5.10-2), [`RFC8210-5.8-1`](#rfc8210-5.8-1), [`RFC8210-5.11-1`](#rfc8210-5.11-1), [`RFC8210-5.11-2`](#rfc8210-5.11-2), [`RFC8210-5.11-3`](#rfc8210-5.11-3), [`RFC8210-5.11-4`](#rfc8210-5.11-4), [`RFC8210-6-1`](#rfc8210-6-1), [`RFC8210-6-2`](#rfc8210-6-2), [`RFC8210-7-1`](#rfc8210-7-1), [`RFC8210-7-2`](#rfc8210-7-2), [`RFC8210-7-3`](#rfc8210-7-3), [`RFC8210-7-4`](#rfc8210-7-4), [`RFC8210-4-1`](#rfc8210-4-1), [`RFC8210-8.1-1`](#rfc8210-8.1-1), [`RFC8210-12-1`](#rfc8210-12-1), [`RFC8210-8.2-1`](#rfc8210-8.2-1), [`RFC8210-9-1`](#rfc8210-9-1), [`RFC8210-9-2`](#rfc8210-9-2), [`RFC8210-9-3`](#rfc8210-9-3), [`RFC8210-9.1-1`](#rfc8210-9.1-1), [`RFC8210-9.1-2`](#rfc8210-9.1-2), [`RFC8210-9.2-1`](#rfc8210-9.2-1), [`RFC8210-9.2-2`](#rfc8210-9.2-2), [`RFC8210-9.2-3`](#rfc8210-9.2-3), [`RFC8210-9.2-4`](#rfc8210-9.2-4), [`RFC8210-9.2-5`](#rfc8210-9.2-5), [`RFC8210-9.2-6`](#rfc8210-9.2-6), [`RFC8210-9.3-1`](#rfc8210-9.3-1), [`RFC8210-9.4-1`](#rfc8210-9.4-1), [`RFC8210-4-2`](#rfc8210-4-2), [`RFC8210-4-3`](#rfc8210-4-3), [`RFC8210-7-7`](#rfc8210-7-7), [`RFC8210-9.2-8`](#rfc8210-9.2-8), [`RFC8210-10-2`](#rfc8210-10-2), [`RFC8210-9.3-2`](#rfc8210-9.3-2), [`RFC8210-9.4-2`](#rfc8210-9.4-2), [`RFC8210-9.4-3`](#rfc8210-9.4-3), [`RFC8210-9.2-9`](#rfc8210-9.2-9), [`RFC8210-9.2-10`](#rfc8210-9.2-10), [`RFC8210-8.1-2`](#rfc8210-8.1-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8210-5-1` | Reserved fields must be zero on transmission and ignored on receipt (§5) | MUST | 5 | **positive:** `unit/verify` [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L19). **positive:** `unit/verify` [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L55). **negative:** `unit/verify` [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L39) |
| `RFC8210-5.1-1` | Flags field bits other than bit 0 must be zero on transmission and ignored on receipt (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L424). **negative:** `unit/verify` [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L433) |
| `RFC8210-5.1-2` | Max Length must not be less than Prefix Length in IPv4/IPv6 Prefix PDUs (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L59). **negative:** `unit/verify` [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L132) |
| `RFC8210-5.1-3` | Party detecting Session ID mismatch must immediately terminate with Error Report PDU code 0 (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze detects no Session ID mismatch -- handlePDU adopts the Cache Response Session ID unconditionally (internal/component/bgp/plugins/rpki/rtr_session.go:233, s.sessionID = hdr.SessionID) with no comparison against the prior value, and the plugin has no Error Report writer to emit code 0 |
| `RFC8210-5.1-4` | Router must flush all data learned from a cache on Session ID mismatch (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no cache flush on a Session ID change -- the mismatch is never detected (internal/component/bgp/plugins/rpki/rtr_session.go:233) and ROACache.Clear (internal/component/bgp/plugins/rpki/roa_cache.go:317) has no production caller, so VRPs learned under the old Session ID stay in the cache |
| `RFC8210-5.1-5` | Routers must treat sessions with different Protocol Version fields as separate sessions even if same Session ID (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** sessions are not separated by protocol version -- on a version downgrade Run reconnects without clearing sessionID or serial (internal/component/bgp/plugins/rpki/rtr_session.go:96-99), so connectAndSync re-sends the previous version's Session ID and Serial Number in the new version's Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:172) |
| `RFC8210-5.3-1` | Cache must return the minimum set of changes needed to bring router into sync on Serial Query reply (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side response obligation -- ze is the RTR router: it parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and writes only Reset and Serial Queries (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, 103), so it computes and sends no change set |
| `RFC8210-5.3-2` | Cache must merge multiple changes to same prefix/key into simplest possible view (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side response obligation -- ze emits no payload PDUs at all (the only writers are writeResetQuery and writeSerialQuery, internal/component/bgp/plugins/rpki/rtr_pdu.go:92, 103), so it merges no changes for transmission |
| `RFC8210-5.5-1` | Withdraw/announce field in payload PDUs must have value 1 (announce) when replying to Reset Query (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) and has no Prefix PDU writer, so it never sets this field on transmission |
| `RFC8210-5.6-1` | Cache must ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any point in time (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission guarantee -- ze only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| `RFC8210-5.10-1` | Cache must ensure one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any point in time (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission guarantee -- ze has no Router Key PDU writer; the only Router Key code is the discard arm in handlePDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379), so it emits no Router Key PDU whose uniqueness it would have to ensure |
| `RFC8210-5.10-2` | Implementations must compare Subject Public Key values as well as SKIs when detecting duplicate PDUs (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs no Router Key duplicate detection -- handlePDU discards every Router Key PDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379) and stores neither SKI nor Subject Public Key, so no Subject Public Key comparison happens anywhere |
| `RFC8210-5.8-1` | End of Data Session ID and Protocol Version must match the corresponding Cache Response (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads End of Data for its serial and timing parameters only (internal/component/bgp/plugins/rpki/rtr_session.go:291-306) and emits neither Cache Response nor End of Data PDUs whose Session ID and version it would have to match |
| `RFC8210-5.11-1` | Error Report PDU must not be sent for an Error Report PDU (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze only reads received Error Reports (internal/component/bgp/plugins/rpki/rtr_session.go:363) and has no Error Report writer, so it can never emit an Error Report about an Error Report |
| `RFC8210-5.11-2` | Erroneous PDU field must be empty and Length of Encapsulated PDU must be zero for generic errors (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes the Erroneous PDU field or its length |
| `RFC8210-5.11-3` | Error text, if present, must be UTF-8 encoded (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes error text |
| `RFC8210-5.11-4` | Length of Error Text field must be zero if no diagnostic text present (§5.11) | MUST | 5.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never sets Length of Error Text |
| `RFC8210-6-1` | Caches must set Expire Interval to a value larger than either Refresh Interval or Retry Interval (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side timing obligation -- ze is the router: it adopts whatever intervals the End of Data PDU carries (internal/component/bgp/plugins/rpki/rtr_session.go:298-306) and publishes none of its own, so it sets no Expire Interval |
| `RFC8210-6-2` | Router must not retain data past the time indicated by Expire Interval (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze retains VRPs past the Expire Interval -- expireInterval only sets the socket read deadline (internal/component/bgp/plugins/rpki/rtr_session.go:188) and Run simply reconnects after retryInterval (internal/component/bgp/plugins/rpki/rtr_session.go:106-110); ROACache.Clear (internal/component/bgp/plugins/rpki/roa_cache.go:317) has no production caller, so cached VRPs outlive an expired session |
| `RFC8210-7-1` | Router must start each transport connection by issuing either a Reset Query or Serial Query (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L307). **negative:** no negative test. **{single-polarity}:** connectAndSync writes a Reset Query or a Serial Query as the first bytes of every connection (internal/component/bgp/plugins/rpki/rtr_session.go:165-176) before entering readLoop, and no received input can make the router open a connection without a query, so there is no negative case |
| `RFC8210-7-2` | Cache receiving v0 query must downgrade to v0 or send v1 Error Report with Error Code 4 and terminate (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side obligation -- ze runs no RTR cache server; RTRSession dials out to a configured cache (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync) and receives no queries to downgrade or reject |
| `RFC8210-7-3` | Router receiving v0 response must either downgrade to v0 or terminate the connection (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never inspects a received PDU's Protocol Version -- handlePDU switches on hdr.Type alone (internal/component/bgp/plugins/rpki/rtr_session.go:229-383) and reads hdr.Version nowhere, so a version 0 response is processed as if it matched the negotiated version; ze also cannot downgrade below version 1 (rtrVersionMin, internal/component/bgp/plugins/rpki/rtr_pdu.go:16) |
| `RFC8210-5.2-1` | Router must ignore any Serial Notify PDUs received during version negotiation startup period (§5.2, §7) | MUST | 5.2 | **positive:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L218). **negative:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L244) |
| `RFC8210-7-4` | Party receiving PDU for different Protocol Version after negotiation must drop the session (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no post-negotiation version check -- handlePDU compares no received PDU version against the negotiated s.version (internal/component/bgp/plugins/rpki/rtr_session.go:229-383); the one version-sensitive guard (internal/component/bgp/plugins/rpki/rtr_session.go:271) keys off the ASPA PDU type, not the header version field |
| `RFC8210-4-1` | Router must choose the most preferred cache by configuration (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze selects no most-preferred cache -- startSessions launches a Run goroutine for every configured cache concurrently (internal/component/bgp/plugins/rpki/rpki.go:291-298) and the parsed preference is only rendered for display (internal/component/bgp/plugins/rpki/rpki.go:1026) |
| `RFC8210-8.1-1` | Router must send either a Serial Query or Reset Query periodically (§8.1, §8.2) | MUST | 8.1 | **positive:** `unit/verify` [`TestPollingCadenceAtLeastHourly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L90). **negative:** no negative test. **{single-polarity}:** the poll cadence is Run's post-sync wait of s.retryInterval (internal/component/bgp/plugins/rpki/rtr_session.go:106-110) followed by a fresh connectAndSync query, seeded at 600s (internal/component/bgp/plugins/rpki/rtr_session.go:81); no received input produces a non-conformant never-poll case |
| `RFC8210-8.3-1` | Router must issue Reset Query and get entire new load after Cache Reset when no preferred caches available (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L116). **negative:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L132) |
| `RFC8210-12-1` | Errors which are considered fatal must cause the session to be dropped (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Error Code 4 is fatal per RFC 8210 Section 12 but isFatalError excludes it (internal/component/bgp/plugins/rpki/rtr_pdu.go:167-169); once the session already sits at rtrVersionMin the downgrade branch is skipped and handlePDU logs the code and keeps the session alive (internal/component/bgp/plugins/rpki/rtr_session.go:366-375), so this fatal error drops no session |
| `RFC8210-8.2-1` | Cache must rate-limit Serial Notifies to no more frequently than one per minute (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it (internal/component/bgp/plugins/rpki/rtr_session.go:359-361) and has no Serial Notify writer |
| `RFC8210-9-1` | Caches and routers must implement unprotected transport over TCP using port rpki-rtr (323) (§9) | MUST | 9 | **positive:** `unit/verify` [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L96). **negative:** no negative test. **{single-polarity}:** ze implements the unprotected TCP transport by dialing tcp (internal/component/bgp/plugins/rpki/rtr_session.go:127) and defaults each cache server to port 323 (internal/component/bgp/plugins/rpki/rpki_config.go:174); it never declines to offer unprotected TCP, so there is no negative case |
| `RFC8210-9-2` | Cache and routers must be on the same trusted and controlled network when using unprotected TCP (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** operator deployment obligation -- ze dials plain TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) with no code path that can observe or enforce network topology |
| `RFC8210-9-3` | Caches and routers must use one of the protected protocols when available (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no protected RTR transport -- connectAndSync dials only unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and the package contains no SSH, TLS, TCP-MD5 or TCP-AO client, so no protected protocol is available to select |
| `RFC8210-9.1-1` | Cache servers supporting SSH must accept RSA authentication (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server SSH obligation -- ze runs no RTR SSH server and no SSH client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it performs no SSH authentication |
| `RFC8210-9.1-2` | SSH user authentication must be supported (§9.1) | MUST | 9.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** SSH-transport obligation -- ze implements no SSH RTR transport (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so SSH user authentication has no code path |
| `RFC8210-9.2-1` | Client routers using TLS must present client-side certificates (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it presents no client certificate |
| `RFC8210-9.2-2` | TLS client certificates must include subjectAltName with iPAddress identities (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds and presents no rpki-rtr client certificate |
| `RFC8210-9.2-3` | Cache must check TLS client IP against iPAddress identities (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side TLS obligation -- ze runs no RTR TLS server (it dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting client IP against iPAddress identities |
| `RFC8210-9.2-4` | Routers must verify cache TLS server certificate using subjectAltName dNSName per RFC 6125 (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it verifies no cache server certificate |
| `RFC8210-9.2-5` | TLS implementations must not use Common Name (CN-ID) for authentication (§9.2) | MUST NOT | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP), so it uses no certificate identifiers, CN-ID or otherwise |
| `RFC8210-9.2-6` | DNS-ID identifier type must be present in rpki-rtr server certificates (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server-certificate obligation under TLS -- ze runs no RTR TLS server and issues no server certificate (it is a plain-TCP client, internal/component/bgp/plugins/rpki/rtr_session.go:127) |
| `RFC8210-9.3-1` | TCP MD5 implementations must support key lengths of at least 80 printable ASCII bytes (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no MD5 keys |
| `RFC8210-9.4-1` | TCP-AO implementations must support key lengths of at least 80 printable ASCII bytes and MAC lengths of at least 96 bits (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no AO keys and negotiates no MAC length |
| `RFC8210-10-1` | Data from multiple caches must not be distinguished between when performing BGP validation (§10) | MUST | 10 | **positive:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L154). **negative:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L162) |
| `RFC8210-4-2` | Servers' clocks must be correct to a tolerance of approximately an hour (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** server-clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache server and holds no clock this binds |
| `RFC8210-4-3` | Serial Number comparison must take wrap-around into account per RFC 1982 (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze performs no Serial Number ordering comparison -- it stores the serial the cache reports (internal/component/bgp/plugins/rpki/rtr_session.go:297) and tests only serial == 0 to choose between a Reset Query and a Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:166); an equality test against zero is unaffected by wrap-around, and grep over the package finds no other serial comparison |
| `RFC8210-5.1-6` | Cache servers should not use the same Session ID across multiple protocol versions (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.6-2` | Router should raise Duplicate Announcement Received error on duplicate active record (§5.6, §5.10) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-5` | Erroneous Error Report PDU session should be dropped (§5.11) | SHOULD | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-3` | Router should not poll the cache sooner than indicated by Refresh Interval (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-4` | Router should not retry sooner than indicated by Retry Interval (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-5` | Caches should not send Serial Notify PDUs before version negotiation completes (§7) | SHOULD NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.3-2` | Router should attempt to connect to more-preferred caches on Cache Reset (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-4` | Both caches and routers should enable keep-alives when available (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-5` | Caches and routers should use TCP-AO transport (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-3` | Cache servers supporting SSH should accept ECDSA authentication (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.1-4` | Client routers should verify the public key of the cache (§9.1) | SHOULD | 9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.2-7` | DNS names in rpki-rtr server certificates should not contain wildcard "*" (§9.2) | SHOULD NOT | 9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.2-2` | Router may issue an immediate Serial Query or Reset Query upon receipt of Serial Notify (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-5.11-6` | Erroneous PDU field may be truncated for excessively long PDUs (§5.11) | MAY | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-6` | Router may retry connection using protocol version 0 on v0 cache termination (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-6-5` | Router may send first Serial Query or Reset Query immediately after version downgrade (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9-6` | Caches and routers may use SSH, TCP MD5, IPsec, or TLS transport (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-7-7` | If either party receives a PDU with unrecognized Protocol Version during negotiation, it MUST either downgrade to a known version or terminate (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze validates no received Protocol Version -- handlePDU never reads hdr.Version (internal/component/bgp/plugins/rpki/rtr_session.go:229-383), so an unrecognized version in a received PDU triggers neither a downgrade nor a termination; the downgrade at internal/component/bgp/plugins/rpki/rtr_session.go:366-370 fires only on an Error Report carrying code 4 |
| `RFC8210-7-8` | Routers MUST handle Serial Notify PDUs received before version negotiation completes (by ignoring them) (§7) | MUST | 7 | **positive:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L221). **negative:** `unit/verify` [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L248) |
| `RFC8210-9.2-8` | Client router using TLS MUST set its reference identifier to the DNS name of the rpki-rtr cache (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it sets no TLS reference identifier |
| `RFC8210-10-2` | Router MUST keep data from multiple caches marked as to source, as later updates MUST affect the correct data (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** VRPs carry no source marking -- every RTR session writes one shared ROACache (internal/component/bgp/plugins/rpki/rpki.go:292) and vrpEntry holds only MaxLength and ASN (internal/component/bgp/plugins/rpki/roa_cache.go:12-15), so a withdraw from one cache removes a VRP a different cache announced |
| `RFC8210-9.3-2` | TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.3) | MUST | 9.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| `RFC8210-9.4-2` | TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| `RFC8210-9.4-3` | The cryptographic algorithms and associated parameters described in RFC 5926 MUST be supported for TCP-AO (§9.4) | MUST | 9.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |
| `RFC8210-9.2-9` | CAs issuing rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | MUST | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** certification-authority obligation -- ze issues no rpki-rtr server certificates and runs no TLS RTR transport (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP) |
| `RFC8210-9.2-10` | CN field in TLS certificate MUST NOT be used for authentication (§9.2) | MUST NOT | 9.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it reads no certificate CN field for authentication |
| `RFC8210-8.1-2` | When transport first established, router MUST send Reset Query or Serial Query (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L310). **negative:** no negative test. **{single-polarity}:** the first bytes connectAndSync writes on a freshly established transport are a Reset Query when the serial is 0 and a Serial Query otherwise (internal/component/bgp/plugins/rpki/rtr_session.go:165-176); no received input can produce a connection that carries no opening query, so there is no negative case |
| `RFC8210-8.4-1` | If cache cannot supply update and no other caches available, router MUST issue periodic Reset Queries (§8.4) | MUST | 8.4 | **positive:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L165). **negative:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L184) |
| `RFC8210-7-9` | Party dropping session after version mismatch SHOULD send Error Report with error code 8 (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-10-3` | Client SHOULD attempt to maintain at least one set of data regardless of cache changes (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-10-4` | Client switching to new cache SHOULD retain data from previous cache until fully synced (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-9.3-3` | Cache servers supporting TCP MD5 SHOULD support RFC 4808 for key rollover (§9.3) | SHOULD | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.4-2` | Router receiving "No Data Available" error SHOULD attempt to connect to other caches in preference order (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-11-1` | Cache identity SHOULD be verified and authenticated (§11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8210-8.2-2` | Cache SHOULD send Notify PDU with current Serial Number when cache serial changes (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8210-5.1-3`](#rfc8210-5.1-3) Party detecting Session ID mismatch must immediately terminate with Error Report PDU code 0 (§5.1) | {gap}, no test | ze detects no Session ID mismatch -- handlePDU adopts the Cache Response Session ID unconditionally (internal/component/bgp/plugins/rpki/rtr_session.go:233, s.sessionID = hdr.SessionID) with no comparison against the prior value, and the plugin has no Error Report writer to emit code 0 |
| [`RFC8210-5.1-4`](#rfc8210-5.1-4) Router must flush all data learned from a cache on Session ID mismatch (§5.1) | {gap}, no test | no cache flush on a Session ID change -- the mismatch is never detected (internal/component/bgp/plugins/rpki/rtr_session.go:233) and ROACache.Clear (internal/component/bgp/plugins/rpki/roa_cache.go:317) has no production caller, so VRPs learned under the old Session ID stay in the cache |
| [`RFC8210-5.1-5`](#rfc8210-5.1-5) Routers must treat sessions with different Protocol Version fields as separate sessions even if same Session ID (§5.1) | {gap}, no test | sessions are not separated by protocol version -- on a version downgrade Run reconnects without clearing sessionID or serial (internal/component/bgp/plugins/rpki/rtr_session.go:96-99), so connectAndSync re-sends the previous version's Session ID and Serial Number in the new version's Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:172) |
| [`RFC8210-5.3-1`](#rfc8210-5.3-1) Cache must return the minimum set of changes needed to bring router into sync on Serial Query reply (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side response obligation -- ze is the RTR router: it parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and writes only Reset and Serial Queries (internal/component/bgp/plugins/rpki/rtr_pdu.go:92, 103), so it computes and sends no change set |
| [`RFC8210-5.3-2`](#rfc8210-5.3-2) Cache must merge multiple changes to same prefix/key into simplest possible view (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side response obligation -- ze emits no payload PDUs at all (the only writers are writeResetQuery and writeSerialQuery, internal/component/bgp/plugins/rpki/rtr_pdu.go:92, 103), so it merges no changes for transmission |
| [`RFC8210-5.5-1`](#rfc8210-5.5-1) Withdraw/announce field in payload PDUs must have value 1 (announce) when replying to Reset Query (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) and has no Prefix PDU writer, so it never sets this field on transmission |
| [`RFC8210-5.6-1`](#rfc8210-5.6-1) Cache must ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any point in time (§5.6) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission guarantee -- ze only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| [`RFC8210-5.10-1`](#rfc8210-5.10-1) Cache must ensure one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any point in time (§5.10) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission guarantee -- ze has no Router Key PDU writer; the only Router Key code is the discard arm in handlePDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379), so it emits no Router Key PDU whose uniqueness it would have to ensure |
| [`RFC8210-5.10-2`](#rfc8210-5.10-2) Implementations must compare Subject Public Key values as well as SKIs when detecting duplicate PDUs (§5.10) | {gap}, no test | ze performs no Router Key duplicate detection -- handlePDU discards every Router Key PDU (internal/component/bgp/plugins/rpki/rtr_session.go:377-379) and stores neither SKI nor Subject Public Key, so no Subject Public Key comparison happens anywhere |
| [`RFC8210-5.8-1`](#rfc8210-5.8-1) End of Data Session ID and Protocol Version must match the corresponding Cache Response (§5.8) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads End of Data for its serial and timing parameters only (internal/component/bgp/plugins/rpki/rtr_session.go:291-306) and emits neither Cache Response nor End of Data PDUs whose Session ID and version it would have to match |
| [`RFC8210-5.11-1`](#rfc8210-5.11-1) Error Report PDU must not be sent for an Error Report PDU (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze only reads received Error Reports (internal/component/bgp/plugins/rpki/rtr_session.go:363) and has no Error Report writer, so it can never emit an Error Report about an Error Report |
| [`RFC8210-5.11-2`](#rfc8210-5.11-2) Erroneous PDU field must be empty and Length of Encapsulated PDU must be zero for generic errors (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes the Erroneous PDU field or its length |
| [`RFC8210-5.11-3`](#rfc8210-5.11-3) Error text, if present, must be UTF-8 encoded (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes error text |
| [`RFC8210-5.11-4`](#rfc8210-5.11-4) Length of Error Text field must be zero if no diagnostic text present (§5.11) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never sets Length of Error Text |
| [`RFC8210-6-1`](#rfc8210-6-1) Caches must set Expire Interval to a value larger than either Refresh Interval or Retry Interval (§6) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side timing obligation -- ze is the router: it adopts whatever intervals the End of Data PDU carries (internal/component/bgp/plugins/rpki/rtr_session.go:298-306) and publishes none of its own, so it sets no Expire Interval |
| [`RFC8210-6-2`](#rfc8210-6-2) Router must not retain data past the time indicated by Expire Interval (§6) | {gap}, no test | ze retains VRPs past the Expire Interval -- expireInterval only sets the socket read deadline (internal/component/bgp/plugins/rpki/rtr_session.go:188) and Run simply reconnects after retryInterval (internal/component/bgp/plugins/rpki/rtr_session.go:106-110); ROACache.Clear (internal/component/bgp/plugins/rpki/roa_cache.go:317) has no production caller, so cached VRPs outlive an expired session |
| [`RFC8210-7-2`](#rfc8210-7-2) Cache receiving v0 query must downgrade to v0 or send v1 Error Report with Error Code 4 and terminate (§7) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side obligation -- ze runs no RTR cache server; RTRSession dials out to a configured cache (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync) and receives no queries to downgrade or reject |
| [`RFC8210-7-3`](#rfc8210-7-3) Router receiving v0 response must either downgrade to v0 or terminate the connection (§7) | {gap}, no test | ze never inspects a received PDU's Protocol Version -- handlePDU switches on hdr.Type alone (internal/component/bgp/plugins/rpki/rtr_session.go:229-383) and reads hdr.Version nowhere, so a version 0 response is processed as if it matched the negotiated version; ze also cannot downgrade below version 1 (rtrVersionMin, internal/component/bgp/plugins/rpki/rtr_pdu.go:16) |
| [`RFC8210-7-4`](#rfc8210-7-4) Party receiving PDU for different Protocol Version after negotiation must drop the session (§7) | {gap}, no test | no post-negotiation version check -- handlePDU compares no received PDU version against the negotiated s.version (internal/component/bgp/plugins/rpki/rtr_session.go:229-383); the one version-sensitive guard (internal/component/bgp/plugins/rpki/rtr_session.go:271) keys off the ASPA PDU type, not the header version field |
| [`RFC8210-4-1`](#rfc8210-4-1) Router must choose the most preferred cache by configuration (§4) | {gap}, no test | ze selects no most-preferred cache -- startSessions launches a Run goroutine for every configured cache concurrently (internal/component/bgp/plugins/rpki/rpki.go:291-298) and the parsed preference is only rendered for display (internal/component/bgp/plugins/rpki/rpki.go:1026) |
| [`RFC8210-12-1`](#rfc8210-12-1) Errors which are considered fatal must cause the session to be dropped (§12) | {gap}, no test | Error Code 4 is fatal per RFC 8210 Section 12 but isFatalError excludes it (internal/component/bgp/plugins/rpki/rtr_pdu.go:167-169); once the session already sits at rtrVersionMin the downgrade branch is skipped and handlePDU logs the code and keeps the session alive (internal/component/bgp/plugins/rpki/rtr_session.go:366-375), so this fatal error drops no session |
| [`RFC8210-8.2-1`](#rfc8210-8.2-1) Cache must rate-limit Serial Notifies to no more frequently than one per minute (§8.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it (internal/component/bgp/plugins/rpki/rtr_session.go:359-361) and has no Serial Notify writer |
| [`RFC8210-9-2`](#rfc8210-9-2) Cache and routers must be on the same trusted and controlled network when using unprotected TCP (§9) | no test | no test carries this requirement id; annotated {not-applicable}: operator deployment obligation -- ze dials plain TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) with no code path that can observe or enforce network topology |
| [`RFC8210-9-3`](#rfc8210-9-3) Caches and routers must use one of the protected protocols when available (§9) | {gap}, no test | ze implements no protected RTR transport -- connectAndSync dials only unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and the package contains no SSH, TLS, TCP-MD5 or TCP-AO client, so no protected protocol is available to select |
| [`RFC8210-9.1-1`](#rfc8210-9.1-1) Cache servers supporting SSH must accept RSA authentication (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server SSH obligation -- ze runs no RTR SSH server and no SSH client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it performs no SSH authentication |
| [`RFC8210-9.1-2`](#rfc8210-9.1-2) SSH user authentication must be supported (§9.1) | no test | no test carries this requirement id; annotated {not-applicable}: SSH-transport obligation -- ze implements no SSH RTR transport (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so SSH user authentication has no code path |
| [`RFC8210-9.2-1`](#rfc8210-9.2-1) Client routers using TLS must present client-side certificates (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it presents no client certificate |
| [`RFC8210-9.2-2`](#rfc8210-9.2-2) TLS client certificates must include subjectAltName with iPAddress identities (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds and presents no rpki-rtr client certificate |
| [`RFC8210-9.2-3`](#rfc8210-9.2-3) Cache must check TLS client IP against iPAddress identities (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side TLS obligation -- ze runs no RTR TLS server (it dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting client IP against iPAddress identities |
| [`RFC8210-9.2-4`](#rfc8210-9.2-4) Routers must verify cache TLS server certificate using subjectAltName dNSName per RFC 6125 (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it verifies no cache server certificate |
| [`RFC8210-9.2-5`](#rfc8210-9.2-5) TLS implementations must not use Common Name (CN-ID) for authentication (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP), so it uses no certificate identifiers, CN-ID or otherwise |
| [`RFC8210-9.2-6`](#rfc8210-9.2-6) DNS-ID identifier type must be present in rpki-rtr server certificates (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server-certificate obligation under TLS -- ze runs no RTR TLS server and issues no server certificate (it is a plain-TCP client, internal/component/bgp/plugins/rpki/rtr_session.go:127) |
| [`RFC8210-9.3-1`](#rfc8210-9.3-1) TCP MD5 implementations must support key lengths of at least 80 printable ASCII bytes (§9.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no MD5 keys |
| [`RFC8210-9.4-1`](#rfc8210-9.4-1) TCP-AO implementations must support key lengths of at least 80 printable ASCII bytes and MAC lengths of at least 96 bits (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no AO keys and negotiates no MAC length |
| [`RFC8210-4-2`](#rfc8210-4-2) Servers' clocks must be correct to a tolerance of approximately an hour (§4) | no test | no test carries this requirement id; annotated {not-applicable}: server-clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache server and holds no clock this binds |
| [`RFC8210-4-3`](#rfc8210-4-3) Serial Number comparison must take wrap-around into account per RFC 1982 (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze performs no Serial Number ordering comparison -- it stores the serial the cache reports (internal/component/bgp/plugins/rpki/rtr_session.go:297) and tests only serial == 0 to choose between a Reset Query and a Serial Query (internal/component/bgp/plugins/rpki/rtr_session.go:166); an equality test against zero is unaffected by wrap-around, and grep over the package finds no other serial comparison |
| [`RFC8210-7-7`](#rfc8210-7-7) If either party receives a PDU with unrecognized Protocol Version during negotiation, it MUST either downgrade to a known version or terminate (§7) | {gap}, no test | ze validates no received Protocol Version -- handlePDU never reads hdr.Version (internal/component/bgp/plugins/rpki/rtr_session.go:229-383), so an unrecognized version in a received PDU triggers neither a downgrade nor a termination; the downgrade at internal/component/bgp/plugins/rpki/rtr_session.go:366-370 fires only on an Error Report carrying code 4 |
| [`RFC8210-9.2-8`](#rfc8210-9.2-8) Client router using TLS MUST set its reference identifier to the DNS name of the rpki-rtr cache (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it sets no TLS reference identifier |
| [`RFC8210-10-2`](#rfc8210-10-2) Router MUST keep data from multiple caches marked as to source, as later updates MUST affect the correct data (§10) | {gap}, no test | VRPs carry no source marking -- every RTR session writes one shared ROACache (internal/component/bgp/plugins/rpki/rpki.go:292) and vrpEntry holds only MaxLength and ASN (internal/component/bgp/plugins/rpki/roa_cache.go:12-15), so a withdraw from one cache removes a VRP a different cache announced |
| [`RFC8210-9.3-2`](#rfc8210-9.3-2) TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| [`RFC8210-9.4-2`](#rfc8210-9.4-2) TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| [`RFC8210-9.4-3`](#rfc8210-9.4-3) The cryptographic algorithms and associated parameters described in RFC 5926 MUST be supported for TCP-AO (§9.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |
| [`RFC8210-9.2-9`](#rfc8210-9.2-9) CAs issuing rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: certification-authority obligation -- ze issues no rpki-rtr server certificates and runs no TLS RTR transport (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP) |
| [`RFC8210-9.2-10`](#rfc8210-9.2-10) CN field in TLS certificate MUST NOT be used for authentication (§9.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it reads no certificate CN field for authentication |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8210-5-1`](#rfc8210-5-1)

Reserved fields must be zero on transmission and ignored on receipt (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L39) | unit/verify | unproven |
| positive | [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L55) | unit/verify | unproven |
| positive | [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L19) | unit/verify | unproven |

### [`RFC8210-5.1-1`](#rfc8210-5.1-1)

Flags field bits other than bit 0 must be zero on transmission and ignored on receipt (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L433) | unit/verify | unproven |
| positive | [`TestParsePrefixPDUFlagsHighBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L424) | unit/verify | unproven |

### [`RFC8210-5.1-2`](#rfc8210-5.1-2)

Max Length must not be less than Prefix Length in IPv4/IPv6 Prefix PDUs (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L132) | unit/verify | unproven |
| positive | [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L59) | unit/verify | unproven |

### [`RFC8210-5.1-3`](#rfc8210-5.1-3)

Party detecting Session ID mismatch must immediately terminate with Error Report PDU code 0 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-3, so no unit is bound to it.

### [`RFC8210-5.1-4`](#rfc8210-5.1-4)

Router must flush all data learned from a cache on Session ID mismatch (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-4, so no unit is bound to it.

### [`RFC8210-5.1-5`](#rfc8210-5.1-5)

Routers must treat sessions with different Protocol Version fields as separate sessions even if same Session ID (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.1-5, so no unit is bound to it.

### [`RFC8210-5.3-1`](#rfc8210-5.3-1)

Cache must return the minimum set of changes needed to bring router into sync on Serial Query reply (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.3-1, so no unit is bound to it.

### [`RFC8210-5.3-2`](#rfc8210-5.3-2)

Cache must merge multiple changes to same prefix/key into simplest possible view (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.3-2, so no unit is bound to it.

### [`RFC8210-5.5-1`](#rfc8210-5.5-1)

Withdraw/announce field in payload PDUs must have value 1 (announce) when replying to Reset Query (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.5-1, so no unit is bound to it.

### [`RFC8210-5.6-1`](#rfc8210-5.6-1)

Cache must ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any point in time (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.6-1, so no unit is bound to it.

### [`RFC8210-5.10-1`](#rfc8210-5.10-1)

Cache must ensure one and only one Router Key PDU for a unique {SKI, ASN, Subject Public Key} at any point in time (§5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.10-1, so no unit is bound to it.

### [`RFC8210-5.10-2`](#rfc8210-5.10-2)

Implementations must compare Subject Public Key values as well as SKIs when detecting duplicate PDUs (§5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.10-2, so no unit is bound to it.

### [`RFC8210-5.8-1`](#rfc8210-5.8-1)

End of Data Session ID and Protocol Version must match the corresponding Cache Response (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.8-1, so no unit is bound to it.

### [`RFC8210-5.11-1`](#rfc8210-5.11-1)

Error Report PDU must not be sent for an Error Report PDU (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-1, so no unit is bound to it.

### [`RFC8210-5.11-2`](#rfc8210-5.11-2)

Erroneous PDU field must be empty and Length of Encapsulated PDU must be zero for generic errors (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-2, so no unit is bound to it.

### [`RFC8210-5.11-3`](#rfc8210-5.11-3)

Error text, if present, must be UTF-8 encoded (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-3, so no unit is bound to it.

### [`RFC8210-5.11-4`](#rfc8210-5.11-4)

Length of Error Text field must be zero if no diagnostic text present (§5.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-5.11-4, so no unit is bound to it.

### [`RFC8210-6-1`](#rfc8210-6-1)

Caches must set Expire Interval to a value larger than either Refresh Interval or Retry Interval (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-6-1, so no unit is bound to it.

### [`RFC8210-6-2`](#rfc8210-6-2)

Router must not retain data past the time indicated by Expire Interval (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-6-2, so no unit is bound to it.

### [`RFC8210-7-1`](#rfc8210-7-1)

Router must start each transport connection by issuing either a Reset Query or Serial Query (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L307) | unit/verify | unproven |

### [`RFC8210-7-2`](#rfc8210-7-2)

Cache receiving v0 query must downgrade to v0 or send v1 Error Report with Error Code 4 and terminate (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-2, so no unit is bound to it.

### [`RFC8210-7-3`](#rfc8210-7-3)

Router receiving v0 response must either downgrade to v0 or terminate the connection (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-3, so no unit is bound to it.

### [`RFC8210-5.2-1`](#rfc8210-5.2-1)

Router must ignore any Serial Notify PDUs received during version negotiation startup period (§5.2, §7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L244) | unit/verify | unproven |
| positive | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L218) | unit/verify | unproven |

### [`RFC8210-7-4`](#rfc8210-7-4)

Party receiving PDU for different Protocol Version after negotiation must drop the session (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-4, so no unit is bound to it.

### [`RFC8210-4-1`](#rfc8210-4-1)

Router must choose the most preferred cache by configuration (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-4-1, so no unit is bound to it.

### [`RFC8210-8.1-1`](#rfc8210-8.1-1)

Router must send either a Serial Query or Reset Query periodically (§8.1, §8.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPollingCadenceAtLeastHourly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L90) | unit/verify | unproven |

### [`RFC8210-8.3-1`](#rfc8210-8.3-1)

Router must issue Reset Query and get entire new load after Cache Reset when no preferred caches available (§8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L132) | unit/verify | unproven |
| positive | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L116) | unit/verify | unproven |

### [`RFC8210-12-1`](#rfc8210-12-1)

Errors which are considered fatal must cause the session to be dropped (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-12-1, so no unit is bound to it.

### [`RFC8210-8.2-1`](#rfc8210-8.2-1)

Cache must rate-limit Serial Notifies to no more frequently than one per minute (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-8.2-1, so no unit is bound to it.

### [`RFC8210-9-1`](#rfc8210-9-1)

Caches and routers must implement unprotected transport over TCP using port rpki-rtr (323) (§9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L96) | unit/verify | unproven |

### [`RFC8210-9-2`](#rfc8210-9-2)

Cache and routers must be on the same trusted and controlled network when using unprotected TCP (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9-2, so no unit is bound to it.

### [`RFC8210-9-3`](#rfc8210-9-3)

Caches and routers must use one of the protected protocols when available (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9-3, so no unit is bound to it.

### [`RFC8210-9.1-1`](#rfc8210-9.1-1)

Cache servers supporting SSH must accept RSA authentication (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.1-1, so no unit is bound to it.

### [`RFC8210-9.1-2`](#rfc8210-9.1-2)

SSH user authentication must be supported (§9.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.1-2, so no unit is bound to it.

### [`RFC8210-9.2-1`](#rfc8210-9.2-1)

Client routers using TLS must present client-side certificates (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-1, so no unit is bound to it.

### [`RFC8210-9.2-2`](#rfc8210-9.2-2)

TLS client certificates must include subjectAltName with iPAddress identities (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-2, so no unit is bound to it.

### [`RFC8210-9.2-3`](#rfc8210-9.2-3)

Cache must check TLS client IP against iPAddress identities (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-3, so no unit is bound to it.

### [`RFC8210-9.2-4`](#rfc8210-9.2-4)

Routers must verify cache TLS server certificate using subjectAltName dNSName per RFC 6125 (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-4, so no unit is bound to it.

### [`RFC8210-9.2-5`](#rfc8210-9.2-5)

TLS implementations must not use Common Name (CN-ID) for authentication (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-5, so no unit is bound to it.

### [`RFC8210-9.2-6`](#rfc8210-9.2-6)

DNS-ID identifier type must be present in rpki-rtr server certificates (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-6, so no unit is bound to it.

### [`RFC8210-9.3-1`](#rfc8210-9.3-1)

TCP MD5 implementations must support key lengths of at least 80 printable ASCII bytes (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.3-1, so no unit is bound to it.

### [`RFC8210-9.4-1`](#rfc8210-9.4-1)

TCP-AO implementations must support key lengths of at least 80 printable ASCII bytes and MAC lengths of at least 96 bits (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-1, so no unit is bound to it.

### [`RFC8210-10-1`](#rfc8210-10-1)

Data from multiple caches must not be distinguished between when performing BGP validation (§10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L162) | unit/verify | unproven |
| positive | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L154) | unit/verify | unproven |

### [`RFC8210-4-2`](#rfc8210-4-2)

Servers' clocks must be correct to a tolerance of approximately an hour (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-4-2, so no unit is bound to it.

### [`RFC8210-4-3`](#rfc8210-4-3)

Serial Number comparison must take wrap-around into account per RFC 1982 (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-4-3, so no unit is bound to it.

### [`RFC8210-7-7`](#rfc8210-7-7)

If either party receives a PDU with unrecognized Protocol Version during negotiation, it MUST either downgrade to a known version or terminate (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-7-7, so no unit is bound to it.

### [`RFC8210-7-8`](#rfc8210-7-8)

Routers MUST handle Serial Notify PDUs received before version negotiation completes (by ignoring them) (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L248) | unit/verify | unproven |
| positive | [`TestSerialNotifyIgnoredDuringStartup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L221) | unit/verify | unproven |

### [`RFC8210-9.2-8`](#rfc8210-9.2-8)

Client router using TLS MUST set its reference identifier to the DNS name of the rpki-rtr cache (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-8, so no unit is bound to it.

### [`RFC8210-10-2`](#rfc8210-10-2)

Router MUST keep data from multiple caches marked as to source, as later updates MUST affect the correct data (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-10-2, so no unit is bound to it.

### [`RFC8210-9.3-2`](#rfc8210-9.3-2)

TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.3-2, so no unit is bound to it.

### [`RFC8210-9.4-2`](#rfc8210-9.4-2)

TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (128 bits) (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-2, so no unit is bound to it.

### [`RFC8210-9.4-3`](#rfc8210-9.4-3)

The cryptographic algorithms and associated parameters described in RFC 5926 MUST be supported for TCP-AO (§9.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.4-3, so no unit is bound to it.

### [`RFC8210-9.2-9`](#rfc8210-9.2-9)

CAs issuing rpki-rtr server certificates MUST support the DNS-ID identifier type (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-9, so no unit is bound to it.

### [`RFC8210-9.2-10`](#rfc8210-9.2-10)

CN field in TLS certificate MUST NOT be used for authentication (§9.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8210-9.2-10, so no unit is bound to it.

### [`RFC8210-8.1-2`](#rfc8210-8.1-2)

When transport first established, router MUST send Reset Query or Serial Query (§8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFirstPDUOnConnectionIsAQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L310) | unit/verify | unproven |

### [`RFC8210-8.4-1`](#rfc8210-8.4-1)

If cache cannot supply update and no other caches available, router MUST issue periodic Reset Queries (§8.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L184) | unit/verify | unproven |
| positive | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L165) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8210, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8210, so its obligations are stated where they were written.
