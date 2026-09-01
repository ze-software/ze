# RFC 6810 - The Resource Public Key Infrastructure (RPKI) to Router Protocol

Partial. Every requirement this repository extracted from RFC 6810, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 45.5% | 5 of 11 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 18.2% | 2 of 11 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 39 | of 67 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 28 | of 39 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 36.4% | 4 of 11 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 11 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 67 |
| Gated MUST-level | 39 |
| Obligations that bind Ze | 11 |
| Not applicable, so out of scope | 28 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6810.md` |
| Requirement shard | `rfc/requirements/rfc6810.md` |
| RFC text | `rfc/full/rfc6810.txt` |

## Enrolment

Enrolled: RPKI to Router Protocol v0 / RPKI-RTR (RFC 6810): router / cache-client role. 5 MET + 2 single-polarity positive + 4 gap + 28 not-applicable (cache-server-side obligations ze does not perform)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- ze is the RTR router (client): it parses IPv4/IPv6 Prefix, Cache Response, End of Data, Cache Reset, Serial Notify and Error Report PDUs ([`internal/component/bgp/plugins/rpki/rtr_pdu.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu.go), rtr_session.go), sends Reset/Serial Queries, rejects a Prefix PDU whose Max Length is below its Prefix Length on receipt, validates route origins against the merged VRP cache (validate.go), and re-issues a Reset Query on Cache Reset and No Data Available. The v0 wire is reached through these shared router behaviors
- ze negotiates RTR v1/v2 (min version 1).


**What the ledger says remains**

Four MUST-level gaps, each annotated in [`rfc/short/rfc6810.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6810.md): [`RFC6810-5.1-2`](#rfc6810-5.1-2) -- no Session ID mismatch detection or cache flush (rtr_session.go adopts the cache Session ID unconditionally); [`RFC6810-4-1`](#rfc6810-4-1) -- no most-preferred-cache selection (rpki.go runs every configured cache concurrently); [`RFC6810-8-1`](#rfc6810-8-1) -- VRPs are not marked by source cache (one shared ROACache, rpki.go, roa_cache.go); [`RFC6810-7-3`](#rfc6810-7-3) -- no protected RTR transport (rtr_session.go dials only unprotected TCP, with no SSH/TLS/TCP-MD5/TCP-AO client).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 34 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **39** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC6810-5-1`](#rfc6810-5-1), [`RFC6810-5.1-1`](#rfc6810-5.1-1), [`RFC6810-6.3-1`](#rfc6810-6.3-1), [`RFC6810-6.4-1`](#rfc6810-6.4-1), [`RFC6810-8-2`](#rfc6810-8-2)

**Annotated instead of tested (34):** [`RFC6810-5.6-1`](#rfc6810-5.6-1), [`RFC6810-5.5-1`](#rfc6810-5.5-1), [`RFC6810-5.8-1`](#rfc6810-5.8-1), [`RFC6810-5.10-1`](#rfc6810-5.10-1), [`RFC6810-5.10-2`](#rfc6810-5.10-2), [`RFC6810-5.10-3`](#rfc6810-5.10-3), [`RFC6810-5.10-4`](#rfc6810-5.10-4), [`RFC6810-2-1`](#rfc6810-2-1), [`RFC6810-5.1-2`](#rfc6810-5.1-2), [`RFC6810-4-1`](#rfc6810-4-1), [`RFC6810-6.1-1`](#rfc6810-6.1-1), [`RFC6810-6.2-1`](#rfc6810-6.2-1), [`RFC6810-3-1`](#rfc6810-3-1), [`RFC6810-4-2`](#rfc6810-4-2), [`RFC6810-7-1`](#rfc6810-7-1), [`RFC6810-7-2`](#rfc6810-7-2), [`RFC6810-7-3`](#rfc6810-7-3), [`RFC6810-7.1-1`](#rfc6810-7.1-1), [`RFC6810-7.1-2`](#rfc6810-7.1-2), [`RFC6810-8-1`](#rfc6810-8-1), [`RFC6810-7.2-1`](#rfc6810-7.2-1), [`RFC6810-7.2-2`](#rfc6810-7.2-2), [`RFC6810-7.2-3`](#rfc6810-7.2-3), [`RFC6810-7.2-4`](#rfc6810-7.2-4), [`RFC6810-7.2-5`](#rfc6810-7.2-5), [`RFC6810-7.2-6`](#rfc6810-7.2-6), [`RFC6810-7.2-7`](#rfc6810-7.2-7), [`RFC6810-7.2-8`](#rfc6810-7.2-8), [`RFC6810-7.3-1`](#rfc6810-7.3-1), [`RFC6810-7.3-2`](#rfc6810-7.3-2), [`RFC6810-7.4-1`](#rfc6810-7.4-1), [`RFC6810-7.4-2`](#rfc6810-7.4-2), [`RFC6810-7.4-3`](#rfc6810-7.4-3), [`RFC6810-7.4-4`](#rfc6810-7.4-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6810-5-1` | Fields with unspecified content MUST be zero on transmission (Section 5) | MUST | 5 | **positive:** `unit/verify` [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L16). **positive:** `unit/verify` [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L52). **negative:** `unit/verify` [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L35) |
| `RFC6810-5.1-1` | Max Length MUST NOT be less than Prefix Length (Section 5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L57). **negative:** `unit/verify` [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L130) |
| `RFC6810-5.6-1` | Cache MUST ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time (Section 5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission guarantee -- ze is the RTR router; it only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| `RFC6810-5.5-1` | In response to a Reset Query, the withdraw/announce field in payload PDUs MUST have the value 1 (announce) (Section 5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) but sends no Prefix PDUs, so it never sets this field on transmission |
| `RFC6810-5.8-1` | Session ID in End of Data MUST be the same as the corresponding Cache Response (Section 5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side emission obligation -- ze reads End of Data for its serial only (internal/component/bgp/plugins/rpki/rtr_session.go:291) and emits neither Cache Response nor End of Data PDUs whose Session IDs it would have to match |
| `RFC6810-5.10-1` | If the Erroneous PDU field is empty, Length of Encapsulated PDU MUST be zero (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze only reads received Error Reports (internal/component/bgp/plugins/rpki/rtr_session.go:363) and has no Error Report writer, so it never encodes Length of Encapsulated PDU |
| `RFC6810-5.10-2` | An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | MUST NOT | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze has no Error Report writer (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it can never emit an Error Report about an Error Report |
| `RFC6810-5.10-3` | If error text is present, it MUST be UTF-8 encoded (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes error text |
| `RFC6810-5.10-4` | If diagnostic text is not present, Length of Error Text MUST be zero (Section 5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never sets Length of Error Text |
| `RFC6810-2-1` | Incoming data associated with new serial MUST NOT be sent until the fetch is complete (Section 2, Serial Number definition) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side transmission obligation on serial-associated data -- ze is the consumer; it buffers a delta in pendingVRPs and applies it atomically only at End of Data (internal/component/bgp/plugins/rpki/rtr_session.go:307), and sends no serial-tagged data |
| `RFC6810-5.1-2` | If Session ID mismatch, MUST completely drop the session and flush all data learned from that cache (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not detect a Session ID change -- handlePDU adopts the Cache Response Session ID unconditionally (internal/component/bgp/plugins/rpki/rtr_session.go:233, s.sessionID = hdr.SessionID) with no comparison to the prior value and no cache flush |
| `RFC6810-4-1` | The router MUST choose the most preferred cache by configuration (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not select the most preferred cache -- startSessions launches a Run goroutine for every configured cache concurrently (internal/component/bgp/plugins/rpki/rpki.go:291-298) and the parsed preference is only surfaced for display (rpki.go:1026), never used to order or choose caches |
| `RFC6810-6.1-1` | Router MUST send Serial Query or Reset Query no less frequently than once an hour (Section 6.1, Section 6.2) | MUST | 6.1 | **positive:** `unit/verify` [`TestPollingCadenceAtLeastHourly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L87). **negative:** no negative test. **{single-polarity}:** the re-query cadence is Run's post-sync wait of s.retryInterval (internal/component/bgp/plugins/rpki/rtr_session.go:106-110), defaulting to 600s (rtr_session.go:81) which is below the one-hour ceiling; there is no malformed input that yields a too-infrequent negative case |
| `RFC6810-6.2-1` | Cache MUST rate limit Serial Notifies to no more than one per minute (Section 6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it during sync (internal/component/bgp/plugins/rpki/rtr_session.go:359) and never sends one |
| `RFC6810-6.3-1` | If Cache Reset received and no more preferred caches, router MUST issue a Reset Query (Section 6.3) | MUST | 6.3 | **positive:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L114). **negative:** `unit/verify` [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L130) |
| `RFC6810-6.4-1` | If No Data Available and no other caches, router MUST issue periodic Reset Queries (Section 6.4) | MUST | 6.4 | **positive:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L162). **negative:** `unit/verify` [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L181) |
| `RFC6810-3-1` | A relying party MUST have a trust relationship with, and a trusted transport channel to, any authoritative caches it uses (Section 3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** operator deployment obligation -- the trust relationship and trusted channel are established by network placement; ze dials the operator-configured cache over unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and has no in-band trust mechanism to produce or violate |
| `RFC6810-4-2` | Cache servers' clocks MUST be correct to a tolerance of approximately an hour (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache and holds no clock this binds |
| `RFC6810-7-1` | Caches and routers MUST implement unprotected TCP transport on port rpki-rtr (323) (Section 7) | MUST | 7 | **positive:** `unit/verify` [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L93). **negative:** no negative test. **{single-polarity}:** ze implements the unprotected TCP transport by dialing tcp (internal/component/bgp/plugins/rpki/rtr_session.go:127) and defaults the RTR port to 323 (rpki_config.go:174); it never declines to offer unprotected TCP, so there is no negative case |
| `RFC6810-7-2` | If unprotected TCP is the transport, cache and routers MUST be on the same trusted and controlled network (Section 7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** operator deployment obligation -- when unprotected TCP is used the trusted-network placement is the operator's; ze dials plain TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) with no code path to enforce network topology |
| `RFC6810-7-3` | If available, caches and routers MUST use one of the more protected protocols (Section 7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no protected RTR transport -- connectAndSync dials only unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and the plugin has no SSH, TLS, TCP-MD5, or TCP-AO client, so no more protected protocol is available to select |
| `RFC6810-7.1-1` | Cache servers supporting SSH MUST accept RSA and DSA authentication (Section 7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server SSH obligation -- ze runs no RTR SSH server and no SSH client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it performs no SSH authentication |
| `RFC6810-7.1-2` | SSH user authentication MUST be supported (Section 7.1) | MUST | 7.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** SSH-transport obligation -- ze implements no SSH RTR transport (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so SSH user authentication has no code path |
| `RFC6810-8-1` | Client MUST keep data marked as to source, as later updates MUST affect the correct data (Section 8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not mark VRPs by source cache -- every RTR session writes one shared ROACache (internal/component/bgp/plugins/rpki/rpki.go:292) and vrpEntry carries only MaxLength and ASN (roa_cache.go:12-15) with no source field, so a withdraw from one cache can remove a VRP another announced |
| `RFC6810-8-2` | If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8) | MUST NOT | 8 | **positive:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L152). **negative:** `unit/verify` [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L159) |
| `RFC6810-7.2-1` | TLS client routers MUST present client-side certificates (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so the client-certificate requirement has no code path |
| `RFC6810-7.2-2` | TLS certificates MUST include subjectAltName extension with iPAddress identities (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR transport (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it issues and validates no rpki-rtr TLS certificates |
| `RFC6810-7.2-3` | Cache MUST check IP address of TLS connection against iPAddress identities (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-side TLS obligation -- ze runs no RTR TLS server (it is a client that dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting IP against iPAddress identities |
| `RFC6810-7.2-4` | Routers MUST verify the cache's TLS server certificate using subjectAltName dNSName identities (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it verifies no cache server certificate |
| `RFC6810-7.2-5` | DNS-ID identifier type support is REQUIRED in rpki-rtr TLS implementations (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP), so DNS-ID identifier support does not apply |
| `RFC6810-7.2-6` | DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** cache-server-certificate obligation under TLS -- ze runs no RTR TLS server and issues no server certificate (it is a plain-TCP client, internal/component/bgp/plugins/rpki/rtr_session.go:127) |
| `RFC6810-7.2-7` | rpki-rtr TLS implementations MUST NOT use CN-ID identifiers for authentication (Section 7.2) | MUST NOT | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it uses no certificate identifiers, CN-ID or otherwise |
| `RFC6810-7.2-8` | The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it sets no TLS reference identifier |
| `RFC6810-7.3-1` | TCP MD5 implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no MD5 keys |
| `RFC6810-7.3-2` | TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| `RFC6810-7.4-1` | TCP-AO implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no AO keys |
| `RFC6810-7.4-2` | TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| `RFC6810-7.4-3` | TCP-AO MAC lengths of at least 96 bits MUST be supported (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it negotiates no MAC length |
| `RFC6810-7.4-4` | TCP-AO cryptographic algorithms described in RFC 5926 MUST be supported (Section 7.4) | MUST | 7.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |
| `RFC6810-6.2-2` | Cache SHOULD send a Notify PDU when its serial changes (Section 6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.6-2` | Router receiving a duplicate announcement SHOULD raise Duplicate Announcement Received error (Section 5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-4` | Caches and routers SHOULD use TCP-AO, SSHv2, TCP MD5, or IPsec transport (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-5` | Both caches and routers SHOULD enable transport keep-alives (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.3-2` | Router SHOULD attempt more preferred caches on Cache Reset (Section 6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.4-2` | Router SHOULD attempt other caches on No Data Available (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-3` | Client SHOULD retain data from previous cache during switchover (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-4` | Client SHOULD delete data from unreachable cache after 2x polling period (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-1` | Cache SHOULD be topologically close to router (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.10-5` | If erroneous Error Report PDU received, session SHOULD be dropped (Section 5.10) | SHOULD | 5.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-3` | Cache servers supporting SSH SHOULD accept ECDSA authentication (Section 7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-4` | Client routers SHOULD verify the public key of the cache to avoid MITM attacks (Section 7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.2-9` | TLS cache SHOULD reject connections if no iPAddress identities match (Section 7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.2-10` | DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*" (Section 7.2) | SHOULD NOT | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-6` | Operators SHOULD use procedural means (e.g., ACLs) to reduce exposure to authentication issues when using unprotected TCP (Section 7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.3-3` | Cache servers SHOULD support RFC 4808 for TCP MD5 key rollover (Section 7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-5` | The client SHOULD attempt to maintain at least one set of data regardless of cache changes (Section 8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-2` | The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-11-3` | Protocols that provide integrity and authenticity SHOULD be used for cache-router transport (Section 11) | SHOULD | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-10-1` | Fatal errors SHOULD cause the session to be dropped (Section 10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-6.1-2` | Router MAY start with Serial Query if it has unexpired data from a previous session (Section 6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5.10-6` | Truncated Erroneous PDU MAY be included for oversized PDUs (Section 5.10) | MAY | 5.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-5-2` | Fields with unspecified content MAY be ignored on receipt (Section 5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7-7` | Routers MAY use SSHv2, TCP MD5, IPsec, or TLS transport (Section 7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-5` | SSH host authentication MAY be supported (Section 7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-7.1-6` | SSH implementations MAY support password authentication (Section 7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-8-6` | A client MAY drop the data from a particular cache when it is fully in sync with other caches (Section 8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC6810-3-2` | There MAY be mechanisms for the router to assure authenticity of the cache (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6810-5.6-1`](#rfc6810-5.6-1) Cache MUST ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time (Section 5.6) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission guarantee -- ze is the RTR router; it only parses Prefix PDUs (internal/component/bgp/plugins/rpki/rtr_pdu.go:114 parsePrefixPDU) and has no Prefix PDU writer, so it never emits or enforces one-PDU-per-VRP |
| [`RFC6810-5.5-1`](#rfc6810-5.5-1) In response to a Reset Query, the withdraw/announce field in payload PDUs MUST have the value 1 (announce) (Section 5.5) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads the announce/withdraw flag on receipt (internal/component/bgp/plugins/rpki/rtr_pdu.go:132) but sends no Prefix PDUs, so it never sets this field on transmission |
| [`RFC6810-5.8-1`](#rfc6810-5.8-1) Session ID in End of Data MUST be the same as the corresponding Cache Response (Section 5.8) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side emission obligation -- ze reads End of Data for its serial only (internal/component/bgp/plugins/rpki/rtr_session.go:291) and emits neither Cache Response nor End of Data PDUs whose Session IDs it would have to match |
| [`RFC6810-5.10-1`](#rfc6810-5.10-1) If the Erroneous PDU field is empty, Length of Encapsulated PDU MUST be zero (Section 5.10) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze only reads received Error Reports (internal/component/bgp/plugins/rpki/rtr_session.go:363) and has no Error Report writer, so it never encodes Length of Encapsulated PDU |
| [`RFC6810-5.10-2`](#rfc6810-5.10-2) An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze has no Error Report writer (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it can never emit an Error Report about an Error Report |
| [`RFC6810-5.10-3`](#rfc6810-5.10-3) If error text is present, it MUST be UTF-8 encoded (Section 5.10) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never encodes error text |
| [`RFC6810-5.10-4`](#rfc6810-5.10-4) If diagnostic text is not present, Length of Error Text MUST be zero (Section 5.10) | no test | no test carries this requirement id; annotated {not-applicable}: Error Report sender obligation -- ze emits no Error Report PDU (it only reads them at internal/component/bgp/plugins/rpki/rtr_session.go:363), so it never sets Length of Error Text |
| [`RFC6810-2-1`](#rfc6810-2-1) Incoming data associated with new serial MUST NOT be sent until the fetch is complete (Section 2, Serial Number definition) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side transmission obligation on serial-associated data -- ze is the consumer; it buffers a delta in pendingVRPs and applies it atomically only at End of Data (internal/component/bgp/plugins/rpki/rtr_session.go:307), and sends no serial-tagged data |
| [`RFC6810-5.1-2`](#rfc6810-5.1-2) If Session ID mismatch, MUST completely drop the session and flush all data learned from that cache (Section 5.1) | {gap}, no test | ze does not detect a Session ID change -- handlePDU adopts the Cache Response Session ID unconditionally (internal/component/bgp/plugins/rpki/rtr_session.go:233, s.sessionID = hdr.SessionID) with no comparison to the prior value and no cache flush |
| [`RFC6810-4-1`](#rfc6810-4-1) The router MUST choose the most preferred cache by configuration (Section 4) | {gap}, no test | ze does not select the most preferred cache -- startSessions launches a Run goroutine for every configured cache concurrently (internal/component/bgp/plugins/rpki/rpki.go:291-298) and the parsed preference is only surfaced for display (rpki.go:1026), never used to order or choose caches |
| [`RFC6810-6.2-1`](#rfc6810-6.2-1) Cache MUST rate limit Serial Notifies to no more than one per minute (Section 6.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side rate limit on Serial Notify emission -- ze receives Serial Notify and ignores it during sync (internal/component/bgp/plugins/rpki/rtr_session.go:359) and never sends one |
| [`RFC6810-3-1`](#rfc6810-3-1) A relying party MUST have a trust relationship with, and a trusted transport channel to, any authoritative caches it uses (Section 3) | no test | no test carries this requirement id; annotated {not-applicable}: operator deployment obligation -- the trust relationship and trusted channel are established by network placement; ze dials the operator-configured cache over unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and has no in-band trust mechanism to produce or violate |
| [`RFC6810-4-2`](#rfc6810-4-2) Cache servers' clocks MUST be correct to a tolerance of approximately an hour (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server clock obligation -- ze is the RTR router client that dials out (internal/component/bgp/plugins/rpki/rtr_session.go:125 connectAndSync); it runs no cache and holds no clock this binds |
| [`RFC6810-7-2`](#rfc6810-7-2) If unprotected TCP is the transport, cache and routers MUST be on the same trusted and controlled network (Section 7) | no test | no test carries this requirement id; annotated {not-applicable}: operator deployment obligation -- when unprotected TCP is used the trusted-network placement is the operator's; ze dials plain TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) with no code path to enforce network topology |
| [`RFC6810-7-3`](#rfc6810-7-3) If available, caches and routers MUST use one of the more protected protocols (Section 7) | {gap}, no test | ze implements no protected RTR transport -- connectAndSync dials only unprotected TCP (internal/component/bgp/plugins/rpki/rtr_session.go:127) and the plugin has no SSH, TLS, TCP-MD5, or TCP-AO client, so no more protected protocol is available to select |
| [`RFC6810-7.1-1`](#rfc6810-7.1-1) Cache servers supporting SSH MUST accept RSA and DSA authentication (Section 7.1) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server SSH obligation -- ze runs no RTR SSH server and no SSH client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it performs no SSH authentication |
| [`RFC6810-7.1-2`](#rfc6810-7.1-2) SSH user authentication MUST be supported (Section 7.1) | no test | no test carries this requirement id; annotated {not-applicable}: SSH-transport obligation -- ze implements no SSH RTR transport (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so SSH user authentication has no code path |
| [`RFC6810-8-1`](#rfc6810-8-1) Client MUST keep data marked as to source, as later updates MUST affect the correct data (Section 8) | {gap}, no test | ze does not mark VRPs by source cache -- every RTR session writes one shared ROACache (internal/component/bgp/plugins/rpki/rpki.go:292) and vrpEntry carries only MaxLength and ASN (roa_cache.go:12-15) with no source field, so a withdraw from one cache can remove a VRP another announced |
| [`RFC6810-7.2-1`](#rfc6810-7.2-1) TLS client routers MUST present client-side certificates (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so the client-certificate requirement has no code path |
| [`RFC6810-7.2-2`](#rfc6810-7.2-2) TLS certificates MUST include subjectAltName extension with iPAddress identities (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR transport (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it issues and validates no rpki-rtr TLS certificates |
| [`RFC6810-7.2-3`](#rfc6810-7.2-3) Cache MUST check IP address of TLS connection against iPAddress identities (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-side TLS obligation -- ze runs no RTR TLS server (it is a client that dials out, internal/component/bgp/plugins/rpki/rtr_session.go:125), so it checks no connecting IP against iPAddress identities |
| [`RFC6810-7.2-4`](#rfc6810-7.2-4) Routers MUST verify the cache's TLS server certificate using subjectAltName dNSName identities (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it verifies no cache server certificate |
| [`RFC6810-7.2-5`](#rfc6810-7.2-5) DNS-ID identifier type support is REQUIRED in rpki-rtr TLS implementations (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127 dials plain TCP), so DNS-ID identifier support does not apply |
| [`RFC6810-7.2-6`](#rfc6810-7.2-6) DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: cache-server-certificate obligation under TLS -- ze runs no RTR TLS server and issues no server certificate (it is a plain-TCP client, internal/component/bgp/plugins/rpki/rtr_session.go:127) |
| [`RFC6810-7.2-7`](#rfc6810-7.2-7) rpki-rtr TLS implementations MUST NOT use CN-ID identifiers for authentication (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze has no rpki-rtr TLS implementation (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it uses no certificate identifiers, CN-ID or otherwise |
| [`RFC6810-7.2-8`](#rfc6810-7.2-8) The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | no test | no test carries this requirement id; annotated {not-applicable}: TLS-transport obligation -- ze implements no TLS RTR client (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it sets no TLS reference identifier |
| [`RFC6810-7.3-1`](#rfc6810-7.3-1) TCP MD5 implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no MD5 keys |
| [`RFC6810-7.3-2`](#rfc6810-7.3-2) TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.3) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-MD5 transport obligation -- ze implements no TCP-MD5 for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no MD5 key material |
| [`RFC6810-7.4-1`](#rfc6810-7.4-1) TCP-AO implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (connectAndSync dials plain TCP, internal/component/bgp/plugins/rpki/rtr_session.go:127), so it holds no AO keys |
| [`RFC6810-7.4-2`](#rfc6810-7.4-2) TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it parses no AO key material |
| [`RFC6810-7.4-3`](#rfc6810-7.4-3) TCP-AO MAC lengths of at least 96 bits MUST be supported (Section 7.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it negotiates no MAC length |
| [`RFC6810-7.4-4`](#rfc6810-7.4-4) TCP-AO cryptographic algorithms described in RFC 5926 MUST be supported (Section 7.4) | no test | no test carries this requirement id; annotated {not-applicable}: TCP-AO transport obligation -- ze implements no TCP-AO for RTR (internal/component/bgp/plugins/rpki/rtr_session.go:127), so it supports no RFC 5926 algorithms |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6810-5-1`](#rfc6810-5-1)

Fields with unspecified content MUST be zero on transmission (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestWriteSerialQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L35) | unit/verify | unproven |
| positive | [`TestWriteResetQueryZeroesReservedOverGarbage`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_reserved_test.go#L52) | unit/verify | unproven |
| positive | [`TestWriteResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L16) | unit/verify | unproven |

### [`RFC6810-5.1-1`](#rfc6810-5.1-1)

Max Length MUST NOT be less than Prefix Length (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseIPv4PrefixMaxLenLessThanPrefixLen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L130) | unit/verify | unproven |
| positive | [`TestParseIPv4Prefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_pdu_test.go#L57) | unit/verify | unproven |

### [`RFC6810-5.6-1`](#rfc6810-5.6-1)

Cache MUST ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time (Section 5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.6-1, so no unit is bound to it.

### [`RFC6810-5.5-1`](#rfc6810-5.5-1)

In response to a Reset Query, the withdraw/announce field in payload PDUs MUST have the value 1 (announce) (Section 5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.5-1, so no unit is bound to it.

### [`RFC6810-5.8-1`](#rfc6810-5.8-1)

Session ID in End of Data MUST be the same as the corresponding Cache Response (Section 5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.8-1, so no unit is bound to it.

### [`RFC6810-5.10-1`](#rfc6810-5.10-1)

If the Erroneous PDU field is empty, Length of Encapsulated PDU MUST be zero (Section 5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.10-1, so no unit is bound to it.

### [`RFC6810-5.10-2`](#rfc6810-5.10-2)

An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.10-2, so no unit is bound to it.

### [`RFC6810-5.10-3`](#rfc6810-5.10-3)

If error text is present, it MUST be UTF-8 encoded (Section 5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.10-3, so no unit is bound to it.

### [`RFC6810-5.10-4`](#rfc6810-5.10-4)

If diagnostic text is not present, Length of Error Text MUST be zero (Section 5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.10-4, so no unit is bound to it.

### [`RFC6810-2-1`](#rfc6810-2-1)

Incoming data associated with new serial MUST NOT be sent until the fetch is complete (Section 2, Serial Number definition)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-2-1, so no unit is bound to it.

### [`RFC6810-5.1-2`](#rfc6810-5.1-2)

If Session ID mismatch, MUST completely drop the session and flush all data learned from that cache (Section 5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-5.1-2, so no unit is bound to it.

### [`RFC6810-4-1`](#rfc6810-4-1)

The router MUST choose the most preferred cache by configuration (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-4-1, so no unit is bound to it.

### [`RFC6810-6.1-1`](#rfc6810-6.1-1)

Router MUST send Serial Query or Reset Query no less frequently than once an hour (Section 6.1, Section 6.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPollingCadenceAtLeastHourly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L87) | unit/verify | unproven |

### [`RFC6810-6.2-1`](#rfc6810-6.2-1)

Cache MUST rate limit Serial Notifies to no more than one per minute (Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-6.2-1, so no unit is bound to it.

### [`RFC6810-6.3-1`](#rfc6810-6.3-1)

If Cache Reset received and no more preferred caches, router MUST issue a Reset Query (Section 6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L130) | unit/verify | unproven |
| positive | [`TestCacheResetTriggersResetQuery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L114) | unit/verify | unproven |

### [`RFC6810-6.4-1`](#rfc6810-6.4-1)

If No Data Available and no other caches, router MUST issue periodic Reset Queries (Section 6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L181) | unit/verify | unproven |
| positive | [`TestNoDataAvailableKeepsResetQueryMode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rtr_session_test.go#L162) | unit/verify | unproven |

### [`RFC6810-3-1`](#rfc6810-3-1)

A relying party MUST have a trust relationship with, and a trusted transport channel to, any authoritative caches it uses (Section 3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-3-1, so no unit is bound to it.

### [`RFC6810-4-2`](#rfc6810-4-2)

Cache servers' clocks MUST be correct to a tolerance of approximately an hour (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-4-2, so no unit is bound to it.

### [`RFC6810-7-1`](#rfc6810-7-1)

Caches and routers MUST implement unprotected TCP transport on port rpki-rtr (323) (Section 7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseRPKIConfigDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/rpki_config_test.go#L93) | unit/verify | unproven |

### [`RFC6810-7-2`](#rfc6810-7-2)

If unprotected TCP is the transport, cache and routers MUST be on the same trusted and controlled network (Section 7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7-2, so no unit is bound to it.

### [`RFC6810-7-3`](#rfc6810-7-3)

If available, caches and routers MUST use one of the more protected protocols (Section 7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7-3, so no unit is bound to it.

### [`RFC6810-7.1-1`](#rfc6810-7.1-1)

Cache servers supporting SSH MUST accept RSA and DSA authentication (Section 7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.1-1, so no unit is bound to it.

### [`RFC6810-7.1-2`](#rfc6810-7.1-2)

SSH user authentication MUST be supported (Section 7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.1-2, so no unit is bound to it.

### [`RFC6810-8-1`](#rfc6810-8-1)

Client MUST keep data marked as to source, as later updates MUST affect the correct data (Section 8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-8-1, so no unit is bound to it.

### [`RFC6810-8-2`](#rfc6810-8-2)

If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L159) | unit/verify | unproven |
| positive | [`TestValidationDoesNotDistinguishCacheSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/roa_cache_test.go#L152) | unit/verify | unproven |

### [`RFC6810-7.2-1`](#rfc6810-7.2-1)

TLS client routers MUST present client-side certificates (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-1, so no unit is bound to it.

### [`RFC6810-7.2-2`](#rfc6810-7.2-2)

TLS certificates MUST include subjectAltName extension with iPAddress identities (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-2, so no unit is bound to it.

### [`RFC6810-7.2-3`](#rfc6810-7.2-3)

Cache MUST check IP address of TLS connection against iPAddress identities (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-3, so no unit is bound to it.

### [`RFC6810-7.2-4`](#rfc6810-7.2-4)

Routers MUST verify the cache's TLS server certificate using subjectAltName dNSName identities (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-4, so no unit is bound to it.

### [`RFC6810-7.2-5`](#rfc6810-7.2-5)

DNS-ID identifier type support is REQUIRED in rpki-rtr TLS implementations (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-5, so no unit is bound to it.

### [`RFC6810-7.2-6`](#rfc6810-7.2-6)

DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-6, so no unit is bound to it.

### [`RFC6810-7.2-7`](#rfc6810-7.2-7)

rpki-rtr TLS implementations MUST NOT use CN-ID identifiers for authentication (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-7, so no unit is bound to it.

### [`RFC6810-7.2-8`](#rfc6810-7.2-8)

The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.2-8, so no unit is bound to it.

### [`RFC6810-7.3-1`](#rfc6810-7.3-1)

TCP MD5 implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.3-1, so no unit is bound to it.

### [`RFC6810-7.3-2`](#rfc6810-7.3-2)

TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.3-2, so no unit is bound to it.

### [`RFC6810-7.4-1`](#rfc6810-7.4-1)

TCP-AO implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.4-1, so no unit is bound to it.

### [`RFC6810-7.4-2`](#rfc6810-7.4-2)

TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.4-2, so no unit is bound to it.

### [`RFC6810-7.4-3`](#rfc6810-7.4-3)

TCP-AO MAC lengths of at least 96 bits MUST be supported (Section 7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.4-3, so no unit is bound to it.

### [`RFC6810-7.4-4`](#rfc6810-7.4-4)

TCP-AO cryptographic algorithms described in RFC 5926 MUST be supported (Section 7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6810-7.4-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6810, so no reviewer has walked its text sentence by sentence.

## Superseded

RFC 6810 is obsoleted by RFC 8210.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC6810-5-1`](#rfc6810-5-1) Fields with unspecified content MUST be zero on transmission (Section 5) | restated | RFC8210-5-1 | RFC 8210 Section 5 keeps the sentence and names the field, that reserved fields marked zero in the PDU diagrams MUST be zero on transmission |
| [`RFC6810-5.1-1`](#rfc6810-5.1-1) Max Length MUST NOT be less than Prefix Length (Section 5.1) | restated | RFC8210-5.1-2 | RFC 8210 Section 5.1 keeps the rule for the IPv4 and IPv6 Prefix PDUs, that the Max Length must not be less than the Prefix Length |
| [`RFC6810-5.6-1`](#rfc6810-5.6-1) Cache MUST ensure one and only one IPvX PDU for a unique {Prefix, Len, Max-Len, ASN} at any one point in time (Section 5.6) | restated | RFC8210-5.6-1 | RFC 8210 Section 5.6 keeps the one-and-only-one rule for the IPvX Prefix PDU, and Section 5.10 states the same rule for the new Router Key PDU at RFC8210-5.10-1 |
| [`RFC6810-5.5-1`](#rfc6810-5.5-1) In response to a Reset Query, the withdraw/announce field in payload PDUs MUST have the value 1 (announce) (Section 5.5) | restated | RFC8210-5.5-1 | RFC 8210 Section 5.5 keeps the rule that payload PDUs replying to a Reset Query carry withdraw/announce value 1 |
| [`RFC6810-5.8-1`](#rfc6810-5.8-1) Session ID in End of Data MUST be the same as the corresponding Cache Response (Section 5.8) | restated | RFC8210-5.8-1 | RFC 8210 Section 5.8 keeps the Session ID rule and adds the Protocol Version to it, because version negotiation is new in version 1 |
| [`RFC6810-5.10-1`](#rfc6810-5.10-1) If the Erroneous PDU field is empty, Length of Encapsulated PDU MUST be zero (Section 5.10) | restated | RFC8210-5.11-2 | the Error Report PDU moved from Section 5.10 to Section 5.11 when the Router Key PDU took 5.10, and the rule is kept, that a generic error leaves the Erroneous PDU field empty and the Length of Encapsulated PDU zero |
| [`RFC6810-5.10-2`](#rfc6810-5.10-2) An Error Report PDU MUST NOT be sent for an Error Report PDU (Section 5.10) | restated | RFC8210-5.11-1 | the Error Report PDU moved to Section 5.11 and keeps the rule that no Error Report PDU is sent for an Error Report PDU |
| [`RFC6810-5.10-3`](#rfc6810-5.10-3) If error text is present, it MUST be UTF-8 encoded (Section 5.10) | restated | RFC8210-5.11-3 | the Error Report PDU moved to Section 5.11 and keeps the rule that error text, when present, is UTF-8 encoded |
| [`RFC6810-5.10-4`](#rfc6810-5.10-4) If diagnostic text is not present, Length of Error Text MUST be zero (Section 5.10) | restated | RFC8210-5.11-4 | the Error Report PDU moved to Section 5.11 and keeps the rule that the Length of Error Text is zero when no diagnostic text is present |
| [`RFC6810-2-1`](#rfc6810-2-1) Incoming data associated with new serial MUST NOT be sent until the fetch is complete (Section 2, Serial Number definition) | unextracted | §2 | RFC 8210 Section 2 keeps the Serial Number glossary entry word for word, that while a cache is receiving updates, new incoming data and implicit deletes are associated with the new serial but MUST NOT be sent until the fetch is complete. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-5.1-2`](#rfc6810-5.1-2) If Session ID mismatch, MUST completely drop the session and flush all data learned from that cache (Section 5.1) | restated | RFC8210-5.1-4 | RFC 8210 Section 5.1 keeps the flush and splits the sentence in two, stating the immediate termination with an Error Report PDU of code 0 separately at RFC8210-5.1-3 |
| [`RFC6810-4-1`](#rfc6810-4-1) The router MUST choose the most preferred cache by configuration (Section 4) | restated | RFC8210-4-1 | RFC 8210 Section 4 keeps the rule that the router chooses the most preferred cache by configuration |
| [`RFC6810-6.1-1`](#rfc6810-6.1-1) Router MUST send Serial Query or Reset Query no less frequently than once an hour (Section 6.1, Section 6.2) | restated | RFC8210-8.1-1 | the protocol sequences moved from Section 6 to Section 8, and RFC 8210 replaces the fixed once-an-hour floor with the cache-signalled Refresh Interval of Section 6. The periodic query itself stays a MUST |
| [`RFC6810-6.2-1`](#rfc6810-6.2-1) Cache MUST rate limit Serial Notifies to no more than one per minute (Section 6.2) | restated | RFC8210-8.2-1 | the protocol sequences moved to Section 8, and Section 8.2 keeps the one-per-minute rate limit on Serial Notifies |
| [`RFC6810-6.3-1`](#rfc6810-6.3-1) If Cache Reset received and no more preferred caches, router MUST issue a Reset Query (Section 6.3) | restated | RFC8210-8.3-1 | Section 8.3 keeps the rule that a router with no more-preferred cache available issues a Reset Query and takes an entire new load after a Cache Reset |
| [`RFC6810-6.4-1`](#rfc6810-6.4-1) If No Data Available and no other caches, router MUST issue periodic Reset Queries (Section 6.4) | restated | RFC8210-8.4-1 | Section 8.4 keeps the rule that a router with no other cache available issues periodic Reset Queries when the cache cannot supply an update |
| [`RFC6810-3-1`](#rfc6810-3-1) A relying party MUST have a trust relationship with, and a trusted transport channel to, any authoritative caches it uses (Section 3) | unextracted | §3 | RFC 8210 Section 3 keeps the sentence word for word, that a Relying Party MUST have a trust relationship with, and a trusted transport channel to, any cache it uses. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-4-2`](#rfc6810-4-2) Cache servers' clocks MUST be correct to a tolerance of approximately an hour (Section 4) | restated | RFC8210-4-2 | RFC 8210 Section 4 keeps the tolerance, that servers' clocks are correct to approximately an hour |
| [`RFC6810-7-1`](#rfc6810-7-1) Caches and routers MUST implement unprotected TCP transport on port rpki-rtr (323) (Section 7) | restated | RFC8210-9-1 | the transport section moved from Section 7 to Section 9, which keeps the obligation to implement unprotected transport over TCP on the rpki-rtr port 323 |
| [`RFC6810-7-2`](#rfc6810-7-2) If unprotected TCP is the transport, cache and routers MUST be on the same trusted and controlled network (Section 7) | restated | RFC8210-9-2 | Section 9 keeps the rule that cache and routers are on the same trusted and controlled network when unprotected TCP is the transport |
| [`RFC6810-7-3`](#rfc6810-7-3) If available, caches and routers MUST use one of the more protected protocols (Section 7) | restated | RFC8210-9-3 | Section 9 keeps the obligation to use one of the more protected protocols when it is available to the operator |
| [`RFC6810-7.1-1`](#rfc6810-7.1-1) Cache servers supporting SSH MUST accept RSA and DSA authentication (Section 7.1) | restated | RFC8210-9.1-1 | RFC 8210 Section 9.1 keeps the obligation for RSA and REMOVES DSA. Where RFC 6810 required a cache server supporting SSH to accept RSA and DSA authentication, RFC 8210 requires RSA alone, and its only other keyword in that sentence is the SHOULD for ECDSA at RFC8210-9.1-3 which RFC 6810 also carried |
| [`RFC6810-7.1-2`](#rfc6810-7.1-2) SSH user authentication MUST be supported (Section 7.1) | restated | RFC8210-9.1-2 | Section 9.1 keeps the obligation to support SSH user authentication |
| [`RFC6810-8-1`](#rfc6810-8-1) Client MUST keep data marked as to source, as later updates MUST affect the correct data (Section 8) | restated | RFC8210-10-2 | the router-cache setup moved from Section 8 to Section 10, which keeps the obligation to mark held data as to source because later updates must affect the correct data |
| [`RFC6810-8-2`](#rfc6810-8-2) If data from multiple caches are held, implementations MUST NOT distinguish between data sources when performing validation (Section 8) | restated | RFC8210-10-1 | Section 10 keeps the prohibition on distinguishing between data sources when performing validation of BGP announcements |
| [`RFC6810-7.2-1`](#rfc6810-7.2-1) TLS client routers MUST present client-side certificates (Section 7.2) | restated | RFC8210-9.2-1 | the TLS transport moved to Section 9.2, which keeps the obligation on a client router to present client-side certificates |
| [`RFC6810-7.2-2`](#rfc6810-7.2-2) TLS certificates MUST include subjectAltName extension with iPAddress identities (Section 7.2) | restated | RFC8210-9.2-2 | Section 9.2 keeps the obligation that a client router certificate include a subjectAltName extension carrying one or more iPAddress identities |
| [`RFC6810-7.2-3`](#rfc6810-7.2-3) Cache MUST check IP address of TLS connection against iPAddress identities (Section 7.2) | restated | RFC8210-9.2-3 | Section 9.2 keeps the obligation on the cache to check the IP address of the TLS connection against those iPAddress identities |
| [`RFC6810-7.2-4`](#rfc6810-7.2-4) Routers MUST verify the cache's TLS server certificate using subjectAltName dNSName identities (Section 7.2) | restated | RFC8210-9.2-4 | Section 9.2 keeps the obligation on routers to verify the cache's TLS server certificate using subjectAltName dNSName identities as described in RFC 6125 |
| [`RFC6810-7.2-5`](#rfc6810-7.2-5) DNS-ID identifier type support is REQUIRED in rpki-rtr TLS implementations (Section 7.2) | unextracted | §9.2 | RFC 8210 Section 9.2 keeps the sentence word for word, that support for the DNS-ID identifier type is REQUIRED in rpki-rtr server and client implementations which use TLS. rfc/short/rfc8210.md declares rows for the Certification Authority half and for the certificate half, at RFC8210-9.2-9 and RFC8210-9.2-6, and none for the implementation-support half |
| [`RFC6810-7.2-6`](#rfc6810-7.2-6) DNS-ID identifier type MUST be present in rpki-rtr server certificates (Section 7.2) | restated | RFC8210-9.2-6 | Section 9.2 keeps the obligation that the DNS-ID identifier type be present in rpki-rtr server certificates |
| [`RFC6810-7.2-7`](#rfc6810-7.2-7) rpki-rtr TLS implementations MUST NOT use CN-ID identifiers for authentication (Section 7.2) | restated | RFC8210-9.2-5 | Section 9.2 keeps the prohibition on CN-ID identifiers, and states its second half, that a CN field present in the subject name is not used for authentication, at RFC8210-9.2-10 |
| [`RFC6810-7.2-8`](#rfc6810-7.2-8) The client router MUST set its "reference identifier" to the DNS name of the rpki-rtr cache (Section 7.2) | restated | RFC8210-9.2-8 | Section 9.2 keeps the obligation on the client router to set its reference identifier to the DNS name of the rpki-rtr cache |
| [`RFC6810-7.3-1`](#rfc6810-7.3-1) TCP MD5 implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.3) | restated | RFC8210-9.3-1 | the TCP MD5 transport moved to Section 9.3, which keeps the key length of at least 80 printable ASCII bytes |
| [`RFC6810-7.3-2`](#rfc6810-7.3-2) TCP MD5 implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.3) | restated | RFC8210-9.3-2 | Section 9.3 keeps the obligation to support hexadecimal sequences of at least 32 characters, that is 128 bits |
| [`RFC6810-7.4-1`](#rfc6810-7.4-1) TCP-AO implementations MUST support key lengths of at least 80 printable ASCII bytes (Section 7.4) | restated | RFC8210-9.4-1 | the TCP-AO transport moved to Section 9.4, which keeps the key length of at least 80 printable ASCII bytes |
| [`RFC6810-7.4-2`](#rfc6810-7.4-2) TCP-AO implementations MUST support hexadecimal sequences of at least 32 characters (Section 7.4) | restated | RFC8210-9.4-2 | Section 9.4 keeps the obligation to support hexadecimal sequences of at least 32 characters, that is 128 bits |
| [`RFC6810-7.4-3`](#rfc6810-7.4-3) TCP-AO MAC lengths of at least 96 bits MUST be supported (Section 7.4) | restated | RFC8210-9.4-1 | Section 9.4 keeps the MAC length of at least 96 bits per Section 5.1 of RFC 5925, and rfc/short/rfc8210.md carries it in the same row as the key length |
| [`RFC6810-7.4-4`](#rfc6810-7.4-4) TCP-AO cryptographic algorithms described in RFC 5926 MUST be supported (Section 7.4) | restated | RFC8210-9.4-3 | Section 9.4 keeps the obligation to support the cryptographic algorithms and associated parameters described in RFC 5926 |
| [`RFC6810-6.2-2`](#rfc6810-6.2-2) Cache SHOULD send a Notify PDU when its serial changes (Section 6.2) | restated | RFC8210-8.2-2 | Section 8.2 keeps the SHOULD to send a Notify PDU carrying the current Serial Number when the cache's serial changes |
| [`RFC6810-5.6-2`](#rfc6810-5.6-2) Router receiving a duplicate announcement SHOULD raise Duplicate Announcement Received error (Section 5.6) | restated | RFC8210-5.6-2 | RFC 8210 Section 5.6 keeps the SHOULD to raise a Duplicate Announcement Received error, and Section 5.10 extends it to the new Router Key PDU |
| [`RFC6810-7-4`](#rfc6810-7-4) Caches and routers SHOULD use TCP-AO, SSHv2, TCP MD5, or IPsec transport (Section 7) | restated | RFC8210-9-5 | Section 9 keeps the SHOULD for TCP-AO alone and demotes the rest of the list to MAY, at RFC8210-9-6, adding TLS to it. RFC 6810 wrote one SHOULD over TCP-AO, SSHv2, TCP MD5 and IPsec |
| [`RFC6810-7-5`](#rfc6810-7-5) Both caches and routers SHOULD enable transport keep-alives (Section 7) | restated | RFC8210-9-4 | Section 9 keeps the SHOULD to enable keep-alives when they are available in the chosen transport protocol |
| [`RFC6810-6.3-2`](#rfc6810-6.3-2) Router SHOULD attempt more preferred caches on Cache Reset (Section 6.3) | restated | RFC8210-8.3-2 | Section 8.3 keeps the SHOULD to attempt a more-preferred cache on a Cache Reset |
| [`RFC6810-6.4-2`](#rfc6810-6.4-2) Router SHOULD attempt other caches on No Data Available (Section 6.4) | restated | RFC8210-8.4-2 | Section 8.4 keeps the SHOULD to attempt other caches in preference order on No Data Available |
| [`RFC6810-8-3`](#rfc6810-8-3) Client SHOULD retain data from previous cache during switchover (Section 8) | restated | RFC8210-10-4 | Section 10 keeps the SHOULD to retain the previous cache's data until a full set of data is held from one or more other caches |
| [`RFC6810-8-4`](#rfc6810-8-4) Client SHOULD delete data from unreachable cache after 2x polling period (Section 8) | restated | RFC8210-6-2 | RFC 8210 replaces the locally configured timer, whose default was twice the polling period, with the Expire Interval the cache sends in the End Of Data PDU, and RAISES the rule from a SHOULD to delete to a MUST NOT retain. Section 10 now defers to Section 6 for what to do when the client cannot refresh from a cache |
| [`RFC6810-11-1`](#rfc6810-11-1) Cache SHOULD be topologically close to router (Section 11) | unextracted | §13 | the Security Considerations moved from Section 11 to Section 13, which keeps both SHOULDs word for word, that the cache really SHOULD be as close as possible in the sense of controlled and protected transport, and SHOULD be topologically close. rfc/short/rfc8210.md declares no row for either |
| [`RFC6810-5.10-5`](#rfc6810-5.10-5) If erroneous Error Report PDU received, session SHOULD be dropped (Section 5.10) | restated | RFC8210-5.11-5 | the Error Report PDU moved to Section 5.11 and keeps the SHOULD to drop the session on an erroneous Error Report PDU |
| [`RFC6810-7.1-3`](#rfc6810-7.1-3) Cache servers supporting SSH SHOULD accept ECDSA authentication (Section 7.1) | restated | RFC8210-9.1-3 | Section 9.1 keeps the SHOULD to accept ECDSA authentication |
| [`RFC6810-7.1-4`](#rfc6810-7.1-4) Client routers SHOULD verify the public key of the cache to avoid MITM attacks (Section 7.1) | restated | RFC8210-9.1-4 | Section 9.1 keeps the SHOULD for the client router to verify the public key of the cache against a monkey-in-the-middle attack |
| [`RFC6810-7.2-9`](#rfc6810-7.2-9) TLS cache SHOULD reject connections if no iPAddress identities match (Section 7.2) | unextracted | §9.2 | RFC 8210 Section 9.2 keeps the clause word for word, that the cache SHOULD reject the connection if none of the iPAddress identities match the connection. rfc/short/rfc8210.md declares a row for the MUST check that precedes it, RFC8210-9.2-3, and none for this SHOULD |
| [`RFC6810-7.2-10`](#rfc6810-7.2-10) DNS names in rpki-rtr server certificates SHOULD NOT contain the wildcard character "*" (Section 7.2) | restated | RFC8210-9.2-7 | Section 9.2 keeps the SHOULD NOT on the wildcard character in a DNS name in an rpki-rtr server certificate |
| [`RFC6810-7-6`](#rfc6810-7-6) Operators SHOULD use procedural means (e.g., ACLs) to reduce exposure to authentication issues when using unprotected TCP (Section 7) | unextracted | §9 | RFC 8210 Section 9 keeps the sentence word for word, that operators SHOULD use procedural means, for example access control lists, to reduce the exposure to authentication issues. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-7.3-3`](#rfc6810-7.3-3) Cache servers SHOULD support RFC 4808 for TCP MD5 key rollover (Section 7.3) | restated | RFC8210-9.3-3 | Section 9.3 keeps the SHOULD for a cache server to support RFC 4808, because key rollover with TCP MD5 stays problematic |
| [`RFC6810-8-5`](#rfc6810-8-5) The client SHOULD attempt to maintain at least one set of data regardless of cache changes (Section 8) | restated | RFC8210-10-3 | Section 10 keeps the SHOULD to maintain at least one set of data regardless of a cache change or a new connection |
| [`RFC6810-11-2`](#rfc6810-11-2) The identity of the cache server SHOULD be verified and authenticated by the router client, and vice versa (Section 11) | restated | RFC8210-11-1 | the Security Considerations moved to Section 13, which keeps the sentence word for word, that the identity of the cache server SHOULD be verified and authenticated by the router client and vice versa before any data are exchanged. The rfc/short/rfc8210.md row cites §11, which is that summary's own error and not a change in the document |
| [`RFC6810-11-3`](#rfc6810-11-3) Protocols that provide integrity and authenticity SHOULD be used for cache-router transport (Section 11) | unextracted | §13 | the Security Considerations moved to Section 13, which keeps the sentence word for word, that protocols which provide integrity and authenticity SHOULD be used, and that if they cannot be, the router and cache MUST be on the same trusted controlled network. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-10-1`](#rfc6810-10-1) Fatal errors SHOULD cause the session to be dropped (Section 10) | restated | RFC8210-12-1 | the error codes moved from Section 10 to Section 12, and RFC 8210 RAISES the rule, from errors considered fatal SHOULD cause the session to be dropped to MUST cause the session to be dropped |
| [`RFC6810-6.1-2`](#rfc6810-6.1-2) Router MAY start with Serial Query if it has unexpired data from a previous session (Section 6.1) | restated | RFC8210-8.1-2 | RFC 8210 Section 8.1 turns the pair of permissions into one obligation, that on a new transport connection the router MUST send either a Reset Query or a Serial Query, and keeps the unexpired-data condition as indicative prose, adding that the router MUST fall back to a Reset Query in all other cases |
| [`RFC6810-5.10-6`](#rfc6810-5.10-6) Truncated Erroneous PDU MAY be included for oversized PDUs (Section 5.10) | restated | RFC8210-5.11-6 | the Error Report PDU moved to Section 5.11 and keeps the permission to truncate the Erroneous PDU field for an excessively long PDU |
| [`RFC6810-5-2`](#rfc6810-5-2) Fields with unspecified content MAY be ignored on receipt (Section 5) | restated | RFC8210-5-1 | RFC 8210 Section 5 states both halves in one sentence and RAISES this one, from MAY be ignored on receipt to MUST be ignored on receipt |
| [`RFC6810-7-7`](#rfc6810-7-7) Routers MAY use SSHv2, TCP MD5, IPsec, or TLS transport (Section 7) | restated | RFC8210-9-6 | Section 9 keeps the permission and adds TLS on port rpki-rtr-tls 324 to the list |
| [`RFC6810-7.1-5`](#rfc6810-7.1-5) SSH host authentication MAY be supported (Section 7.1) | unextracted | §9.1 | RFC 8210 Section 9.1 keeps the sentence word for word, that host authentication MAY be supported. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-7.1-6`](#rfc6810-7.1-6) SSH implementations MAY support password authentication (Section 7.1) | unextracted | §9.1 | RFC 8210 Section 9.1 keeps the sentence word for word, that implementations MAY support password authentication. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-8-6`](#rfc6810-8-6) A client MAY drop the data from a particular cache when it is fully in sync with other caches (Section 8) | unextracted | §10 | RFC 8210 Section 10 keeps the sentence word for word, that a client MAY drop the data from a particular cache when it is fully in sync with one or more other caches. rfc/short/rfc8210.md declares no row for it |
| [`RFC6810-3-2`](#rfc6810-3-2) There MAY be mechanisms for the router to assure authenticity of the cache (Section 3) | unextracted | §3 | RFC 8210 Section 3 keeps the sentence word for word, that there MAY be mechanisms for the router to assure itself of the authenticity of the cache and to authenticate itself to the cache. rfc/short/rfc8210.md declares no row for it |
