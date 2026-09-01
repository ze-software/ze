# RFC 8907 - The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol

Partial. Every requirement this repository extracted from RFC 8907, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 27.3% | 3 of 11 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 54.5% | 6 of 11 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.2% | 2 of 11 binding obligations | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 16 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 11 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8907.md` |
| Requirement shard | `rfc/requirements/rfc8907.md` |
| RFC text | `rfc/full/rfc8907.txt` |

## Enrolment

Enrolled: The TACACS+ Protocol (ze as a TACACS+ client/NAS): eleven MUST-level requirements. Eight are met in internal/component/tacacs: 4-1 (the major version nibble is 0xC, other versions rejected) and 4-2 (the sequence number is client-odd and the reply is request+1) carry positive+negative tags; 4-3 (the session id is drawn from a cryptographic RNG) is {single-polarity: positive} with a new uniqueness test; 4-4 (one session id for the whole session), 4-5 (multi-octet header fields are network byte order), 7-1 and 7-2 (an accounting REQUEST sets only Start or Stop, never the MORE flag), and 10-1 (the Unencrypted flag is never set when a key is configured) are {single-polarity: positive}. 5-1 (sequence-number wrap ends the session) is {not-applicable}: ze runs a single-exchange PAP authentication with no CONTINUE loop, so the sequence never approaches 0xFE. Two are {gap}: 4.6-1 (no exact decrypted-body-length check, so a wrong shared secret yielding a plausibly-sized body is not cleanly rejected) and 6-1 (authorization is decided on the response Status alone; mandatory response arguments are never parsed). Disclosed in the docs/features/rfc-status.md RFC 8907 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- SSH login PAP auth, ordered failover, MD5 pseudo-pad encryption, command accounting, optional authorization, single-connect mode
- tests bound per requirement in [`rfc/requirements/rfc8907.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8907.md).


**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc8907.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8907.md): [`RFC8907-4.6-1`](#rfc8907-4.6-1) -- no exact decrypted-body-length check, so a wrong shared secret yielding a plausibly-sized body is not cleanly rejected (ErrBadSecret is defined but unused); and [`RFC8907-6-1`](#rfc8907-6-1) -- authorization is decided on the response Status alone, and mandatory response arguments (=/*) are never parsed or enforced.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC8907-4-1`](#rfc8907-4-1), [`RFC8907-4-2`](#rfc8907-4-2), [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1)

**Annotated instead of tested (9):** [`RFC8907-4-3`](#rfc8907-4-3), [`RFC8907-4-4`](#rfc8907-4-4), [`RFC8907-4-5`](#rfc8907-4-5), [`RFC8907-4.6-1`](#rfc8907-4.6-1), [`RFC8907-5-1`](#rfc8907-5-1), [`RFC8907-7-1`](#rfc8907-7-1), [`RFC8907-7-2`](#rfc8907-7-2), [`RFC8907-10-1`](#rfc8907-10-1), [`RFC8907-6-1`](#rfc8907-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8907-4-1` | Major version MUST be 0xC (12 decimal) (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L146). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L241) |
| `RFC8907-4-2` | Client sends odd seq_no, server sends even seq_no (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L147). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L250) |
| `RFC8907-4-3` | session_id MUST be cryptographically random (§4, Session Lifecycle) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8907SessionIDComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_sessionid_test.go#L38). **positive:** `unit/verify` [`TestRandomSessionIDDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L332). **negative:** no negative test. **{single-polarity}:** the session id is drawn from crypto/rand (internal/component/tacacs/client.go:497-503) with no predictable/reject path |
| `RFC8907-4-4` | session_id MUST remain constant for entire session (§4, Session Lifecycle) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L148). **negative:** no negative test. **{single-polarity}:** ze generates one session id at internal/component/tacacs/client.go:209 and reuses it for the single request/reply exchange; the reply mismatch guard (client.go:354-358) enforces constancy and the positive path exercises it, but a single-exchange client emits no second packet whose id could differ, so there is no client-side constancy-violation to test |
| `RFC8907-4-5` | Body length field MUST be in network byte order (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L14). **negative:** no negative test. **{single-polarity}:** multi-octet header fields (session_id, length) are written and read with binary.BigEndian at internal/component/tacacs/packet.go:75-76 and 90-91; the marshal/unmarshal round-trip is symmetric with no independent little-endian oracle, so a negative would only test a different codec |
| `RFC8907-4.6-1` | After decryption, unmarshalled field lengths must sum to header's body length; mismatch indicates wrong shared secret (§4.6, Body Encryption) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze detects only gross truncation of the TACACS+ body (>= checks at internal/component/tacacs/authen.go:137-141, author.go:140-143, acct.go:136-140) and does not verify the decrypted body length exactly matches the header length; the ErrBadSecret error (internal/component/tacacs/packet.go:97) is defined but unused, so a wrong shared secret that yields a plausibly-sized body is not cleanly rejected |
| `RFC8907-5-1` | Max sequence number is 0xFE (254); if reached, session MUST abort (§5, Authentication) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TACACS+ client performs a single-exchange PAP authentication (internal/component/tacacs/client.go:219 sends seq 1 and expects seq 2, internal/component/tacacs/authen.go NewPAPAuthenStart with no CONTINUE loop), so the sequence number never approaches 0xFE and the ErrSeqOverflow guard (packet.go:99) has no reachable code path |
| `RFC8907-7-1` | Accounting flag MORE (0x01) is deprecated and MUST NOT be set (§7, Accounting) | MUST NOT | 7 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L13). **negative:** no negative test. **{single-polarity}:** ze builds accounting requests with Flags set to AcctFlagStart or AcctFlagStop only (internal/component/tacacs/accounting.go:171 and 198); the deprecated MORE bit is never emitted, so there is no code path that sets it to drive a negative against |
| `RFC8907-7-2` | START and STOP accounting flags are mutually exclusive (§7, Accounting) | MUST | 7 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L14). **negative:** no negative test. **{single-polarity}:** ze emits Flags as exactly AcctFlagStart (0x02) or AcctFlagStop (0x04) at internal/component/tacacs/accounting.go:171 and 198, never combined; a Start-plus-Stop combination is unreachable by construction, so there is no negative to test |
| `RFC8907-10-1` | Unencrypted mode (flag 0x01) MUST only be used with TLS (§10, Security) | MUST | 10 | **positive:** `unit/verify` [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L127). **positive:** `unit/verify` [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137). **negative:** no negative test. **{single-polarity}:** the client never sets TAC_PLUS_UNENCRYPTED_FLAG and never emits a readable body: MarshalInto (internal/component/tacacs/packet.go MarshalInto) obfuscates whenever a key is configured and the client only ORs FlagSingleConnect (internal/component/tacacs/client.go trySend), so the emission is judged on the octets the client puts on the connection (TestRFC8907ClientNeverSendsUnobfuscatedBody); no configuration reaches an unencrypted send, so there is no negative path |
| `RFC8907-10.5.2-1` | A client that receives a reply whose obfuscation state disagrees with the shared-secret configuration of the server it came from MUST close the TCP session and process the reply as a FAIL (§10.5.2, Connections and Obfuscation) | MUST | 10.5.2 | **positive:** `unit/verify` [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L221). **negative:** `unit/verify` [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L194) |
| `RFC8907-6-1` | Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decides authorization on the response Status alone (internal/component/tacacs/authorizer.go:116-135); AuthorResponse.Args is unmarshalled (author.go:171-175) but never inspected, and no attribute-value =/* separator parsing exists, so a server PASS carrying an unknown mandatory argument is honored rather than rejected |
| `RFC8907-4.6-2` | Servers SHOULD reject unencrypted packets unless explicitly configured (§4.6, Body Encryption) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-x-1` | Server SHOULD use a configurable connection timeout (Connection Management) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-x-2` | Clients SHOULD NOT follow FOLLOW (redirect) responses unless explicitly configured (Error Handling) | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6-2` | Authorization argument separator `*` (asterisk): client MAY ignore optional attributes if not understood (§6, Authorization) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8907-4.6-1`](#rfc8907-4.6-1) After decryption, unmarshalled field lengths must sum to header's body length; mismatch indicates wrong shared secret (§4.6, Body Encryption) | {gap}, no test | ze detects only gross truncation of the TACACS+ body (>= checks at internal/component/tacacs/authen.go:137-141, author.go:140-143, acct.go:136-140) and does not verify the decrypted body length exactly matches the header length; the ErrBadSecret error (internal/component/tacacs/packet.go:97) is defined but unused, so a wrong shared secret that yields a plausibly-sized body is not cleanly rejected |
| [`RFC8907-5-1`](#rfc8907-5-1) Max sequence number is 0xFE (254); if reached, session MUST abort (§5, Authentication) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TACACS+ client performs a single-exchange PAP authentication (internal/component/tacacs/client.go:219 sends seq 1 and expects seq 2, internal/component/tacacs/authen.go NewPAPAuthenStart with no CONTINUE loop), so the sequence number never approaches 0xFE and the ErrSeqOverflow guard (packet.go:99) has no reachable code path |
| [`RFC8907-6-1`](#rfc8907-6-1) Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization) | {gap}, no test | ze decides authorization on the response Status alone (internal/component/tacacs/authorizer.go:116-135); AuthorResponse.Args is unmarshalled (author.go:171-175) but never inspected, and no attribute-value =/* separator parsing exists, so a server PASS carrying an unknown mandatory argument is honored rather than rejected |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8907-4-1`](#rfc8907-4-1)

Major version MUST be 0xC (12 decimal) (§4, Packet Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L241) | unit/verify | unproven |
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L146) | unit/verify | unproven |

### [`RFC8907-4-2`](#rfc8907-4-2)

Client sends odd seq_no, server sends even seq_no (§4, Packet Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L250) | unit/verify | unproven |
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L147) | unit/verify | unproven |

### [`RFC8907-4-3`](#rfc8907-4-3)

session_id MUST be cryptographically random (§4, Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRandomSessionIDDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L332) | unit/verify | unproven |
| positive | [`TestRFC8907SessionIDComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_sessionid_test.go#L38) | unit/verify | unproven |

### [`RFC8907-4-4`](#rfc8907-4-4)

session_id MUST remain constant for entire session (§4, Session Lifecycle)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L148) | unit/verify | unproven |

### [`RFC8907-4-5`](#rfc8907-4-5)

Body length field MUST be in network byte order (§4, Packet Header)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L14) | unit/verify | unproven |

### [`RFC8907-4.6-1`](#rfc8907-4.6-1)

After decryption, unmarshalled field lengths must sum to header's body length; mismatch indicates wrong shared secret (§4.6, Body Encryption)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.6-1, so no unit is bound to it.

### [`RFC8907-5-1`](#rfc8907-5-1)

Max sequence number is 0xFE (254); if reached, session MUST abort (§5, Authentication)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5-1, so no unit is bound to it.

### [`RFC8907-7-1`](#rfc8907-7-1)

Accounting flag MORE (0x01) is deprecated and MUST NOT be set (§7, Accounting)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L13) | unit/verify | unproven |

### [`RFC8907-7-2`](#rfc8907-7-2)

START and STOP accounting flags are mutually exclusive (§7, Accounting)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L14) | unit/verify | unproven |

### [`RFC8907-10-1`](#rfc8907-10-1)

Unencrypted mode (flag 0x01) MUST only be used with TLS (§10, Security)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137) | unit/verify | unproven |

### [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1)

A client that receives a reply whose obfuscation state disagrees with the shared-secret configuration of the server it came from MUST close the TCP session and process the reply as a FAIL (§10.5.2, Connections and Obfuscation)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L194) | unit/verify | unproven |
| positive | [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L221) | unit/verify | unproven |

### [`RFC8907-6-1`](#rfc8907-6-1)

Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8907, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8907, so its obligations are stated where they were written.
