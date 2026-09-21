# RFC 8907 - The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol

Partial. Every requirement this repository extracted from RFC 8907, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.1% | 4 of 56 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.7% | 6 of 56 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 56 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 11.8% | 2 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 56 | of 59 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 56 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 1.8% | 1 of 56 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 56 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 56 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 80.4% | 45 of 56 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 56 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 59 |
| Gated MUST-level | 56 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 2 |
| Gated with no test | 43 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc8907.md` |
| Requirement shard | `rfc/requirements/rfc8907.md` |
| RFC text | `rfc/full/rfc8907.txt` |

## Enrolment

Enrolled: The TACACS+ Protocol (ze as a TACACS+ client/NAS): fifty-five MUST-level requirements. Thirteen were carried at enrolment, and ten of those are met in internal/component/tacacs: 4-1 (the major version nibble is 0xC, other versions rejected) and 4-2 (the sequence number is client-odd and the reply is request+1) carry positive+negative tags; 4-3 (the session id is drawn from a cryptographic RNG) is {single-polarity: positive} with a new uniqueness test; 4-4 (one session id for the whole session), 4-5 (multi-octet header fields are network byte order), 7-1 and 7-2 (an accounting REQUEST sets only Start or Stop, never the MORE flag), and 10-1 (the Unencrypted flag is never set when a key is configured) are {single-polarity: positive}. 10.5.2-1 (a reply whose obfuscation state disagrees with the shared-secret configuration of the server it came from closes the TCP session and reads as a FAIL) and 10-2 (a client never sets TAC_PLUS_UNENCRYPTED_FLAG) carry positive+negative tags. 5-1 (sequence-number wrap ends the session) is {not-applicable}: ze runs a single-exchange PAP authentication with no CONTINUE loop, so the sequence never approaches 0xFE. Two are {gap}: 4.6-1 (no exact decrypted-body-length check, so a wrong shared secret yielding a plausibly-sized body is not cleanly rejected) and 6-1 (authorization is decided on the response Status alone; mandatory response arguments are never parsed). The 2026-09-21 extraction walk added forty-two MUST-level rows for sentences this summary did not carry, drawn from §3.7, §4.1, §4.3, §4.4, §5.1, §5.4, §5.4.2.2 through §5.4.3, §6.1, §6.2, §7.2, §8 through §8.3, §10.5, §10.5.1 and §10.5.4. None of the forty-two carries a test, so each is an open gap on this ledger. Disclosed in the docs/features/rfc-status.md RFC 8907 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- SSH login PAP auth, ordered failover, MD5 pseudo-pad encryption, command accounting, optional authorization, single-connect mode
- tests bound per requirement in [`rfc/requirements/rfc8907.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8907.md).


**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc8907.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8907.md): [`RFC8907-4.6-1`](#rfc8907-4.6-1) -- no exact decrypted-body-length check, so a wrong shared secret yielding a plausibly-sized body is not cleanly rejected (ErrBadSecret is defined but unused); and [`RFC8907-6-1`](#rfc8907-6-1) -- authorization is decided on the response Status alone, and mandatory response arguments (=/*) are never parsed or enforced. Forty-two further MUST rows were added by the 2026-09-21 extraction walk and none is tested: the text-string profile (§3.7), the header reading rules (§4.1), Single Connection Mode client behavior (§4.3), ERROR handling and connection close (§4.4), the authentication flows (§5.1 to §5.4.3, of which only PAP is implemented, so the CHAP and MS-CHAP rows are unbuilt features), the authorization request and reply rules (§6.1, §6.2), the accounting flag rule (§7.2), the argument dictionary (§8 to §8.3) and the best practices binding a client (§10.5, §10.5.1, §10.5.4).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 43 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **56** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC8907-4-1`](#rfc8907-4-1), [`RFC8907-4-2`](#rfc8907-4-2), [`RFC8907-10-2`](#rfc8907-10-2), [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1)

**Annotated instead of tested (9):** [`RFC8907-4-3`](#rfc8907-4-3), [`RFC8907-4-4`](#rfc8907-4-4), [`RFC8907-4-5`](#rfc8907-4-5), [`RFC8907-4.6-1`](#rfc8907-4.6-1), [`RFC8907-5-1`](#rfc8907-5-1), [`RFC8907-7-1`](#rfc8907-7-1), [`RFC8907-7-2`](#rfc8907-7-2), [`RFC8907-10-1`](#rfc8907-10-1), [`RFC8907-6-1`](#rfc8907-6-1)

**No test and no annotation (43):** [`RFC8907-3.7-1`](#rfc8907-3.7-1), [`RFC8907-3.7-2`](#rfc8907-3.7-2), [`RFC8907-4.1-1`](#rfc8907-4.1-1), [`RFC8907-4.1-2`](#rfc8907-4.1-2), [`RFC8907-4.1-3`](#rfc8907-4.1-3), [`RFC8907-4.3-1`](#rfc8907-4.3-1), [`RFC8907-4.3-2`](#rfc8907-4.3-2), [`RFC8907-4.3-3`](#rfc8907-4.3-3), [`RFC8907-4.4-1`](#rfc8907-4.4-1), [`RFC8907-4.4-2`](#rfc8907-4.4-2), [`RFC8907-4.4-3`](#rfc8907-4.4-3), [`RFC8907-5.1-1`](#rfc8907-5.1-1), [`RFC8907-5.4-1`](#rfc8907-5.4-1), [`RFC8907-5.4-2`](#rfc8907-5.4-2), [`RFC8907-5.4.2.2-1`](#rfc8907-5.4.2.2-1), [`RFC8907-5.4.2.2-2`](#rfc8907-5.4.2.2-2), [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1), [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1), [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1), [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1), [`RFC8907-5.4.2.6-2`](#rfc8907-5.4.2.6-2), [`RFC8907-5.4.3-1`](#rfc8907-5.4.3-1), [`RFC8907-6.1-1`](#rfc8907-6.1-1), [`RFC8907-6.1-2`](#rfc8907-6.1-2), [`RFC8907-6.2-1`](#rfc8907-6.2-1), [`RFC8907-6.2-2`](#rfc8907-6.2-2), [`RFC8907-6.2-3`](#rfc8907-6.2-3), [`RFC8907-7.2-1`](#rfc8907-7.2-1), [`RFC8907-8-1`](#rfc8907-8-1), [`RFC8907-8.1-1`](#rfc8907-8.1-1), [`RFC8907-8.1-2`](#rfc8907-8.1-2), [`RFC8907-8.2-1`](#rfc8907-8.2-1), [`RFC8907-8.2-2`](#rfc8907-8.2-2), [`RFC8907-8.3-1`](#rfc8907-8.3-1), [`RFC8907-8.3-2`](#rfc8907-8.3-2), [`RFC8907-8.3-3`](#rfc8907-8.3-3), [`RFC8907-8.3-4`](#rfc8907-8.3-4), [`RFC8907-8.3-5`](#rfc8907-8.3-5), [`RFC8907-10.5-1`](#rfc8907-10.5-1), [`RFC8907-10.5.1-1`](#rfc8907-10.5.1-1), [`RFC8907-10.5.1-2`](#rfc8907-10.5.1-2), [`RFC8907-10.5.4-1`](#rfc8907-10.5.4-1), [`RFC8907-x-2`](#rfc8907-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8907-4-1` | Major version MUST be 0xC (12 decimal) (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L146). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L241) |
| `RFC8907-4-2` | Client sends odd seq_no, server sends even seq_no (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L147). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L250) |
| `RFC8907-4-3` | session_id MUST be cryptographically random (§4, Session Lifecycle) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8907SessionIDComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_sessionid_test.go#L38). **positive:** `unit/verify` [`TestRandomSessionIDDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L332). **negative:** no negative test. **{single-polarity}:** the session id is drawn from crypto/rand (internal/component/tacacs/client.go:497-503) with no predictable/reject path |
| `RFC8907-4-4` | session_id MUST remain constant for entire session (§4, Session Lifecycle) | MUST | 4 | **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/client_test.go#L148). **negative:** no negative test. **{single-polarity}:** ze generates one session id at internal/component/tacacs/client.go:209 and reuses it for the single request/reply exchange; the reply mismatch guard (client.go:354-358) enforces constancy and the positive path exercises it, but a single-exchange client emits no second packet whose id could differ, so there is no client-side constancy-violation to test |
| `RFC8907-4-5` | Body length field MUST be in network byte order (§4, Packet Header) | MUST | 4 | **positive:** `unit/verify` [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L15). **negative:** no negative test. **{single-polarity}:** multi-octet header fields (session_id, length) are written and read with binary.BigEndian at internal/component/tacacs/packet.go:75-76 and 90-91; the marshal/unmarshal round-trip is symmetric with no independent little-endian oracle, so a negative would only test a different codec |
| `RFC8907-4.6-1` | After decryption, unmarshalled field lengths must sum to header's body length; mismatch indicates wrong shared secret (§4.6, Body Encryption) | MUST | 4.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze detects only gross truncation of the TACACS+ body (>= checks at internal/component/tacacs/authen.go:137-141, author.go:140-143, acct.go:136-140) and does not verify the decrypted body length exactly matches the header length; the ErrBadSecret error (internal/component/tacacs/packet.go:97) is defined but unused, so a wrong shared secret that yields a plausibly-sized body is not cleanly rejected |
| `RFC8907-5-1` | Max sequence number is 0xFE (254); if reached, session MUST abort (§5, Authentication) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TACACS+ client performs a single-exchange PAP authentication (internal/component/tacacs/client.go:219 sends seq 1 and expects seq 2, internal/component/tacacs/authen.go NewPAPAuthenStart with no CONTINUE loop), so the sequence number never approaches 0xFE and the ErrSeqOverflow guard (packet.go:99) has no reachable code path |
| `RFC8907-7-1` | Accounting flag MORE (0x01) is deprecated and MUST NOT be set (§7, Accounting) | MUST NOT | 7 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L13). **negative:** no negative test. **{single-polarity}:** ze builds accounting requests with Flags set to AcctFlagStart or AcctFlagStop only (internal/component/tacacs/accounting.go:171 and 198); the deprecated MORE bit is never emitted, so there is no code path that sets it to drive a negative against |
| `RFC8907-7-2` | START and STOP accounting flags are mutually exclusive (§7, Accounting) | MUST | 7 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/acct_test.go#L14). **negative:** no negative test. **{single-polarity}:** ze emits Flags as exactly AcctFlagStart (0x02) or AcctFlagStop (0x04) at internal/component/tacacs/accounting.go:171 and 198, never combined; a Start-plus-Stop combination is unreachable by construction, so there is no negative to test |
| `RFC8907-10-1` | §10.5.2: "Clients MUST be implemented in a way that requires explicit configuration to enable the use of TAC_PLUS_UNENCRYPTED_FLAG." (§10, Security) | MUST | 10 | **positive:** `unit/verify` [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L128). **positive:** `unit/verify` [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137). **positive:** `unit/verify` [`TestRFC8907UnencryptedModeIsNotReachableFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_unencrypted_config_test.go#L93). **negative:** no negative test. **{single-polarity}:** ze exposes no configuration that enables TAC_PLUS_UNENCRYPTED_FLAG: the schema declares no leaf for it and makes the shared secret mandatory (internal/component/tacacs/yang/ze-tacacs-conf.yang), the only request flag any producer sets is FlagSingleConnect (internal/component/tacacs/client.go trySend), and (*Packet).MarshalInto (internal/component/tacacs/packet.go) refuses a marshal with no shared secret instead of falling back to an unencrypted send, so there is no enabled state to drive a negative against |
| `RFC8907-10-2` | "TACACS+ clients MUST NOT set TAC_PLUS_UNENCRYPTED_FLAG." (§10, Security) | MUST NOT | 10 | **positive:** `unit/verify` [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L142). **negative:** `unit/verify` [`TestPacketMarshalNoEncryption`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L167) |
| `RFC8907-10.5.2-1` | A client that receives a reply whose obfuscation state disagrees with the shared-secret configuration of the server it came from MUST close the TCP session and process the reply as a FAIL (§10.5.2, Connections and Obfuscation) | MUST | 10.5.2 | **positive:** `unit/verify` [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L225). **negative:** `unit/verify` [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L198) |
| `RFC8907-6-1` | Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decides authorization on the response Status alone (internal/component/tacacs/authorizer.go:116-135); AuthorResponse.Args is unmarshalled (author.go:171-175) but never inspected, and no attribute-value =/* separator parsing exists, so a server PASS carrying an unknown mandatory argument is honored rather than rejected |
| `RFC8907-3.7-1` | "Usernames MUST be encoded and handled using the UsernameCasePreserved Profile specified in [RFC8265]." (§3.7, Treatment of Text Strings) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-3.7-2` | "All other text fields in TACACS+ MUST be treated as printable byte arrays of US-ASCII as defined by [RFC0020]", and printable "MUST exclude the 'Control Characters' defined in Section 5.2 of [RFC0020]" (§3.7, Treatment of Text Strings) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.1-1` | A variable-length field whose length value is zero is unused, and such fields "MUST be ignored, and treated as if not present" (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.1-2` | "The first packet in a session MUST have the sequence number 1" (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.1-3` | Header flag bits other than TAC_PLUS_UNENCRYPTED_FLAG and TAC_PLUS_SINGLE_CONNECT_FLAG "MUST be ignored when reading, and SHOULD be set to zero when writing" (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.3-1` | "The client MUST NOT send a second packet on a connection until single-connect status has been established." (§4.3, Single Connection Mode) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.3-2` | "the client and server MUST ignore the flag after the second packet on a connection" (§4.3, Single Connection Mode) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.3-3` | "The client MUST accommodate such closures on a TCP session even after Single Connection Mode has been established." (§4.3, Single Connection Mode) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.4-1` | On a REPLY of ERROR "The client cannot apply the result, and it MUST behave as if the server could not be connected to." (§4.4, Session Completion) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.4-2` | After an ERROR caused by connection issues on a Single Connection Mode connection, "any further new sessions MUST NOT be accepted on the connection" (§4.4, Session Completion) | MUST NOT | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.4-3` | "Once all active sessions are completed, then the connection MUST be closed." (§4.4, Session Completion) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.1-1` | The username is optional in the START packet; "If it is absent, the client MUST set user_len to 0." (§5.1, The Authentication START Packet Body) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4-1` | On a REPLY of GETDATA, GETUSER or GETPASS, "The client MUST then return a CONTINUE packet containing the requested information in the user_msg field." (§5.4, Description of Authentication Process) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4-2` | When the client queries the user for information the server marked NOECHO, "the response MUST NOT be reflected in the user interface as it is entered" (§5.4, Description of Authentication Process) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.2-1` | A PAP login exchange "MUST consist of a single START packet and a single REPLY" (§5.4.2.2, PAP Login) | MUST | 5.4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.2-2` | "The START packet MUST contain a username and the data field MUST contain the PAP ASCII password." (§5.4.2.2, PAP Login) | MUST | 5.4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.3-1` | A CHAP login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response." (§5.4.2.3, CHAP Login) | MUST | 5.4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.4-1` | An MS-CHAP v1 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.4, MS-CHAP v1 Login) | MUST | 5.4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.5-1` | An MS-CHAP v2 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.5, MS-CHAP v2 Login) | MUST | 5.4.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.6-1` | "the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE" (§5.4.2.6, Enable Requests) | MUST | 5.4.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.2.6-2` | authen_service "MUST NOT be set to this value when requesting any other operation" (§5.4.2.6, Enable Requests) | MUST NOT | 5.4.2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-5.4.3-1` | "If a client does not implement the TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL." (§5.4.3, Aborting an Authentication Session) | MUST | 5.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6.1-1` | "The user_len MUST indicate the length of the user field, in bytes." (§6.1, The Authorization REQUEST Packet Body) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6.1-2` | "An argument name MUST NOT contain either of the separators." (§6.1, The Authorization REQUEST Packet Body) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6.2-1` | On TAC_PLUS_AUTHOR_STATUS_PASS_ADD "the arguments in the response MUST be applied according to the rules described above" (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6.2-2` | On TAC_PLUS_AUTHOR_STATUS_PASS_REPL "the client MUST use the authorization argument-value pairs (if any) in the response instead of the authorization argument-value pairs from the request" (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6.2-3` | On TAC_PLUS_AUTHOR_STATUS_FAIL "the requested authorization MUST be denied" (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-7.2-1` | "The STOP flag MUST NOT be set in conjunction with the WATCHDOG flag." (§7.2, The Accounting REPLY Packet Body) | MUST NOT | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8-1` | "Clients MUST use these arguments when supporting the corresponding use cases." (§8, Argument-Value Pairs) | MUST | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.1-1` | "TACACS+ implementations MUST verify that they can accommodate the lengths of numeric arguments before attempting to process them", and a length that cannot be accommodated means "the argument MUST be regarded as not handled and the logic in 'Authorization' (Section 6.1) regarding the processing of arguments MUST be applied" (§8.1, Value Encoding) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.1-2` | For an absolute date/time argument, "The time zone MUST be UTC unless a time zone argument is specified." (§8.1, Value Encoding) | MUST | 8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.2-1` | The "service" argument "MUST always be included" (§8.2, Authorization Arguments) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.2-2` | "The 'cmd' argument MUST be specified if service equals 'shell'." (§8.2, Authorization Arguments) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.3-1` | Accounting arguments "MUST precede any argument-value pairs that are defined in 'Authorization' (Section 6)" (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.3-2` | "Start and stop records for the same event MUST have matching task_id argument values." (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.3-3` | "The client MUST ensure that active task_ids are not duplicated; a client MUST NOT reuse a task_id in a start record until it has sent a stop record for that task_id." (§8.3, Accounting Arguments) | MUST NOT | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.3-4` | "TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized." (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-8.3-5` | "These 'Command Accounting' packets MUST include the 'service' and 'cmd' arguments, and if needed, the 'cmd-arg' arguments detailed in Section 8.2." (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-10.5-1` | "New implementations, and upgrades of current implementations, MUST implement these recommendations." (§10.5, TACACS+ Best Practices) | MUST | 10.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-10.5.1-1` | "TACACS+ servers and clients MUST treat shared secrets as sensitive data to be managed securely, as would be expected for other sensitive data such as identity credential information." (§10.5.1, Shared Secrets) | MUST | 10.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-10.5.1-2` | "TACACS+ servers and clients MUST support shared keys that are at least 32 characters long." (§10.5.1, Shared Secrets) | MUST | 10.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-10.5.4-1` | "administrators and implementers MUST ensure that the argument and value pairs shared between the clients and servers have consistent interpretation" (§10.5.4, Authorization) | MUST | 10.5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-4.6-2` | Servers SHOULD reject unencrypted packets unless explicitly configured (§4.6, Body Encryption) | SHOULD | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-x-1` | Server SHOULD use a configurable connection timeout (Connection Management) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-x-2` | The TAC_PLUS_AUTHEN_STATUS_FOLLOW redirection mechanism is deprecated, and "This mechanism MUST NOT be used in modern deployments. It MUST NOT be used outside a secured deployment." (Error Handling) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-6-2` | Authorization argument separator `*` (asterisk): client MAY ignore optional attributes if not understood (§6, Authorization) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8907-4.6-1`](#rfc8907-4.6-1) After decryption, unmarshalled field lengths must sum to header's body length; mismatch indicates wrong shared secret (§4.6, Body Encryption) | {gap}, no test | ze detects only gross truncation of the TACACS+ body (>= checks at internal/component/tacacs/authen.go:137-141, author.go:140-143, acct.go:136-140) and does not verify the decrypted body length exactly matches the header length; the ErrBadSecret error (internal/component/tacacs/packet.go:97) is defined but unused, so a wrong shared secret that yields a plausibly-sized body is not cleanly rejected |
| [`RFC8907-5-1`](#rfc8907-5-1) Max sequence number is 0xFE (254); if reached, session MUST abort (§5, Authentication) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TACACS+ client performs a single-exchange PAP authentication (internal/component/tacacs/client.go:219 sends seq 1 and expects seq 2, internal/component/tacacs/authen.go NewPAPAuthenStart with no CONTINUE loop), so the sequence number never approaches 0xFE and the ErrSeqOverflow guard (packet.go:99) has no reachable code path |
| [`RFC8907-6-1`](#rfc8907-6-1) Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization) | {gap}, no test | ze decides authorization on the response Status alone (internal/component/tacacs/authorizer.go:116-135); AuthorResponse.Args is unmarshalled (author.go:171-175) but never inspected, and no attribute-value =/* separator parsing exists, so a server PASS carrying an unknown mandatory argument is honored rather than rejected |
| [`RFC8907-3.7-1`](#rfc8907-3.7-1) "Usernames MUST be encoded and handled using the UsernameCasePreserved Profile specified in [RFC8265]." (§3.7, Treatment of Text Strings) | no test | no test carries this requirement id |
| [`RFC8907-3.7-2`](#rfc8907-3.7-2) "All other text fields in TACACS+ MUST be treated as printable byte arrays of US-ASCII as defined by [RFC0020]", and printable "MUST exclude the 'Control Characters' defined in Section 5.2 of [RFC0020]" (§3.7, Treatment of Text Strings) | no test | no test carries this requirement id |
| [`RFC8907-4.1-1`](#rfc8907-4.1-1) A variable-length field whose length value is zero is unused, and such fields "MUST be ignored, and treated as if not present" (§4.1, The TACACS+ Packet Header) | no test | no test carries this requirement id |
| [`RFC8907-4.1-2`](#rfc8907-4.1-2) "The first packet in a session MUST have the sequence number 1" (§4.1, The TACACS+ Packet Header) | no test | no test carries this requirement id |
| [`RFC8907-4.1-3`](#rfc8907-4.1-3) Header flag bits other than TAC_PLUS_UNENCRYPTED_FLAG and TAC_PLUS_SINGLE_CONNECT_FLAG "MUST be ignored when reading, and SHOULD be set to zero when writing" (§4.1, The TACACS+ Packet Header) | no test | no test carries this requirement id |
| [`RFC8907-4.3-1`](#rfc8907-4.3-1) "The client MUST NOT send a second packet on a connection until single-connect status has been established." (§4.3, Single Connection Mode) | no test | no test carries this requirement id |
| [`RFC8907-4.3-2`](#rfc8907-4.3-2) "the client and server MUST ignore the flag after the second packet on a connection" (§4.3, Single Connection Mode) | no test | no test carries this requirement id |
| [`RFC8907-4.3-3`](#rfc8907-4.3-3) "The client MUST accommodate such closures on a TCP session even after Single Connection Mode has been established." (§4.3, Single Connection Mode) | no test | no test carries this requirement id |
| [`RFC8907-4.4-1`](#rfc8907-4.4-1) On a REPLY of ERROR "The client cannot apply the result, and it MUST behave as if the server could not be connected to." (§4.4, Session Completion) | no test | no test carries this requirement id |
| [`RFC8907-4.4-2`](#rfc8907-4.4-2) After an ERROR caused by connection issues on a Single Connection Mode connection, "any further new sessions MUST NOT be accepted on the connection" (§4.4, Session Completion) | no test | no test carries this requirement id |
| [`RFC8907-4.4-3`](#rfc8907-4.4-3) "Once all active sessions are completed, then the connection MUST be closed." (§4.4, Session Completion) | no test | no test carries this requirement id |
| [`RFC8907-5.1-1`](#rfc8907-5.1-1) The username is optional in the START packet; "If it is absent, the client MUST set user_len to 0." (§5.1, The Authentication START Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-5.4-1`](#rfc8907-5.4-1) On a REPLY of GETDATA, GETUSER or GETPASS, "The client MUST then return a CONTINUE packet containing the requested information in the user_msg field." (§5.4, Description of Authentication Process) | no test | no test carries this requirement id |
| [`RFC8907-5.4-2`](#rfc8907-5.4-2) When the client queries the user for information the server marked NOECHO, "the response MUST NOT be reflected in the user interface as it is entered" (§5.4, Description of Authentication Process) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.2-1`](#rfc8907-5.4.2.2-1) A PAP login exchange "MUST consist of a single START packet and a single REPLY" (§5.4.2.2, PAP Login) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.2-2`](#rfc8907-5.4.2.2-2) "The START packet MUST contain a username and the data field MUST contain the PAP ASCII password." (§5.4.2.2, PAP Login) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1) A CHAP login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response." (§5.4.2.3, CHAP Login) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1) An MS-CHAP v1 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.4, MS-CHAP v1 Login) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1) An MS-CHAP v2 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.5, MS-CHAP v2 Login) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1) "the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE" (§5.4.2.6, Enable Requests) | no test | no test carries this requirement id |
| [`RFC8907-5.4.2.6-2`](#rfc8907-5.4.2.6-2) authen_service "MUST NOT be set to this value when requesting any other operation" (§5.4.2.6, Enable Requests) | no test | no test carries this requirement id |
| [`RFC8907-5.4.3-1`](#rfc8907-5.4.3-1) "If a client does not implement the TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL." (§5.4.3, Aborting an Authentication Session) | no test | no test carries this requirement id |
| [`RFC8907-6.1-1`](#rfc8907-6.1-1) "The user_len MUST indicate the length of the user field, in bytes." (§6.1, The Authorization REQUEST Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-6.1-2`](#rfc8907-6.1-2) "An argument name MUST NOT contain either of the separators." (§6.1, The Authorization REQUEST Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-6.2-1`](#rfc8907-6.2-1) On TAC_PLUS_AUTHOR_STATUS_PASS_ADD "the arguments in the response MUST be applied according to the rules described above" (§6.2, The Authorization REPLY Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-6.2-2`](#rfc8907-6.2-2) On TAC_PLUS_AUTHOR_STATUS_PASS_REPL "the client MUST use the authorization argument-value pairs (if any) in the response instead of the authorization argument-value pairs from the request" (§6.2, The Authorization REPLY Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-6.2-3`](#rfc8907-6.2-3) On TAC_PLUS_AUTHOR_STATUS_FAIL "the requested authorization MUST be denied" (§6.2, The Authorization REPLY Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-7.2-1`](#rfc8907-7.2-1) "The STOP flag MUST NOT be set in conjunction with the WATCHDOG flag." (§7.2, The Accounting REPLY Packet Body) | no test | no test carries this requirement id |
| [`RFC8907-8-1`](#rfc8907-8-1) "Clients MUST use these arguments when supporting the corresponding use cases." (§8, Argument-Value Pairs) | no test | no test carries this requirement id |
| [`RFC8907-8.1-1`](#rfc8907-8.1-1) "TACACS+ implementations MUST verify that they can accommodate the lengths of numeric arguments before attempting to process them", and a length that cannot be accommodated means "the argument MUST be regarded as not handled and the logic in 'Authorization' (Section 6.1) regarding the processing of arguments MUST be applied" (§8.1, Value Encoding) | no test | no test carries this requirement id |
| [`RFC8907-8.1-2`](#rfc8907-8.1-2) For an absolute date/time argument, "The time zone MUST be UTC unless a time zone argument is specified." (§8.1, Value Encoding) | no test | no test carries this requirement id |
| [`RFC8907-8.2-1`](#rfc8907-8.2-1) The "service" argument "MUST always be included" (§8.2, Authorization Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.2-2`](#rfc8907-8.2-2) "The 'cmd' argument MUST be specified if service equals 'shell'." (§8.2, Authorization Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.3-1`](#rfc8907-8.3-1) Accounting arguments "MUST precede any argument-value pairs that are defined in 'Authorization' (Section 6)" (§8.3, Accounting Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.3-2`](#rfc8907-8.3-2) "Start and stop records for the same event MUST have matching task_id argument values." (§8.3, Accounting Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.3-3`](#rfc8907-8.3-3) "The client MUST ensure that active task_ids are not duplicated; a client MUST NOT reuse a task_id in a start record until it has sent a stop record for that task_id." (§8.3, Accounting Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.3-4`](#rfc8907-8.3-4) "TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized." (§8.3, Accounting Arguments) | no test | no test carries this requirement id |
| [`RFC8907-8.3-5`](#rfc8907-8.3-5) "These 'Command Accounting' packets MUST include the 'service' and 'cmd' arguments, and if needed, the 'cmd-arg' arguments detailed in Section 8.2." (§8.3, Accounting Arguments) | no test | no test carries this requirement id |
| [`RFC8907-10.5-1`](#rfc8907-10.5-1) "New implementations, and upgrades of current implementations, MUST implement these recommendations." (§10.5, TACACS+ Best Practices) | no test | no test carries this requirement id |
| [`RFC8907-10.5.1-1`](#rfc8907-10.5.1-1) "TACACS+ servers and clients MUST treat shared secrets as sensitive data to be managed securely, as would be expected for other sensitive data such as identity credential information." (§10.5.1, Shared Secrets) | no test | no test carries this requirement id |
| [`RFC8907-10.5.1-2`](#rfc8907-10.5.1-2) "TACACS+ servers and clients MUST support shared keys that are at least 32 characters long." (§10.5.1, Shared Secrets) | no test | no test carries this requirement id |
| [`RFC8907-10.5.4-1`](#rfc8907-10.5.4-1) "administrators and implementers MUST ensure that the argument and value pairs shared between the clients and servers have consistent interpretation" (§10.5.4, Authorization) | no test | no test carries this requirement id |
| [`RFC8907-x-2`](#rfc8907-x-2) The TAC_PLUS_AUTHEN_STATUS_FOLLOW redirection mechanism is deprecated, and "This mechanism MUST NOT be used in modern deployments. It MUST NOT be used outside a secured deployment." (Error Handling) | no test | no test carries this requirement id |

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
| positive | [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L15) | unit/verify | unproven |

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

§10.5.2: "Clients MUST be implemented in a way that requires explicit configuration to enable the use of TAC_PLUS_UNENCRYPTED_FLAG." (§10, Security)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137) | unit/verify | unproven |
| positive | [`TestRFC8907UnencryptedModeIsNotReachableFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_unencrypted_config_test.go#L93) | unit/verify | revert, verified |

### [`RFC8907-10-2`](#rfc8907-10-2)

"TACACS+ clients MUST NOT set TAC_PLUS_UNENCRYPTED_FLAG." (§10, Security)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPacketMarshalNoEncryption`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/packet_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L142) | unit/verify | revert, verified |

### [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1)

A client that receives a reply whose obfuscation state disagrees with the shared-secret configuration of the server it came from MUST close the TCP session and process the reply as a FAIL (§10.5.2, Connections and Obfuscation)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L225) | unit/verify | unproven |

### [`RFC8907-6-1`](#rfc8907-6-1)

Authorization argument separator `=` (equals): client MUST be able to act on mandatory attributes or reject the authorization (§6, Authorization)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6-1, so no unit is bound to it.

### [`RFC8907-3.7-1`](#rfc8907-3.7-1)

"Usernames MUST be encoded and handled using the UsernameCasePreserved Profile specified in [RFC8265]." (§3.7, Treatment of Text Strings)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-3.7-1, so no unit is bound to it.

### [`RFC8907-3.7-2`](#rfc8907-3.7-2)

"All other text fields in TACACS+ MUST be treated as printable byte arrays of US-ASCII as defined by [RFC0020]", and printable "MUST exclude the 'Control Characters' defined in Section 5.2 of [RFC0020]" (§3.7, Treatment of Text Strings)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-3.7-2, so no unit is bound to it.

### [`RFC8907-4.1-1`](#rfc8907-4.1-1)

A variable-length field whose length value is zero is unused, and such fields "MUST be ignored, and treated as if not present" (§4.1, The TACACS+ Packet Header)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.1-1, so no unit is bound to it.

### [`RFC8907-4.1-2`](#rfc8907-4.1-2)

"The first packet in a session MUST have the sequence number 1" (§4.1, The TACACS+ Packet Header)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.1-2, so no unit is bound to it.

### [`RFC8907-4.1-3`](#rfc8907-4.1-3)

Header flag bits other than TAC_PLUS_UNENCRYPTED_FLAG and TAC_PLUS_SINGLE_CONNECT_FLAG "MUST be ignored when reading, and SHOULD be set to zero when writing" (§4.1, The TACACS+ Packet Header)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.1-3, so no unit is bound to it.

### [`RFC8907-4.3-1`](#rfc8907-4.3-1)

"The client MUST NOT send a second packet on a connection until single-connect status has been established." (§4.3, Single Connection Mode)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.3-1, so no unit is bound to it.

### [`RFC8907-4.3-2`](#rfc8907-4.3-2)

"the client and server MUST ignore the flag after the second packet on a connection" (§4.3, Single Connection Mode)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.3-2, so no unit is bound to it.

### [`RFC8907-4.3-3`](#rfc8907-4.3-3)

"The client MUST accommodate such closures on a TCP session even after Single Connection Mode has been established." (§4.3, Single Connection Mode)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.3-3, so no unit is bound to it.

### [`RFC8907-4.4-1`](#rfc8907-4.4-1)

On a REPLY of ERROR "The client cannot apply the result, and it MUST behave as if the server could not be connected to." (§4.4, Session Completion)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.4-1, so no unit is bound to it.

### [`RFC8907-4.4-2`](#rfc8907-4.4-2)

After an ERROR caused by connection issues on a Single Connection Mode connection, "any further new sessions MUST NOT be accepted on the connection" (§4.4, Session Completion)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.4-2, so no unit is bound to it.

### [`RFC8907-4.4-3`](#rfc8907-4.4-3)

"Once all active sessions are completed, then the connection MUST be closed." (§4.4, Session Completion)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.4-3, so no unit is bound to it.

### [`RFC8907-5.1-1`](#rfc8907-5.1-1)

The username is optional in the START packet; "If it is absent, the client MUST set user_len to 0." (§5.1, The Authentication START Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.1-1, so no unit is bound to it.

### [`RFC8907-5.4-1`](#rfc8907-5.4-1)

On a REPLY of GETDATA, GETUSER or GETPASS, "The client MUST then return a CONTINUE packet containing the requested information in the user_msg field." (§5.4, Description of Authentication Process)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4-1, so no unit is bound to it.

### [`RFC8907-5.4-2`](#rfc8907-5.4-2)

When the client queries the user for information the server marked NOECHO, "the response MUST NOT be reflected in the user interface as it is entered" (§5.4, Description of Authentication Process)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4-2, so no unit is bound to it.

### [`RFC8907-5.4.2.2-1`](#rfc8907-5.4.2.2-1)

A PAP login exchange "MUST consist of a single START packet and a single REPLY" (§5.4.2.2, PAP Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.2-1, so no unit is bound to it.

### [`RFC8907-5.4.2.2-2`](#rfc8907-5.4.2.2-2)

"The START packet MUST contain a username and the data field MUST contain the PAP ASCII password." (§5.4.2.2, PAP Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.2-2, so no unit is bound to it.

### [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1)

A CHAP login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response." (§5.4.2.3, CHAP Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.3-1, so no unit is bound to it.

### [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1)

An MS-CHAP v1 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.4, MS-CHAP v1 Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.4-1, so no unit is bound to it.

### [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1)

An MS-CHAP v2 login exchange "MUST consist of a single START packet and a single REPLY", and "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response." (§5.4.2.5, MS-CHAP v2 Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.5-1, so no unit is bound to it.

### [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1)

"the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE" (§5.4.2.6, Enable Requests)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.6-1, so no unit is bound to it.

### [`RFC8907-5.4.2.6-2`](#rfc8907-5.4.2.6-2)

authen_service "MUST NOT be set to this value when requesting any other operation" (§5.4.2.6, Enable Requests)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.6-2, so no unit is bound to it.

### [`RFC8907-5.4.3-1`](#rfc8907-5.4.3-1)

"If a client does not implement the TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL." (§5.4.3, Aborting an Authentication Session)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.3-1, so no unit is bound to it.

### [`RFC8907-6.1-1`](#rfc8907-6.1-1)

"The user_len MUST indicate the length of the user field, in bytes." (§6.1, The Authorization REQUEST Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6.1-1, so no unit is bound to it.

### [`RFC8907-6.1-2`](#rfc8907-6.1-2)

"An argument name MUST NOT contain either of the separators." (§6.1, The Authorization REQUEST Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6.1-2, so no unit is bound to it.

### [`RFC8907-6.2-1`](#rfc8907-6.2-1)

On TAC_PLUS_AUTHOR_STATUS_PASS_ADD "the arguments in the response MUST be applied according to the rules described above" (§6.2, The Authorization REPLY Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6.2-1, so no unit is bound to it.

### [`RFC8907-6.2-2`](#rfc8907-6.2-2)

On TAC_PLUS_AUTHOR_STATUS_PASS_REPL "the client MUST use the authorization argument-value pairs (if any) in the response instead of the authorization argument-value pairs from the request" (§6.2, The Authorization REPLY Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6.2-2, so no unit is bound to it.

### [`RFC8907-6.2-3`](#rfc8907-6.2-3)

On TAC_PLUS_AUTHOR_STATUS_FAIL "the requested authorization MUST be denied" (§6.2, The Authorization REPLY Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-6.2-3, so no unit is bound to it.

### [`RFC8907-7.2-1`](#rfc8907-7.2-1)

"The STOP flag MUST NOT be set in conjunction with the WATCHDOG flag." (§7.2, The Accounting REPLY Packet Body)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-7.2-1, so no unit is bound to it.

### [`RFC8907-8-1`](#rfc8907-8-1)

"Clients MUST use these arguments when supporting the corresponding use cases." (§8, Argument-Value Pairs)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8-1, so no unit is bound to it.

### [`RFC8907-8.1-1`](#rfc8907-8.1-1)

"TACACS+ implementations MUST verify that they can accommodate the lengths of numeric arguments before attempting to process them", and a length that cannot be accommodated means "the argument MUST be regarded as not handled and the logic in 'Authorization' (Section 6.1) regarding the processing of arguments MUST be applied" (§8.1, Value Encoding)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.1-1, so no unit is bound to it.

### [`RFC8907-8.1-2`](#rfc8907-8.1-2)

For an absolute date/time argument, "The time zone MUST be UTC unless a time zone argument is specified." (§8.1, Value Encoding)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.1-2, so no unit is bound to it.

### [`RFC8907-8.2-1`](#rfc8907-8.2-1)

The "service" argument "MUST always be included" (§8.2, Authorization Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.2-1, so no unit is bound to it.

### [`RFC8907-8.2-2`](#rfc8907-8.2-2)

"The 'cmd' argument MUST be specified if service equals 'shell'." (§8.2, Authorization Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.2-2, so no unit is bound to it.

### [`RFC8907-8.3-1`](#rfc8907-8.3-1)

Accounting arguments "MUST precede any argument-value pairs that are defined in 'Authorization' (Section 6)" (§8.3, Accounting Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.3-1, so no unit is bound to it.

### [`RFC8907-8.3-2`](#rfc8907-8.3-2)

"Start and stop records for the same event MUST have matching task_id argument values." (§8.3, Accounting Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.3-2, so no unit is bound to it.

### [`RFC8907-8.3-3`](#rfc8907-8.3-3)

"The client MUST ensure that active task_ids are not duplicated; a client MUST NOT reuse a task_id in a start record until it has sent a stop record for that task_id." (§8.3, Accounting Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.3-3, so no unit is bound to it.

### [`RFC8907-8.3-4`](#rfc8907-8.3-4)

"TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized." (§8.3, Accounting Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.3-4, so no unit is bound to it.

### [`RFC8907-8.3-5`](#rfc8907-8.3-5)

"These 'Command Accounting' packets MUST include the 'service' and 'cmd' arguments, and if needed, the 'cmd-arg' arguments detailed in Section 8.2." (§8.3, Accounting Arguments)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-8.3-5, so no unit is bound to it.

### [`RFC8907-10.5-1`](#rfc8907-10.5-1)

"New implementations, and upgrades of current implementations, MUST implement these recommendations." (§10.5, TACACS+ Best Practices)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-10.5-1, so no unit is bound to it.

### [`RFC8907-10.5.1-1`](#rfc8907-10.5.1-1)

"TACACS+ servers and clients MUST treat shared secrets as sensitive data to be managed securely, as would be expected for other sensitive data such as identity credential information." (§10.5.1, Shared Secrets)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-10.5.1-1, so no unit is bound to it.

### [`RFC8907-10.5.1-2`](#rfc8907-10.5.1-2)

"TACACS+ servers and clients MUST support shared keys that are at least 32 characters long." (§10.5.1, Shared Secrets)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-10.5.1-2, so no unit is bound to it.

### [`RFC8907-10.5.4-1`](#rfc8907-10.5.4-1)

"administrators and implementers MUST ensure that the argument and value pairs shared between the clients and servers have consistent interpretation" (§10.5.4, Authorization)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-10.5.4-1, so no unit is bound to it.

### [`RFC8907-x-2`](#rfc8907-x-2)

The TAC_PLUS_AUTHEN_STATUS_FOLLOW redirection mechanism is deprecated, and "This mechanism MUST NOT be used in modern deployments. It MUST NOT be used outside a secured deployment." (Error Handling)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-x-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8907.txt |
| Source fingerprint | 8bc695c4b9949009 |
| Record | rfc/extraction/rfc8907.json |
| Mapped sentences | 49 |
| Declined as scope | 55 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 1 | walked | not stated |
| `3.7` | not stated | 3 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 6 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 4 | walked | not stated |
| `4.4` | not stated | 3 | walked | not stated |
| `4.5` | not stated | 7 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.4.1` | not stated | 0 | walked | not stated |
| `5.4.2` | not stated | 1 | walked | not stated |
| `5.4.2.1` | not stated | 3 | walked | not stated |
| `5.4.2.2` | not stated | 3 | walked | not stated |
| `5.4.2.3` | not stated | 3 | walked | not stated |
| `5.4.2.4` | not stated | 4 | walked | not stated |
| `5.4.2.5` | not stated | 4 | walked | not stated |
| `5.4.2.6` | not stated | 2 | walked | not stated |
| `5.4.2.7` | not stated | 2 | walked | not stated |
| `5.4.3` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 4 | walked | not stated |
| `6.2` | not stated | 4 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 5 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `8.1` | not stated | 3 | walked | not stated |
| `8.2` | not stated | 2 | walked | not stated |
| `8.3` | not stated | 6 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 1 | walked | not stated |
| `10.2` | not stated | 2 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `10.4` | not stated | 0 | walked | not stated |
| `10.5` | not stated | 3 | walked | not stated |
| `10.5.1` | not stated | 7 | walked | not stated |
| `10.5.2` | not stated | 10 | walked | not stated |
| `10.5.3` | not stated | 2 | walked | not stated |
| `10.5.4` | not stated | 2 | walked | not stated |
| `10.5.5` | not stated | 2 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If an error occurs but the type of the incoming packet cannot be determined, a packet with the identical cleartext header but with a sequence number incremented by one and the length set to zero MUST be returned to indicate an error. |
| `3.7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Defines the word 'printable' used by the sentence the mapped id already carries; the same obligation on the same fields. | The term "printable" used here means the fields MUST exclude the "Control Characters" defined in Section 5.2 of [RFC0020]. |
| `4.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the ban on TAC_PLUS_UNENCRYPTED_FLAG that the client-side row already carries. | This option MUST NOT be used in production. |
| `4.1:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | Implementations MUST allow control over maximum packet sizes accepted by TACACS+ Servers. |
| `4.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | For example, a server MUST be configured to time out a Single Connection Mode TCP connection after a specific period of inactivity to preserve its resources. |
| `4.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The secrecy of the shared key, stated once here and once as an obligation on servers and clients in Section 10.5.1, which the mapped row carries. | The secret keys MUST remain secret. |
| `4.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | Server implementations MUST allow a unique secret key to be associated with each client. |
| `4.5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same obligation seen from the sender: the flag is set to 0 so the body is obfuscated, which is what the client row already requires. | The flag field MUST be configured with TAC_PLUS_UNENCRYPTED_FLAG set to 0 so that the packet body is obfuscated by XORing it bytewise with a pseudo-random pad: |
| `4.5:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | When a server detects that the secrets it has configured for the device do not match, it MUST return ERROR. |
| `4.5:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the deprecation of the unencrypted option that the client row already carries. | This option is deprecated and MUST NOT be used in production. |
| `4.5:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | A request MUST be dropped if TAC_PLUS_UNENCRYPTED_FLAG is set to true. |
| `5.4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the server does not implement an option, it MUST respond with TAC_PLUS_AUTHEN_STATUS_FAIL. |
| `5.4.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the user does not include the username, then the server MUST obtain it from the client with a CONTINUE TAC_PLUS_AUTHEN_STATUS_GETUSER. |
| `5.4.2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the user does not provide a username, then the server can send another TAC_PLUS_AUTHEN_STATUS_GETUSER request, but the server MUST limit the number of retries that are permitted; the recommended limit is three attempts. |
| `5.4.2.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The data fields in both the START and CONTINUE packets are not used for ASCII logins; any content MUST be ignored. |
| `5.4.2.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The REPLY from the server MUST be either a PASS, FAIL, or ERROR. |
| `5.4.2.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the same CHAP START construction, which the mapped row states in full. | The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response. |
| `5.4.2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The REPLY from the server MUST be a PASS, FAIL, or ERROR. |
| `5.4.2.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the same MS-CHAP v1 START construction, which the mapped row states in full. | The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. |
| `5.4.2.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The REPLY from the server MUST be a PASS or FAIL. |
| `5.4.2.4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The TACACS+ server MUST reject authentications where the challenge deviates from 8 bytes as defined in the RFC. |
| `5.4.2.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the same MS-CHAP v2 START construction, which the mapped row states in full. | The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. |
| `5.4.2.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The REPLY from the server MUST be a PASS or FAIL. |
| `5.4.2.5:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The TACACS+ server MUST reject authentications where the challenge deviates from 16 bytes as defined in the RFC. |
| `5.4.2.7:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The status value TAC_PLUS_AUTHEN_STATUS_GETPASS MUST only be used when requesting the "new" password. |
| `5.4.2.7:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | When requesting the "old" password, the status value MUST be set to TAC_PLUS_AUTHEN_STATUS_GETDATA. |
| `6.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | As this information is not always subject to verification, it MUST NOT be used in policy evaluation. |
| `6.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | When the status equals TAC_PLUS_AUTHOR_STATUS_FOLLOW, the arg_cnt MUST be 0. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The server MUST reply with success only when the accounting request has been recorded. |
| `7.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the server did not record the accounting request, then it MUST reply with ERROR. |
| `7.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the START flag is not set, then this indicates only that task is still running, and no new information is provided (servers MUST ignore any arguments). |
| `7.2:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The server MUST respond with TAC_PLUS_ACCT_STATUS_ERROR if the client requests an INVALID option. |
| `8.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The consequence clause of the same numeric-argument obligation, which the mapped row states in full. | If the length cannot be accommodated, then the argument MUST be regarded as not handled and the logic in "Authorization" (Section 6.1) regarding the processing of arguments MUST be applied. |
| `8.3:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | Servers MUST NOT make assumptions about the format of a task_id. |
| `10.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the operator of the deployment, not to an implementation: the obligation is discharged by how the network is built and who may reach it. The producer would be the deploying network administrator, for whom Ze holds no code path, so no Ze function can satisfy or violate it. | For these reasons, users deploying the TACACS+ protocol in their environments MUST limit access to known clients and MUST control the security of the entire transmission path. |
| `10.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the ban on the TAC_PLUS_AUTHEN_STATUS_FOLLOW redirection that the mapped row carries. | It MUST NOT be used outside a secured deployment. |
| `10.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the operator of the deployment, not to an implementation: the obligation is discharged by how the network is built and who may reach it. The producer would be the deploying network administrator, for whom Ze holds no code path, so no Ze function can satisfy or violate it. | With respect to the observations about the security issues described above, a network administrator MUST NOT rely on the obfuscation of the TACACS+ protocol. |
| `10.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the operator of the deployment, not to an implementation: the obligation is discharged by how the network is built and who may reach it. The producer would be the deploying network administrator, for whom Ze holds no code path, so no Ze function can satisfy or violate it. | TACACS+ MUST be used within a secure deployment; TACACS+ MUST be deployed over networks that ensure privacy and integrity of the communication and MUST be deployed over a network that is separated from other traffic. |
| `10.5.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | TACACS+ servers MUST NOT leak sensitive data. |
| `10.5.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | * TACACS+ servers MUST NOT expose shared secrets in logs. |
| `10.5.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | * TACACS+ servers MUST allow a dedicated secret key to be defined for each client. |
| `10.5.1:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | * TACACS+ server management systems MUST provide a mechanism to track secret key lifetimes and notify administrators to update them periodically. |
| `10.5.1:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | * TACACS+ servers MUST support policy to define minimum complexity for shared keys. |
| `10.5.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | TACACS+ servers MUST allow the definition of individual clients. |
| `10.5.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | The servers MUST only accept network connection attempts from these defined known clients. |
| `10.5.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | TACACS+ servers MUST reject connections that have TAC_PLUS_UNENCRYPTED_FLAG set. |
| `10.5.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | There MUST always be a shared secret set on the server for the client requesting the connection. |
| `10.5.2:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If an invalid shared secret is detected when processing packets for a client, TACACS+ servers MUST NOT accept any new sessions on that connection. |
| `10.5.2:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | TACACS+ servers MUST terminate the connection on completion of any sessions that were previously established with a valid shared secret on that connection. |
| `10.5.2:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the production ban on TAC_PLUS_UNENCRYPTED_FLAG that the client row carries. | This option MUST NOT be used when the client is in production. |
| `10.5.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | To help TACACS+ administrators select stronger authentication options, TACACS+ servers MUST allow the administrator to configure the server to only accept challenge/response options for authentication (TAC_PLUS_AUTHEN_TYPE_CHAP or TAC_PLUS_AUTHEN_TYPE_MSCHAP or TAC_PLUS_AUTHEN_TYPE_MSCHAPV2 for authen_type). |
| `10.5.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If they must be implemented, the servers MUST default to the options being disabled and MUST warn the administrator that these options are not secure. |
| `10.5.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same client obligation as Section 6.1: an unrecognized mandatory argument is evaluated as TAC_PLUS_AUTHOR_STATUS_FAIL. | TACACS+ clients that receive an unrecognized mandatory argument MUST evaluate server response as if they received TAC_PLUS_AUTHOR_STATUS_FAIL. |
| `10.5.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | TACACS+ servers MUST deprecate the redirection mechanism. |
| `10.5.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Addressed to the TACACS+ server, the role that accepts connections, receives requests and returns replies. Ze implements the client (NAS) side only: internal/component/tacacs dials out, sends START/REQUEST packets and reads replies, and it never listens on port 49. The producer would be a TACACS+ server daemon, which Ze does not ship. | If the redirection mechanism is implemented, then TACACS+ servers MUST disable it by default and MUST warn TACACS+ server administrators that it must only be enabled within a secure deployment due to the risks of revealing shared secrets. |

## Superseded

No document obsoletes RFC 8907, so its obligations are stated where they were written.
