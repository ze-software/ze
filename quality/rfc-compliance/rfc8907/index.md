# RFC 8907 - The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol

Partial. Every requirement this repository extracted from RFC 8907, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.4% | 45 of 56 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 5.4% | 3 of 56 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 56 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 56 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 56 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 68.0% | 104 of 153 tagged units, 0 escaped and 2 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 56 | of 59 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 56 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.6% | 2 of 56 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 56 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 10.7% | 6 of 56 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 56 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 59 |
| Gated MUST-level | 56 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 153 |
| Tagged units | 153 |
| Recorded audit verdicts | 48 |
| Discrimination records | 106 |
| Summary | `rfc/short/rfc8907.md` |
| Requirement shard | `rfc/requirements/rfc8907.md` |
| RFC text | `rfc/full/rfc8907.txt` |

## Enrolment

Enrolled: Ze implements a TACACS+ client for management AAA. The 2026-09-21 extraction walk expanded the original requirement list; the compliance checklist below is the requirement inventory, and tagged tests name their producers. Verification and discrimination must be rerun after the current implementation changes before publishing a conformance count.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- PAP login followed by shell-session privilege authorization
- printable-ASCII fields and UsernameCasePreserved usernames
- exact decrypted lengths
- ordered ERROR failover
- mandatory command arguments
- command accounting and negotiated single-connect reuse.


**What the ledger says remains**

Current implementation changes await test and discrimination runs. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`: no ASCII interactive, CHAP, MS-CHAP v1/v2 or ENABLE authentication workflows. The conditional method rows retain that selection and their source text. Section 10.5 also includes deployment obligations that a TCP reachability check cannot establish.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 45 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 1 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **56** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (45):** [`RFC8907-4-1`](#rfc8907-4-1), [`RFC8907-4-2`](#rfc8907-4-2), [`RFC8907-4-4`](#rfc8907-4-4), [`RFC8907-4.6-1`](#rfc8907-4.6-1), [`RFC8907-7-1`](#rfc8907-7-1), [`RFC8907-7-2`](#rfc8907-7-2), [`RFC8907-10-2`](#rfc8907-10-2), [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1), [`RFC8907-6-1`](#rfc8907-6-1), [`RFC8907-3.7-1`](#rfc8907-3.7-1), [`RFC8907-3.7-2`](#rfc8907-3.7-2), [`RFC8907-4.1-1`](#rfc8907-4.1-1), [`RFC8907-4.1-2`](#rfc8907-4.1-2), [`RFC8907-4.1-3`](#rfc8907-4.1-3), [`RFC8907-4.3-1`](#rfc8907-4.3-1), [`RFC8907-4.3-2`](#rfc8907-4.3-2), [`RFC8907-4.3-3`](#rfc8907-4.3-3), [`RFC8907-4.4-1`](#rfc8907-4.4-1), [`RFC8907-4.4-2`](#rfc8907-4.4-2), [`RFC8907-4.4-3`](#rfc8907-4.4-3), [`RFC8907-5.1-1`](#rfc8907-5.1-1), [`RFC8907-5.4.2.2-1`](#rfc8907-5.4.2.2-1), [`RFC8907-5.4.2.2-2`](#rfc8907-5.4.2.2-2), [`RFC8907-5.4.2.6-2`](#rfc8907-5.4.2.6-2), [`RFC8907-5.4.3-1`](#rfc8907-5.4.3-1), [`RFC8907-6.1-1`](#rfc8907-6.1-1), [`RFC8907-6.1-2`](#rfc8907-6.1-2), [`RFC8907-6.2-1`](#rfc8907-6.2-1), [`RFC8907-6.2-2`](#rfc8907-6.2-2), [`RFC8907-6.2-3`](#rfc8907-6.2-3), [`RFC8907-7.2-1`](#rfc8907-7.2-1), [`RFC8907-8-1`](#rfc8907-8-1), [`RFC8907-8.1-1`](#rfc8907-8.1-1), [`RFC8907-8.1-2`](#rfc8907-8.1-2), [`RFC8907-8.2-1`](#rfc8907-8.2-1), [`RFC8907-8.2-2`](#rfc8907-8.2-2), [`RFC8907-8.3-1`](#rfc8907-8.3-1), [`RFC8907-8.3-2`](#rfc8907-8.3-2), [`RFC8907-8.3-3`](#rfc8907-8.3-3), [`RFC8907-8.3-4`](#rfc8907-8.3-4), [`RFC8907-8.3-5`](#rfc8907-8.3-5), [`RFC8907-10.5.1-1`](#rfc8907-10.5.1-1), [`RFC8907-10.5.1-2`](#rfc8907-10.5.1-2), [`RFC8907-10.5.4-1`](#rfc8907-10.5.4-1), [`RFC8907-x-2`](#rfc8907-x-2)

**Annotated (including scoped evidence) (11):** [`RFC8907-4-3`](#rfc8907-4-3), [`RFC8907-4-5`](#rfc8907-4-5), [`RFC8907-5-1`](#rfc8907-5-1), [`RFC8907-10-1`](#rfc8907-10-1), [`RFC8907-5.4-1`](#rfc8907-5.4-1), [`RFC8907-5.4-2`](#rfc8907-5.4-2), [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1), [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1), [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1), [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1), [`RFC8907-4.6-2`](#rfc8907-4.6-2)

**Derived from other rows (1):** [`RFC8907-10.5-1`](#rfc8907-10.5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8907-4-1` | This is the major TACACS+ version number. TAC_PLUS_MAJOR_VER := 0xc (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907EveryRequestCarriesMajorVersion12`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L63). **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L146). **negative:** `unit/verify` [`TestRFC8907ReplyWithOtherMajorVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L75). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L241) |
| `RFC8907-4-2` | TACACS+ clients only send packets containing odd sequence numbers, and TACACS+ servers only send packets containing even sequence numbers. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907ClientSendsOddSequenceNumbers`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L94). **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L147). **negative:** `unit/verify` [`TestRFC8907ReplyWithOddSequenceNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L106). **negative:** `unit/verify` [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L250) |
| `RFC8907-4-3` | This number MUST be generated by a cryptographically strong random number generation method. (§4, Session Lifecycle) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8907SessionIDComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_sessionid_test.go#L38). **positive:** `unit/verify` [`TestRandomSessionIDDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L327). **negative:** no negative test. **{single-polarity}:** the session id is drawn from crypto/rand (internal/component/tacacs/client.go:497-503) with no predictable/reject path |
| `RFC8907-4-4` | The Id for this TACACS+ session. This field does not change for the duration of the TACACS+ session. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907ReplyKeepingSessionIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L122). **positive:** `unit/verify` [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L148). **negative:** `unit/verify` [`TestRFC8907ReplyChangingSessionIDRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L134) |
| `RFC8907-4-5` | All length values are unsigned and in network byte order. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L15). **positive:** `unit/verify` [`TestRFC8907LengthsAreUnsignedNetworkOrder`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L150). **negative:** no negative test. **{single-polarity}:** multi-octet header fields (session_id, length) are written and read with binary.BigEndian at internal/component/tacacs/packet.go:75-76 and 90-91; the marshal/unmarshal round-trip is symmetric with no independent little-endian oracle, so a negative would only test a different codec |
| `RFC8907-4.6-1` | After a packet body is de-obfuscated, the lengths of the component values in the packet are summed. If the sum is not identical to the cleartext datalength value from the header, the packet MUST be discarded and an ERROR signaled. (§4.5, Data Obfuscation) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC8907ExactReplyLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L226). **negative:** `unit/verify` [`TestRFC8907OverlongReplyComponentsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L190). **negative:** `unit/verify` [`TestRFC8907TrailingReplyBytesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L239) |
| `RFC8907-5-1` | The sequence number must never wrap, i.e., if the sequence number 2^(8)-1 is ever reached, that session must terminate and be restarted with a sequence number of 1. (§4.1, Packet Header; maximum 255, not 254) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** on 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`; internal/component/tacacs/client.go::Authenticate calls NewPAPAuthenStart and trySend emits sequence 1 and accepts only reply sequence 2. Section 5.4.2.2 requires one START and one REPLY for PAP, so no reachable authentication exchange approaches 255 or wraps; the separate initial/reply sequence tests remain applicable |
| `RFC8907-7-1` | This holds bitmapped flags. Valid values are: TAC_PLUS_ACCT_FLAG_START := 0x02 TAC_PLUS_ACCT_FLAG_STOP := 0x04 TAC_PLUS_ACCT_FLAG_WATCHDOG := 0x08 (§7.1) | MUST NOT | 7.1 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_acct_test.go#L13). **positive:** `unit/verify` [`TestRFC8907AccountingValidFlagsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L201). **negative:** `unit/verify` [`TestRFC8907AccountingMoreFlagRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L277). **negative:** `unit/verify` [`TestRFC8907AccountingOtherFlagsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L210) |
| `RFC8907-7-2` | The START and STOP flags are mutually exclusive. (§7.2) | MUST | 7.2 | **positive:** `unit/verify` [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_acct_test.go#L14). **negative:** `unit/verify` [`TestRFC8907AccountingStartStopCombinationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L288) |
| `RFC8907-10-1` | Clients MUST be implemented in a way that requires explicit configuration to enable the use of TAC_PLUS_UNENCRYPTED_FLAG. (§10.5.2) | MUST | 10.5.2 | **positive:** `unit/verify` [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L128). **positive:** `unit/verify` [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137). **positive:** `unit/verify` [`TestRFC8907UnencryptedModeIsNotReachableFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_unencrypted_config_test.go#L93). **negative:** no negative test. **{single-polarity}:** ze exposes no configuration that enables TAC_PLUS_UNENCRYPTED_FLAG: the schema declares no leaf for it and makes the shared secret mandatory (internal/component/tacacs/yang/ze-tacacs-conf.yang), the only request flag any producer sets is FlagSingleConnect (internal/component/tacacs/client.go trySend), and (*Packet).MarshalInto (internal/component/tacacs/packet.go) refuses a marshal with no shared secret instead of falling back to an unencrypted send, so there is no enabled state to drive a negative against |
| `RFC8907-10-2` | TACACS+ clients MUST NOT set TAC_PLUS_UNENCRYPTED_FLAG. (§10, Security) | MUST NOT | 10 | **positive:** `unit/verify` [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L142). **negative:** `unit/verify` [`TestPacketMarshalNoEncryption`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L167). **negative:** `unit/verify` [`TestRFC8907BuildRejectsMissingSecret`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L111) |
| `RFC8907-10.5.2-1` | When a TACACS+ client receives responses from servers where: * the response packet was received from the server configured with a shared key, but the packet has TAC_PLUS_UNENCRYPTED_FLAG set, and * the response packet was received from the server configured not to use obfuscation, but the packet has TAC_PLUS_UNENCRYPTED_FLAG not set, the TACACS+ client MUST close the TCP session, and process the response in the same way that a TAC_PLUS_AUTHEN_STATUS_FAIL (authentication sessions) or TAC_PLUS_AUTHOR_STATUS_FAIL (authorization sessions) was received. (§10.5.2, Connections and Obfuscation) | MUST | 10.5.2 | **positive:** `unit/verify` [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L222). **positive:** `unit/verify` [`TestRFC8907MatchingObfuscationKeepsStatusAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L460). **negative:** `unit/verify` [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L198). **negative:** `unit/verify` [`TestRFC8907ObfuscationMismatchClosesAndFails`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L435) |
| `RFC8907-6-1` | Mandatory arguments require that the receiving side can handle the argument, that is, its implementation and configuration includes the details of how to act on it. If the client receives a mandatory argument that it cannot handle, it MUST consider the authorization to have failed. (§6.1) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8907MandatoryCommandPolicyAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L50). **positive:** `unit/verify` [`TestRFC8907OptionalCommandPolicyIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L79). **negative:** `unit/verify` [`TestRFC8907MandatoryCommandPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L61). **negative:** `unit/verify` [`TestRFC8907MandatorySessionPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L209) |
| `RFC8907-3.7-1` | Usernames MUST be encoded and handled using the UsernameCasePreserved Profile specified in [RFC8265]. (§3.7, Treatment of Text Strings) | MUST | 3.7 | **positive:** `unit/verify` [`TestRFC8907UsernameProfileOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L63). **negative:** `unit/verify` [`TestRFC8907ForbiddenUsernameNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L90) |
| `RFC8907-3.7-2` | All other text fields in TACACS+ MUST be treated as printable byte arrays of US-ASCII as defined by [RFC0020]. The term "printable" used here means the fields MUST exclude the "Control Characters" defined in Section 5.2 of [RFC0020]. (§3.7, Treatment of Text Strings) | MUST | 3.7 | **positive:** `unit/verify` [`TestRFC8907PrintableTextInEveryField`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L63). **positive:** `unit/verify` [`TestRFC8907PrintableTextRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L108). **positive:** `unit/verify` [`TestRFC8907TrustedDispatchIdentitiesReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L172). **negative:** `unit/verify` [`TestRFC8907ControlCharactersRefusedInEveryField`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L91). **negative:** `unit/verify` [`TestRFC8907NonPrintableTextRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L130) |
| `RFC8907-4.1-1` | To signal that any variable-length data fields are unused, the corresponding length values are set to zero. Such fields MUST be ignored, and treated as if not present. (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907PresentFieldsFollowZeroLengthOnes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L214). **positive:** `unit/verify` [`TestRFC8907ZeroLengthDataIsReadAsAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L59). **negative:** `unit/verify` [`TestRFC8907PresentZeroByteIsNotReadAsAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L67). **negative:** `unit/verify` [`TestRFC8907ZeroLengthFieldsAreNotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L230) |
| `RFC8907-4.1-2` | The first packet in a session MUST have the sequence number 1, and each subsequent packet will increment the sequence number by one. (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907FirstPacketCarriesSequenceOne`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L194). **negative:** `unit/verify` [`TestRFC8907ReplyReusingSequenceOneIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L210) |
| `RFC8907-4.1-3` | All other bits MUST be ignored when reading, and SHOULD be set to zero when writing. (§4.1, The TACACS+ Packet Header) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC8907UnknownHeaderFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L153). **negative:** `unit/verify` [`TestRFC8907UnknownFlagsDoNotMaskDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L165) |
| `RFC8907-4.3-1` | The client MUST NOT send a second packet on a connection until single-connect status has been established. (§4.3, Single Connection Mode) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestRFC8907SecondSessionReusesEstablishedConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L229). **negative:** `unit/verify` [`TestRFC8907NoSecondPacketWithoutSingleConnect`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L240) |
| `RFC8907-4.3-2` | No provision is made for changing Single Connection Mode after the first two packets; the client and server MUST ignore the flag after the second packet on a connection. (§4.3, Single Connection Mode) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC8907FlagClearedOnLaterReplyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L254). **negative:** `unit/verify` [`TestRFC8907ClientDoesNotResignalSingleConnect`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L266) |
| `RFC8907-4.3-3` | The client MUST accommodate such closures on a TCP session even after Single Connection Mode has been established. (§4.3, Single Connection Mode) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC8907ServerClosureOfPooledConnectionIsAccommodated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L281). **negative:** `unit/verify` [`TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L324). **negative:** `unit/verify` [`TestRFC8907ClosureOnFreshDialIsNotRetried`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L294) |
| `RFC8907-4.4-1` | The server responds with an ERROR to indicate that the processing of the request did not complete. The client cannot apply the result, and it MUST behave as if the server could not be connected to. (§4.4, Session Completion) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC8907ErrorUsesBackupServer`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L177). **negative:** `unit/verify` [`TestRFC8907FailDoesNotUseBackupServer`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L203) |
| `RFC8907-4.4-2` | If Single Connection Mode was enabled, but an ERROR occurred due to connection issues (such as an incorrect secret (see Section 4.5)), then any further new sessions MUST NOT be accepted on the connection. (§4.4, Session Completion) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC8907NoNewSessionOnBrokenPooledConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L313). **negative:** `unit/verify` [`TestRFC8907HealthyPooledConnectionKeepsAcceptingSessions`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L328) |
| `RFC8907-4.4-3` | Once all active sessions are completed, then the connection MUST be closed. (§4.4, Session Completion) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC8907BrokenPooledConnectionIsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L339). **negative:** `unit/verify` [`TestRFC8907HealthyPooledConnectionStaysOpen`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L351) |
| `RFC8907-5.1-1` | The username is optional in this packet, depending upon the class of authentication. If it is absent, the client MUST set user_len to 0. (§5.1, The Authentication START Packet Body) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC8907AuthenStartAbsentUserWritesZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L47). **negative:** `unit/verify` [`TestRFC8907AuthenStartPresentUserWritesItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L63) |
| `RFC8907-5.4-1` | When the REPLY status equals TAC_PLUS_AUTHEN_STATUS_GETDATA, TAC_PLUS_AUTHEN_STATUS_GETUSER, or TAC_PLUS_AUTHEN_STATUS_GETPASS, authentication continues and the server SHOULD provide server_msg content for the client to prompt the user for more information. The client MUST then return a CONTINUE packet containing the requested information in the user_msg field. (§5.4, Description of Authentication Process) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The entire exchange MUST consist of a single START packet and a single REPLY."; the PAP-specific rule is §5.4.2.2, which also restricts the reply to PASS, FAIL or ERROR. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ASCII interactive workflows. internal/component/tacacs/client.go::Authenticate produces PAP through NewPAPAuthenStart, and validateReplyStatus rejects GETDATA/GETUSER/GETPASS rather than entering an interactive exchange; this is a selected method boundary, not a server-only obligation |
| `RFC8907-5.4-2` | If the information being requested by the server from the client is sensitive, then the server should set the TAC_PLUS_REPLY_FLAG_NOECHO flag. When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered. (§5.4, Description of Authentication Process) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered."; this rule is conditional on interactive prompting. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ASCII interaction. internal/component/tacacs/client.go::Authenticate receives an already supplied username/password through aaa.AuthRequest and never prompts in response to a TACACS+ reply |
| `RFC8907-5.4.2.2-1` | The entire exchange MUST consist of a single START packet and a single REPLY. (§5.4.2.2, PAP Login) | MUST | 5.4.2.2 | **positive:** `unit/verify` [`TestRFC8907PAPExchangeIsOneStartOneReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L382). **negative:** `unit/verify` [`TestRFC8907PAPClientSendsNoContinue`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L394) |
| `RFC8907-5.4.2.2-2` | The START packet MUST contain a username and the data field MUST contain the PAP ASCII password. (§5.4.2.2, PAP Login) | MUST | 5.4.2.2 | **positive:** `unit/verify` [`TestRFC8907PAPStartCarriesUserAndPassword`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L75). **positive:** `unit/verify` [`TestRFC8907PAPStartOnWireCarriesUserAndPassword`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L158). **negative:** `unit/verify` [`TestRFC8907PAPStartRefusesPasswordItCannotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L89). **negative:** `unit/verify` [`TestRFC8907PAPStartWithoutUsernameNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L170) |
| `RFC8907-5.4.2.3-1` | The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response. (§5.4.2.3, CHAP Login) | MUST | 5.4.2.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response."; §5.4.2.3 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_CHAP. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding CHAP. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product CHAP challenge/response workflow exists |
| `RFC8907-5.4.2.4-1` | The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.4, MS-CHAP v1 Login) | MUST | 5.4.2.4 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response."; §5.4.2.4 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_MSCHAP. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding MS-CHAP v1. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product MS-CHAP v1 challenge/response workflow exists |
| `RFC8907-5.4.2.5-1` | The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.5, MS-CHAP v2 Login) | MUST | 5.4.2.5 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response."; §5.4.2.5 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_MSCHAPV2. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding MS-CHAP v2. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product MS-CHAP v2 challenge/response workflow exists |
| `RFC8907-5.4.2.6-1` | In order to readily distinguish "ENABLE" requests from other types of request, the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE. (§5.4.2.6, Enable Requests) | MUST | 5.4.2.6 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The exchange MAY consist of multiple messages while the server collects the information it requires in order to allow changing the principal's privilege level."; §5.4.2.6 defines an ENABLE privilege-change workflow, and the service rule applies when requesting an ENABLE. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ENABLE workflows. internal/component/tacacs/authen.go::NewPAPAuthenStart sets the LOGIN service for the management password callback; it does not request a privilege change. The separate prohibition on using ENABLE for other operations remains tested |
| `RFC8907-5.4.2.6-2` | It MUST NOT be set to this value when requesting any other operation. (§5.4.2.6, Enable Requests) | MUST NOT | 5.4.2.6 | **positive:** `unit/verify` [`TestRFC8907LoginStartCarriesLoginService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L97). **positive:** `unit/verify` [`TestRFC8907PAPLoginOnWireCarriesLoginService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L453). **negative:** `unit/verify` [`TestRFC8907LoginStartNeverCarriesEnableService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L105). **negative:** `unit/verify` [`TestRFC8907NoRequestCarriesEnableService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L458) |
| `RFC8907-5.4.3-1` | If a client does not implement the TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL. (§5.4.3, Aborting an Authentication Session) | MUST | 5.4.3 | **positive:** `unit/verify` [`TestRFC8907RestartIsProcessedAsFail`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L75). **negative:** `unit/verify` [`TestRFC8907ErrorIsNotProcessedAsFail`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L84) |
| `RFC8907-6.1-1` | The user_len MUST indicate the length of the user field, in bytes. (§6.1, The Authorization REQUEST Packet Body) | MUST | 6.1 | **positive:** `unit/verify` [`TestRFC8907AuthorRequestUserLenMatchesUser`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L114). **positive:** `unit/verify` [`TestRFC8907AuthorUserLenCountsBytes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L179). **negative:** `unit/verify` [`TestRFC8907AuthorRequestRefusesUserItCannotMeasure`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L125). **negative:** `unit/verify` [`TestRFC8907AuthorUserLenRefusesRuneCount`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L190) |
| `RFC8907-6.1-2` | An argument name MUST NOT contain either of the separators. (§6.1, The Authorization REQUEST Packet Body) | MUST NOT | 6.1 | **positive:** `unit/verify` [`TestRFC8907ArgumentNamesCarryNoSeparator`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L141). **negative:** `unit/verify` [`TestRFC8907ArgumentValueKeepsSeparatorOutOfName`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L151) |
| `RFC8907-6.2-1` | If the status equals TAC_PLUS_AUTHOR_STATUS_PASS_ADD, then the arguments specified in the request are authorized and the arguments in the response MUST be applied according to the rules described above. (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC8907PassAddRetainsCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L89). **positive:** `unit/verify` [`TestRFC8907SessionPrivilegeApplied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L169). **negative:** `unit/verify` [`TestRFC8907PassAddCannotIgnoreCommandExtension`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L97) |
| `RFC8907-6.2-2` | If the status equals TAC_PLUS_AUTHOR_STATUS_PASS_REPL, then the client MUST use the authorization argument-value pairs (if any) in the response instead of the authorization argument- value pairs from the request. (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC8907PassReplPreservesExactCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L108). **positive:** `unit/verify` [`TestRFC8907SessionReplacementMapsZero`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L200). **negative:** `unit/verify` [`TestRFC8907IncompleteSessionReplacementDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L223). **negative:** `unit/verify` [`TestRFC8907PassReplCannotAuthorizeOriginalCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L118) |
| `RFC8907-6.2-3` | If the status equals TAC_PLUS_AUTHOR_STATUS_FAIL, then the requested authorization MUST be denied. (§6.2, The Authorization REPLY Packet Body) | MUST | 6.2 | **positive:** `unit/verify` [`TestRFC8907AuthorizationFailDenies`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L113). **negative:** `unit/verify` [`TestRFC8907AuthorizationPassAddIsNotDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L121) |
| `RFC8907-7.2-1` | The STOP flag MUST NOT be set in conjunction with the WATCHDOG flag. (§7.2, The Accounting REPLY Packet Body) | MUST NOT | 7.2 | **positive:** `unit/verify` [`TestRFC8907StopRecordCarriesStopFlagOnly`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L63). **negative:** `unit/verify` [`TestRFC8907AccountingStopWatchdogCombinationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L299). **negative:** `unit/verify` [`TestRFC8907StopRecordNeverCarriesWatchdog`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L71) |
| `RFC8907-8-1` | Clients MUST use these arguments when supporting the corresponding use cases. (§8, Argument-Value Pairs) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8907AuthorizationRequestsUseDictionaryArguments`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L278). **positive:** `unit/verify` [`TestRFC8907CommandRecordsUseDictionaryArguments`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L87). **negative:** `unit/verify` [`TestRFC8907AuthorizationRequestsCarryNoForeignArgument`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L285). **negative:** `unit/verify` [`TestRFC8907CommandRecordsCarryNoForeignArgument`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L103) |
| `RFC8907-8.1-1` | TACACS+ implementations MUST verify that they can accommodate the lengths of numeric arguments before attempting to process them. If the length cannot be accommodated, then the argument MUST be regarded as not handled and the logic in "Authorization" (Section 6.1) regarding the processing of arguments MUST be applied. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC8907OverlongOptionalPrivilegeCannotElevate`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L191). **positive:** `unit/verify` [`TestRFC8907SessionPrivilegeApplied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L168). **negative:** `unit/verify` [`TestRFC8907InvalidMandatoryPrivilegeDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L178) |
| `RFC8907-8.1-2` | Absolute date/times are specified in seconds since the epoch, 12:00am, January 1, 1970. The time zone MUST be UTC unless a time zone argument is specified. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC8907StartTimeIsEpochSeconds`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L116). **positive:** `unit/verify` [`TestRFC8907StopTimeIsEpochSeconds`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L344). **negative:** `unit/verify` [`TestRFC8907StartTimeIgnoresLocalZone`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L132). **negative:** `unit/verify` [`TestRFC8907StopTimeIgnoresLocalZone`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L358) |
| `RFC8907-8.2-1` | Specifying a service argument indicates that this is a request for authorization or accounting of that service. For example: "shell", "tty-server", "connection", "system" and "firewall"; others may be chosen for the required application. This argument MUST always be included. (§8.2) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC8907EveryRequestKindLeadsWithService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L293). **positive:** `unit/verify` [`TestRFC8907ServiceArgumentAlwaysFirst`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L161). **negative:** `unit/verify` [`TestRFC8907NoRequestKindOmitsService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L306). **negative:** `unit/verify` [`TestRFC8907ServiceArgumentPresentForEmptyCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L167) |
| `RFC8907-8.2-2` | The "cmd" argument MUST be specified if service equals "shell". (§8.2, Authorization Arguments) | MUST | 8.2 | **positive:** `unit/verify` [`TestRFC8907CmdArgumentFollowsShellService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L173). **positive:** `unit/verify` [`TestRFC8907EveryShellRequestCarriesCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L321). **negative:** `unit/verify` [`TestRFC8907CmdArgumentPresentForEmptyCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L180). **negative:** `unit/verify` [`TestRFC8907ShellRequestNeverLacksCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L334) |
| `RFC8907-8.3-1` | The following arguments are defined for TACACS+ accounting only. They MUST precede any argument-value pairs that are defined in "Authorization" (Section 6). (§8.3) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC8907AccountingArgumentsPrecedeAuthorizationOnes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L155). **negative:** `unit/verify` [`TestRFC8907NoAuthorizationArgumentPrecedesAccountingOne`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L174) |
| `RFC8907-8.3-2` | Start and stop records for the same event MUST have matching task_id argument values. (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC8907StartAndStopShareTaskID`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L191). **negative:** `unit/verify` [`TestRFC8907SecondCommandCarriesOtherTaskID`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L207) |
| `RFC8907-8.3-3` | The client MUST ensure that active task_ids are not duplicated; a client MUST NOT reuse a task_id in a start record until it has sent a stop record for that task_id. (§8.3, Accounting Arguments) | MUST NOT | 8.3 | **positive:** `unit/verify` [`TestRFC8907ActiveTaskIDsAreDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L221). **negative:** `unit/verify` [`TestRFC8907TaskIDNeverReappears`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L236) |
| `RFC8907-8.3-4` | To support this mode of operation, TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized. (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** `unit/verify` [`TestAPIStreamSourceRunsStreamingHandler`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_api_test.go#L289). **positive:** `unit/verify` [`TestDispatcherAccountingWithoutUsername`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_command_test.go#L1222). **positive:** `unit/verify` [`TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_aaa_lifecycle_test.go#L231). **positive:** `unit/verify` [`TestRFC8907AccountingSaturationRetainsEveryStart`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accountant_test.go#L46). **positive:** `unit/verify` [`TestRFC8907PluginDispatchAccountsCommand`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L111). **positive:** `unit/verify` [`TestRFC8907TrustedDispatchIdentitiesReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L171). **positive:** `unit/verify` [`TestRFC8907TypedDispatchAccountsDeniedCommand`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L33). **negative:** `unit/verify` [`TestAPIStreamSourceAuthorizesReadOnly`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_api_test.go#L355). **negative:** `unit/verify` [`TestDispatcherAccountsRefusedCommands`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_command_test.go#L1286). **negative:** `unit/verify` [`TestRFC8907AccountingRequiresDestination`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L123). **negative:** `unit/verify` [`TestRFC8907AccountingStopDrainsAcceptedStarts`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accountant_test.go#L90). **negative:** `unit/verify` [`TestRFC8907BundleRetirementWaitsForAccountingPair`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_aaa_lifecycle_test.go#L325). **negative:** `unit/verify` [`TestRFC8907OversizedCommandAccountingStillReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L300) |
| `RFC8907-8.3-5` | These "Command Accounting" packets MUST include the "service" and "cmd" arguments, and if needed, the "cmd-arg" arguments detailed in Section 8.2. (§8.3, Accounting Arguments) | MUST | 8.3 | **positive:** `unit/verify` [`TestRFC8907CommandRecordsCarryServiceCmdAndArgs`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L254). **negative:** `unit/verify` [`TestRFC8907EmptyCommandRecordStillCarriesServiceAndCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L270) |
| `RFC8907-10.5-1` | New implementations, and upgrades of current implementations, MUST implement these recommendations. (§10.5, TACACS+ Best Practices) | MUST | 10.5 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8907-10.5.1-1, RFC8907-10.5.1-2, RFC8907-10.5.2-1, RFC8907-10.5.4-1; Section 10.5 makes "these recommendations" of Sections 10.5.1 to 10.5.4 mandatory for new implementations, and those four rows carry the client-side recommendations the checklist gates; the MUSTs of Sections 10.5.3 and 10.5.5 bind TACACS+ servers, a role ze does not fill. **derived:** met |
| `RFC8907-10.5.1-1` | TACACS+ servers and clients MUST treat shared secrets as sensitive data to be managed securely, as would be expected for other sensitive data such as identity credential information. (§10.5.1, Shared Secrets) | MUST | 10.5.1 | **positive:** `unit/verify` [`TestRFC8907SharedSecretConfigBuildAndDisplay`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L37). **negative:** `unit/verify` [`TestRFC8907AccountingRedactsConfigSecretWithoutChangingExecution`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L86). **negative:** `unit/verify` [`TestRFC8907SharedSecretCommandRedaction`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L149). **negative:** `unit/verify` [`TestRFC8907SharedSecretDiagnosticRedaction`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L87) |
| `RFC8907-10.5.1-2` | * TACACS+ servers and clients MUST support shared keys that are at least 32 characters long. (§10.5.1, Shared Secrets) | MUST | 10.5.1 | **positive:** `unit/verify` [`TestRFC8907LongSharedKeysAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L407). **positive:** `unit/verify` [`TestRFC8907SharedSecretConfigBuildAndDisplay`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L38). **negative:** `unit/verify` [`TestRFC8907SharedKeyIsNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L187) |
| `RFC8907-10.5.4-1` | The cost of the flexibility is that administrators and implementers MUST ensure that the argument and value pairs shared between the clients and servers have consistent interpretation. (§10.5.4, Authorization) | MUST | 10.5.4 | **positive:** `unit/verify` [`TestRFC8907MandatoryCommandPolicyAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L51). **positive:** `unit/verify` [`TestRFC8907SharedArgumentsKeepDefinedMeaning`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L379). **negative:** `unit/verify` [`TestRFC8907MandatoryCommandPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L62). **negative:** `unit/verify` [`TestRFC8907MandatorySessionPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L210). **negative:** `unit/verify` [`TestRFC8907SharedArgumentsNeverReinterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L393) |
| `RFC8907-4.6-2` | A request MUST be dropped if TAC_PLUS_UNENCRYPTED_FLAG is set to true. (§4.5, Data Obfuscation; stable historical ID) | MUST | 4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this request-receiver obligation binds a TACACS+ server. Ze implements the management AAA client, not a TACACS+ server; the client's separate obfuscation and reply-validation obligations remain applicable |
| `RFC8907-x-1` | The client should manage connections and handle the case of a server that establishes a connection but does not respond. The exact behavior is implementation specific. It is recommended that the client close the connection after a configurable timeout. (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8907-x-2` | This document deprecates the redirection mechanism using the TAC_PLUS_AUTHEN_STATUS_FOLLOW option, which was included in "The Draft". As part of this process, the secret key for a new server was sent to the client. This public exchange of secret keys means that once one session is broken, it may be possible to leverage that key to attacking connections to other servers. This mechanism MUST NOT be used in modern deployments. It MUST NOT be used outside a secured deployment. (§10.2) | MUST NOT | 10.2 | **positive:** `unit/verify` [`TestRFC8907AuthorizationFollowDeniesWithoutFallback`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L135). **positive:** `unit/verify` [`TestRFC8907FollowIsNotAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L127). **negative:** `unit/verify` [`TestRFC8907FollowTargetIsNeverContacted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L143) |
| `RFC8907-6-2` | Optional arguments are ones that may be disregarded by either client or server. (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8907-5-1`](#rfc8907-5-1) The sequence number must never wrap, i.e., if the sequence number 2^(8)-1 is ever reached, that session must terminate and be restarted with a sequence number of 1. (§4.1, Packet Header; maximum 255, not 254) | no test | no test carries this requirement id; annotated {not-applicable}: on 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`; internal/component/tacacs/client.go::Authenticate calls NewPAPAuthenStart and trySend emits sequence 1 and accepts only reply sequence 2. Section 5.4.2.2 requires one START and one REPLY for PAP, so no reachable authentication exchange approaches 255 or wraps; the separate initial/reply sequence tests remain applicable |
| [`RFC8907-5.4-1`](#rfc8907-5.4-1) When the REPLY status equals TAC_PLUS_AUTHEN_STATUS_GETDATA, TAC_PLUS_AUTHEN_STATUS_GETUSER, or TAC_PLUS_AUTHEN_STATUS_GETPASS, authentication continues and the server SHOULD provide server_msg content for the client to prompt the user for more information. The client MUST then return a CONTINUE packet containing the requested information in the user_msg field. (§5.4, Description of Authentication Process) | no test | no test carries this requirement id; annotated {feature-declined}: "The entire exchange MUST consist of a single START packet and a single REPLY."; the PAP-specific rule is §5.4.2.2, which also restricts the reply to PASS, FAIL or ERROR. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ASCII interactive workflows. internal/component/tacacs/client.go::Authenticate produces PAP through NewPAPAuthenStart, and validateReplyStatus rejects GETDATA/GETUSER/GETPASS rather than entering an interactive exchange; this is a selected method boundary, not a server-only obligation |
| [`RFC8907-5.4-2`](#rfc8907-5.4-2) If the information being requested by the server from the client is sensitive, then the server should set the TAC_PLUS_REPLY_FLAG_NOECHO flag. When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered. (§5.4, Description of Authentication Process) | no test | no test carries this requirement id; annotated {feature-declined}: "When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered."; this rule is conditional on interactive prompting. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ASCII interaction. internal/component/tacacs/client.go::Authenticate receives an already supplied username/password through aaa.AuthRequest and never prompts in response to a TACACS+ reply |
| [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1) The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response. (§5.4.2.3, CHAP Login) | no test | no test carries this requirement id; annotated {feature-declined}: "The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response."; §5.4.2.3 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_CHAP. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding CHAP. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product CHAP challenge/response workflow exists |
| [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1) The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.4, MS-CHAP v1 Login) | no test | no test carries this requirement id; annotated {feature-declined}: "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response."; §5.4.2.4 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_MSCHAP. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding MS-CHAP v1. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product MS-CHAP v1 challenge/response workflow exists |
| [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1) The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.5, MS-CHAP v2 Login) | no test | no test carries this requirement id; annotated {feature-declined}: "The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response."; §5.4.2.5 scopes this construction to authen_type TAC_PLUS_AUTHEN_TYPE_MSCHAPV2. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding MS-CHAP v2. internal/component/tacacs/client.go::Authenticate accepts username/password and uses NewPAPAuthenStart; no product MS-CHAP v2 challenge/response workflow exists |
| [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1) In order to readily distinguish "ENABLE" requests from other types of request, the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE. (§5.4.2.6, Enable Requests) | no test | no test carries this requirement id; annotated {feature-declined}: "The exchange MAY consist of multiple messages while the server collects the information it requires in order to allow changing the principal's privilege level."; §5.4.2.6 defines an ENABLE privilege-change workflow, and the service rule applies when requesting an ENABLE. On 2026-09-21 the owner selected `tacacs_methods='Keep PAP authentication'`, excluding ENABLE workflows. internal/component/tacacs/authen.go::NewPAPAuthenStart sets the LOGIN service for the management password callback; it does not request a privilege change. The separate prohibition on using ENABLE for other operations remains tested |
| [`RFC8907-10.5-1`](#rfc8907-10.5-1) New implementations, and upgrades of current implementations, MUST implement these recommendations. (§10.5, TACACS+ Best Practices) | no test | no test carries this requirement id; annotated {rollup}: RFC8907-10.5.1-1, RFC8907-10.5.1-2, RFC8907-10.5.2-1, RFC8907-10.5.4-1; Section 10.5 makes "these recommendations" of Sections 10.5.1 to 10.5.4 mandatory for new implementations, and those four rows carry the client-side recommendations the checklist gates; the MUSTs of Sections 10.5.3 and 10.5.5 bind TACACS+ servers, a role ze does not fill |
| [`RFC8907-4.6-2`](#rfc8907-4.6-2) A request MUST be dropped if TAC_PLUS_UNENCRYPTED_FLAG is set to true. (§4.5, Data Obfuscation; stable historical ID) | no test | no test carries this requirement id; annotated {not-applicable}: this request-receiver obligation binds a TACACS+ server. Ze implements the management AAA client, not a TACACS+ server; the client's separate obfuscation and reply-validation obligations remain applicable |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8907-4-1`](#rfc8907-4-1)

This is the major TACACS+ version number. TAC_PLUS_MAJOR_VER := 0xc (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907EveryRequestCarriesMajorVersion12 reads the captured request header (not an echo) and requires major nibble 0xc for authentication, authorization and accounting, so a client emitting 0xd goes red. Negative TestRFC8907ReplyWithOtherMajorVersionRefused mutates only the reply major nibble (header check runs before the body is read) for all 15 other values x 3 kinds and requires error + zero status; producer validateResponseHeader.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L241) | unit/verify | revert, verified |
| negative | [`TestRFC8907ReplyWithOtherMajorVersionRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestRFC8907EveryRequestCarriesMajorVersion12`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L63) | unit/verify | revert, verified |

### [`RFC8907-4-2`](#rfc8907-4-2)

TACACS+ clients only send packets containing odd sequence numbers, and TACACS+ servers only send packets containing even sequence numbers. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907ClientSendsOddSequenceNumbers reads the captured request seq_no and requires it odd for all 3 kinds, closing the old echo-server hole (a client sending 2 now goes red). Negative TestRFC8907ReplyWithOddSequenceNumberRefused sends each of the 128 odd server seq_no values x 3 kinds and requires refusal; only the reply seq_no differs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTacacsClientRejectsBadResponseHeader`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L250) | unit/verify | revert, verified |
| negative | [`TestRFC8907ReplyWithOddSequenceNumberRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestRFC8907ClientSendsOddSequenceNumbers`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L94) | unit/verify | revert, verified |

### [`RFC8907-4-3`](#rfc8907-4-3)

This number MUST be generated by a cryptographically strong random number generation method. (§4, Session Lifecycle)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC8907SessionIDComesFromCryptoRand substitutes crypto/rand.Reader and requires the id to be exactly those octets; TestRandomSessionIDDistinct adds cross-session distinctness

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRandomSessionIDDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L327) | unit/verify | unproven |
| positive | [`TestRFC8907SessionIDComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_sessionid_test.go#L38) | unit/verify | unproven |

### [`RFC8907-4-4`](#rfc8907-4-4)

The Id for this TACACS+ session. This field does not change for the duration of the TACACS+ session. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 after the invalid {single-polarity} marker was removed (correction recorded). Negative TestRFC8907ReplyChangingSessionIDRefused flips bit 0, bit 31 and all bits of the reply session_id (only field changed; type/version/seq/flags intact so the trySend session_id guard is the refusing check) for 3 kinds and requires error + zero status. Positive TestRFC8907ReplyKeepingSessionIDAccepted is the accepting path for all 3 kinds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ReplyChangingSessionIDRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestTacacsClientAuthenticatePass`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_client_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRFC8907ReplyKeepingSessionIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L122) | unit/verify | revert, verified |

### [`RFC8907-4-5`](#rfc8907-4-5)

All length values are unsigned and in network byte order. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. TestRFC8907LengthsAreUnsignedNetworkOrder asserts fixed wire octets for session_id and length (an independent big-endian oracle: a little-endian codec goes red), reads length 0xFFFFFFFE as unsigned, and reads 2-octet reply lengths 0x0100 and 0x8001 big-endian and unsigned in authentication, authorization and accounting replies (request lengths are 1 octet). {single-polarity: positive} holds: byte order has no refusal path. The marker's prose still describes the old symmetric round trip and should be refreshed; the argument itself stands.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8907LengthsAreUnsignedNetworkOrder`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestPacketHeaderMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L15) | unit/verify | revert, verified |

### [`RFC8907-4.6-1`](#rfc8907-4.6-1)

After a packet body is de-obfuscated, the lengths of the component values in the packet are summed. If the sum is not identical to the cleartext datalength value from the header, the packet MUST be discarded and an ERROR signaled. (§4.5, Data Obfuscation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. The missing direction is now proven: TestRFC8907OverlongReplyComponentsRejected sends replies whose server_msg_len, data_len or argument length sums past the datalength, for all three kinds, and requires error + zero status (discard and ERROR). The < direction (TestRFC8907TrailingReplyBytesRejected) and exact positive (TestRFC8907ExactReplyLengthsAccepted) stand; producer validateReplyBody.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907OverlongReplyComponentsRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestRFC8907TrailingReplyBytesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L239) | unit/verify | revert, verified |
| positive | [`TestRFC8907ExactReplyLengthsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L226) | unit/verify | revert, verified |

### [`RFC8907-5-1`](#rfc8907-5-1)

The sequence number must never wrap, i.e., if the sequence number 2^(8)-1 is ever reached, that session must terminate and be restarted with a sequence number of 1. (§4.1, Packet Header; maximum 255, not 254)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5-1, so no unit is bound to it.

### [`RFC8907-7-1`](#rfc8907-7-1)

This holds bitmapped flags. Valid values are: TAC_PLUS_ACCT_FLAG_START := 0x02 TAC_PLUS_ACCT_FLAG_STOP := 0x04 TAC_PLUS_ACCT_FLAG_WATCHDOG := 0x08 (§7.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907AccountingValidFlagsWritten writes 0x02, 0x04, 0x08 and 0x0a unchanged. Negative TestRFC8907AccountingOtherFlagsRefused refuses all 252 other values (every undefined bit 0x01, 0x10-0x80 and invalid combinations) and SendAccounting refuses 0x82 before contacting the server. Producer acct.go MarshalBinaryInto allowlist switch; a denylist now goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AccountingOtherFlagsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L210) | unit/verify | revert, verified |
| negative | [`TestRFC8907AccountingMoreFlagRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L277) | unit/verify | revert, verified |
| positive | [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_acct_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC8907AccountingValidFlagsWritten`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L201) | unit/verify | revert, verified |

### [`RFC8907-7-2`](#rfc8907-7-2)

The START and STOP flags are mutually exclusive. (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a request carrying START and STOP together. (b) TestRFC8907AccountingStartStopCombinationRefused requires errors.Is(err, errRequestInvalid) for Flags START|STOP; TestAcctRequestMarshalStartStop marshals START and STOP as separate flags.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AccountingStartStopCombinationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L288) | unit/verify | unproven |
| positive | [`TestAcctRequestMarshalStartStop`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_acct_test.go#L14) | unit/verify | unproven |

### [`RFC8907-10-1`](#rfc8907-10-1)

Clients MUST be implemented in a way that requires explicit configuration to enable the use of TAC_PLUS_UNENCRYPTED_FLAG. (§10.5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity} on the row: no configuration enables the flag. (a) forbidden: any configuration state, including a keyless server, under which the client sends with TAC_PLUS_UNENCRYPTED_FLAG. (b) TestRFC8907UnencryptedModeIsNotReachableFromConfiguration reads the socket: a keyed config puts wire[3]&0x01 == 0 (assert.Zero), a keyless config writes no bytes (assert.Empty); TestRFC8907ClientNeverSendsUnobfuscatedBody requires the wire body to be the pseudo-pad XOR of the plaintext.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L137) | unit/verify | unproven |
| positive | [`TestPacketMarshalRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC8907UnencryptedModeIsNotReachableFromConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_unencrypted_config_test.go#L93) | unit/verify | revert, verified |

### [`RFC8907-10-2`](#rfc8907-10-2)

TACACS+ clients MUST NOT set TAC_PLUS_UNENCRYPTED_FLAG. (§10, Security)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a client request whose flags octet has 0x01 set, or a send with no key. (b) TestRFC8907ClientNeverSendsUnobfuscatedBody fails when wire[3]&FlagUnencrypted != 0 and requires the body to decode through the pad; TestPacketMarshalNoEncryption requires ErrNoSharedSecret for nil and empty keys with zero bytes written; TestRFC8907BuildRejectsMissingSecret requires Build to refuse a keyless server with ErrNoSharedSecret and no authenticator.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPacketMarshalNoEncryption`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_packet_test.go#L167) | unit/verify | unproven |
| negative | [`TestRFC8907BuildRejectsMissingSecret`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L111) | unit/verify | unproven |
| positive | [`TestRFC8907ClientNeverSendsUnobfuscatedBody`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L142) | unit/verify | revert, verified |

### [`RFC8907-10.5.2-1`](#rfc8907-10.5.2-1)

When a TACACS+ client receives responses from servers where: * the response packet was received from the server configured with a shared key, but the packet has TAC_PLUS_UNENCRYPTED_FLAG set, and * the response packet was received from the server configured not to use obfuscation, but the packet has TAC_PLUS_UNENCRYPTED_FLAG not set, the TACACS+ client MUST close the TCP session, and process the response in the same way that a TAC_PLUS_AUTHEN_STATUS_FAIL (authentication sessions) or TAC_PLUS_AUTHOR_STATUS_FAIL (authorization sessions) was received. (§10.5.2, Connections and Obfuscation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Clause (a), keyed server + TAC_PLUS_UNENCRYPTED_FLAG: TestRFC8907ObfuscationMismatchClosesAndFails asserts AUTHEN FAIL and AUTHOR FAIL (the two session kinds the RFC names) AND that the client closed the TCP (probe reads EOF/ECONNRESET after writing a single-connect reply); producer trySend closeAndEvict + sendToServers errObfuscationMismatch->FAIL. Positive TestRFC8907MatchingObfuscationKeepsStatusAndConnection keeps PASS/PASS_ADD and the TCP open, so a client closing on every reply goes red. Clause (b), keyless server + flag clear, cannot arise: Packet.MarshalInto refuses an empty key, so the keyless probe sees no request (asserted); the trySend guard still covers both directions. Old units in rfc8907_obfuscation_test.go remain valid.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ObfuscationMismatchClosesAndFails`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L435) | unit/verify | revert, verified |
| negative | [`TestRFC8907ClientRefusesUnobfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestRFC8907MatchingObfuscationKeepsStatusAndConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L460) | unit/verify | revert, verified |
| positive | [`TestRFC8907ClientAcceptsObfuscatedReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_obfuscation_test.go#L222) | unit/verify | revert, verified |

### [`RFC8907-6-1`](#rfc8907-6-1)

Mandatory arguments require that the receiving side can handle the argument, that is, its implementation and configuration includes the details of how to act on it. If the client receives a mandatory argument that it cannot handle, it MUST consider the authorization to have failed. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: authorizing when a mandatory argument cannot be handled. (b) TestRFC8907MandatoryCommandPolicyDenied fails if an unsupported mandatory argument or value grants the command or consults the local fallback; TestRFC8907MandatorySessionPolicyDenied fails if session login grants profiles under an unenforceable mandatory argument; positives TestRFC8907MandatoryCommandPolicyAccepted (recognized mandatory values authorize) and TestRFC8907OptionalCommandPolicyIgnored (unsupported optional is disregarded).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907MandatoryCommandPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L61) | unit/verify | unproven |
| negative | [`TestRFC8907MandatorySessionPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC8907MandatoryCommandPolicyAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC8907OptionalCommandPolicyIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L79) | unit/verify | unproven |

### [`RFC8907-3.7-1`](#rfc8907-3.7-1)

Usernames MUST be encoded and handled using the UsernameCasePreserved Profile specified in [RFC8265]. (§3.7, Treatment of Text Strings)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a username sent without UsernameCasePreserved enforcement: no width mapping, no NFC, case folded, or a PRECIS-disallowed string (control, private use, ZWJ outside context, invalid UTF-8, bidi-rule violation) sent. (b) TestRFC8907UsernameProfileOnWire reads the username off the wire for all three services and requires "Ａlice\u0301" to arrive as "Alicé" (width-mapped, NFC, case kept); TestRFC8907ForbiddenUsernameNeverSent requires errRequestInvalid and no server contact for "alice\x00", "alice\ue000", "a\u200db", "\xff" and "\u05d0a" on every service.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ForbiddenUsernameNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC8907UsernameProfileOnWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L63) | unit/verify | unproven |

### [`RFC8907-3.7-2`](#rfc8907-3.7-2)

All other text fields in TACACS+ MUST be treated as printable byte arrays of US-ASCII as defined by [RFC0020]. The term "printable" used here means the fields MUST exclude the "Control Characters" defined in Section 5.2 of [RFC0020]. (§3.7, Treatment of Text Strings)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/tacacs/rfc8907_dispatch_wire_test.go::TestRFC8907TrustedDispatchIdentitiesReachWire moved. Rejudged 2026-09-29. With the old units, every text field Ze writes or reads is now refused on NUL, 0x1f, DEL and a non-ASCII byte: authentication port/rem_addr, authorization port/rem_addr/argument, accounting port/rem_addr/argument, authentication reply server_msg, authorization reply server_msg/data/argument, accounting reply server_msg/data (RFC 6.2/7.2 route those data fields to Section 3.7). Negative request cases leave every other field empty/valid, so validateText is the refusing check. Authentication reply data and START/CONTINUE data are correctly excluded (not a printable text encoding). Positive TestRFC8907PrintableTextInEveryField carries all 95 printable characters through each field.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ControlCharactersRefusedInEveryField`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestRFC8907NonPrintableTextRejected`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestRFC8907PrintableTextInEveryField`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC8907TrustedDispatchIdentitiesReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L172) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC8907PrintableTextRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L108) | unit/verify | revert, verified |

### [`RFC8907-4.1-1`](#rfc8907-4.1-1)

To signal that any variable-length data fields are unused, the corresponding length values are set to zero. Such fields MUST be ignored, and treated as if not present. (§4.1, The TACACS+ Packet Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. New units extend the old data-only proof to server_msg, data and reply arguments in all three reply kinds: TestRFC8907ZeroLengthFieldsAreNotPresent requires empty msg/data and that zero-length arguments yield no entry (Args == [a=b]); TestRFC8907PresentFieldsFollowZeroLengthOnes requires present fields read at the offsets the zero-length fields leave (read-past goes red). Request-side fields are written by Ze, not read.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ZeroLengthFieldsAreNotPresent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L230) | unit/verify | revert, verified |
| negative | [`TestRFC8907PresentZeroByteIsNotReadAsAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC8907PresentFieldsFollowZeroLengthOnes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC8907ZeroLengthDataIsReadAsAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L59) | unit/verify | revert, verified |

### [`RFC8907-4.1-2`](#rfc8907-4.1-2)

The first packet in a session MUST have the sequence number 1, and each subsequent packet will increment the sequence number by one. (§4.1, The TACACS+ Packet Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a session's first packet with a seq_no other than 1, or accepting a reply that does not follow it. (b) TestRFC8907FirstPacketCarriesSequenceOne requires requests[0].SeqNo == 1 on each new connection; TestRFC8907ReplyReusingSequenceOneIsRefused requires a reply carrying seq_no 1 to fail Authenticate and validateResponseHeader to report "sequence mismatch". The increment clause is descriptive ("will").

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ReplyReusingSequenceOneIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L210) | unit/verify | revert, verified |
| positive | [`TestRFC8907FirstPacketCarriesSequenceOne`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L194) | unit/verify | revert, verified |

### [`RFC8907-4.1-3`](#rfc8907-4.1-3)

All other bits MUST be ignored when reading, and SHOULD be set to zero when writing. (§4.1, The TACACS+ Packet Header)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: refusing a reply because undefined flag bits are set, or letting undefined bits mask a defined flag. (b) TestRFC8907UnknownHeaderFlagsIgnored sets reply flags 0xf2 (undefined bits only) and fails unless status == PASS with no error; TestRFC8907UnknownFlagsDoNotMaskDowngrade sets 0x81 and fails unless the result is FAIL. The SHOULD-zero-on-write half is not a MUST.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907UnknownFlagsDoNotMaskDowngrade`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L165) | unit/verify | unproven |
| positive | [`TestRFC8907UnknownHeaderFlagsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L153) | unit/verify | unproven |

### [`RFC8907-4.3-1`](#rfc8907-4.3-1)

The client MUST NOT send a second packet on a connection until single-connect status has been established. (§4.3, Single Connection Mode)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a second packet on a connection before the server echoed single-connect. (b) TestRFC8907NoSecondPacketWithoutSingleConnect (no echo) requires two connections and requestsOn(0) of length 1; TestRFC8907SecondSessionReusesEstablishedConnection (echo) requires one connection carrying two requests.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907NoSecondPacketWithoutSingleConnect`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L240) | unit/verify | revert, verified |
| positive | [`TestRFC8907SecondSessionReusesEstablishedConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L229) | unit/verify | revert, verified |

### [`RFC8907-4.3-2`](#rfc8907-4.3-2)

No provision is made for changing Single Connection Mode after the first two packets; the client and server MUST ignore the flag after the second packet on a connection. (§4.3, Single Connection Mode)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: acting on the single-connect flag after the second packet, by leaving the pool when a later reply clears it or by re-signalling it. (b) TestRFC8907FlagClearedOnLaterReplyIsIgnored requires one connection carrying three requests although replies after the first clear the flag; TestRFC8907ClientDoesNotResignalSingleConnect requires requests[1].Flags and requests[2].Flags == 0.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ClientDoesNotResignalSingleConnect`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRFC8907FlagClearedOnLaterReplyIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L254) | unit/verify | revert, verified |

### [`RFC8907-4.3-3`](#rfc8907-4.3-3)

The client MUST accommodate such closures on a TCP session even after Single Connection Mode has been established. (§4.3, Single Connection Mode)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907ServerClosureOfPooledConnectionIsAccommodated (closure between sessions) and new TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated (server reads session 3's request on the established single-connect TCP, closes unanswered; session 3 must PASS on a fresh TCP, counts [3,1]) both go red if the client fails a session after the closure; producer sendReceive pooled retry. The negative-tagged TestRFC8907ClosureOnFreshDialIsNotRetried still asserts a neighbouring property (fresh-dial closure not retried) and is mistagged; it adds nothing to this row and should be retagged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ClosureOnFreshDialIsNotRetried`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L294) | unit/verify | revert, verified |
| negative | [`TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_header_test.go#L324) | unit/verify | revert, verified |
| positive | [`TestRFC8907ServerClosureOfPooledConnectionIsAccommodated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L281) | unit/verify | revert, verified |

### [`RFC8907-4.4-1`](#rfc8907-4.4-1)

The server responds with an ERROR to indicate that the processing of the request did not complete. The client cannot apply the result, and it MUST behave as if the server could not be connected to. (§4.4, Session Completion)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: applying an ERROR reply as a result (terminal rejection) instead of behaving as if the server were unreachable. (b) TestRFC8907ErrorUsesBackupServer fails for authentication, authorization and accounting unless the backup is contacted and its status 1 is returned with no error; TestRFC8907FailDoesNotUseBackupServer is the discriminating pair: a FAIL keeps failStatus and never contacts the backup.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907FailDoesNotUseBackupServer`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC8907ErrorUsesBackupServer`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L177) | unit/verify | unproven |

### [`RFC8907-4.4-2`](#rfc8907-4.4-2)

If Single Connection Mode was enabled, but an ERROR occurred due to connection issues (such as an incorrect secret (see Section 4.5)), then any further new sessions MUST NOT be accepted on the connection. (§4.4, Session Completion)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a new session on a single-connect connection after a reply on it failed validation. (b) TestRFC8907NoNewSessionOnBrokenPooledConnection (session_id corrupted on the second reply) requires two connections and requestsOn(0) of length 2, so a third request on the broken TCP goes red; TestRFC8907HealthyPooledConnectionKeepsAcceptingSessions requires one connection carrying three requests.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907HealthyPooledConnectionKeepsAcceptingSessions`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L328) | unit/verify | revert, verified |
| positive | [`TestRFC8907NoNewSessionOnBrokenPooledConnection`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L313) | unit/verify | revert, verified |

### [`RFC8907-4.4-3`](#rfc8907-4.4-3)

Once all active sessions are completed, then the connection MUST be closed. (§4.4, Session Completion)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: leaving the broken single-connect connection open. (b) TestRFC8907BrokenPooledConnectionIsClosed requires waitClosed on the server side of the broken TCP; TestRFC8907HealthyPooledConnectionStaysOpen requires isClosed false after three healthy sessions.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907HealthyPooledConnectionStaysOpen`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L351) | unit/verify | revert, verified |
| positive | [`TestRFC8907BrokenPooledConnectionIsClosed`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L339) | unit/verify | revert, verified |

### [`RFC8907-5.1-1`](#rfc8907-5.1-1)

The username is optional in this packet, depending upon the class of authentication. If it is absent, the client MUST set user_len to 0. (§5.1, The Authentication START Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a START with no username whose user_len is not 0. (b) TestRFC8907AuthenStartAbsentUserWritesZeroLength requires body[4] == 0, no user bytes and the port at offset 8; TestRFC8907AuthenStartPresentUserWritesItsLength requires body[4] == len("alice") and the user bytes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AuthenStartPresentUserWritesItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthenStartAbsentUserWritesZeroLength`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L47) | unit/verify | revert, verified |

### [`RFC8907-5.4-1`](#rfc8907-5.4-1)

When the REPLY status equals TAC_PLUS_AUTHEN_STATUS_GETDATA, TAC_PLUS_AUTHEN_STATUS_GETUSER, or TAC_PLUS_AUTHEN_STATUS_GETPASS, authentication continues and the server SHOULD provide server_msg content for the client to prompt the user for more information. The client MUST then return a CONTINUE packet containing the requested information in the user_msg field. (§5.4, Description of Authentication Process)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4-1, so no unit is bound to it.

### [`RFC8907-5.4-2`](#rfc8907-5.4-2)

If the information being requested by the server from the client is sensitive, then the server should set the TAC_PLUS_REPLY_FLAG_NOECHO flag. When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered. (§5.4, Description of Authentication Process)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4-2, so no unit is bound to it.

### [`RFC8907-5.4.2.2-1`](#rfc8907-5.4.2.2-1)

The entire exchange MUST consist of a single START packet and a single REPLY. (§5.4.2.2, PAP Login)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a PAP login with more than one START/REPLY, including a CONTINUE. (b) TestRFC8907PAPExchangeIsOneStartOneReply requires exactly one authentication request on the connection; TestRFC8907PAPClientSendsNoContinue replies GETPASS and requires an error, a nil reply and requestsOn(0) of length 1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907PAPClientSendsNoContinue`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L394) | unit/verify | revert, verified |
| positive | [`TestRFC8907PAPExchangeIsOneStartOneReply`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L382) | unit/verify | revert, verified |

### [`RFC8907-5.4.2.2-2`](#rfc8907-5.4.2.2-2)

The START packet MUST contain a username and the data field MUST contain the PAP ASCII password. (§5.4.2.2, PAP Login)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907PAPStartOnWireCarriesUserAndPassword decodes the START from the wire and requires user=alice and data=the password. Negative TestRFC8907PAPStartWithoutUsernameNeverSent requires errRequestInvalid and that the server is never contacted, the tagged red assertion for 'MUST contain a username' the old verdict lacked.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907PAPStartWithoutUsernameNeverSent`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L170) | unit/verify | revert, verified |
| negative | [`TestRFC8907PAPStartRefusesPasswordItCannotCarry`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRFC8907PAPStartOnWireCarriesUserAndPassword`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L158) | unit/verify | revert, verified |
| positive | [`TestRFC8907PAPStartCarriesUserAndPassword`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L75) | unit/verify | revert, verified |

### [`RFC8907-5.4.2.3-1`](#rfc8907-5.4.2.3-1)

The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response. (§5.4.2.3, CHAP Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.3-1, so no unit is bound to it.

### [`RFC8907-5.4.2.4-1`](#rfc8907-5.4.2.4-1)

The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.4, MS-CHAP v1 Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.4-1, so no unit is bound to it.

### [`RFC8907-5.4.2.5-1`](#rfc8907-5.4.2.5-1)

The entire exchange MUST consist of a single START packet and a single REPLY. The START packet MUST contain the username in the user field, and the data field will be a concatenation of the PPP id, the MS-CHAP challenge, and the MS-CHAP response. (§5.4.2.5, MS-CHAP v2 Login)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.5-1, so no unit is bound to it.

### [`RFC8907-5.4.2.6-1`](#rfc8907-5.4.2.6-1)

In order to readily distinguish "ENABLE" requests from other types of request, the value of the authen_service field MUST be set to TAC_PLUS_AUTHEN_SVC_ENABLE when requesting an ENABLE. (§5.4.2.6, Enable Requests)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-5.4.2.6-1, so no unit is bound to it.

### [`RFC8907-5.4.2.6-2`](#rfc8907-5.4.2.6-2)

It MUST NOT be set to this value when requesting any other operation. (§5.4.2.6, Enable Requests)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 on the narrowed row (verbatim second sentence of 5.4.2.6; the first sentence is RFC8907-5.4.2.6-1, feature-declined; correction recorded). Negative TestRFC8907NoRequestCarriesEnableService checks the authen_service octet of all five requests Ze sends (PAP START and session/command authorization from the wire, accounting START/STOP) and requires none be 0x02; it is no longer the positive's assertion in a second hat. Positive TestRFC8907PAPLoginOnWireCarriesLoginService requires LOGIN on the wire START.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907NoRequestCarriesEnableService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L458) | unit/verify | revert, verified |
| negative | [`TestRFC8907LoginStartNeverCarriesEnableService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestRFC8907PAPLoginOnWireCarriesLoginService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L453) | unit/verify | revert, verified |
| positive | [`TestRFC8907LoginStartCarriesLoginService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L97) | unit/verify | revert, verified |

### [`RFC8907-5.4.3-1`](#rfc8907-5.4.3-1)

If a client does not implement the TAC_PLUS_AUTHEN_STATUS_RESTART option, then it MUST process the response as if the status was TAC_PLUS_AUTHEN_STATUS_FAIL. (§5.4.3, Aborting an Authentication Session)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: processing RESTART as anything but FAIL. (b) TestRFC8907RestartIsProcessedAsFail requires ErrorIs aaa.ErrAuthRejected, Authenticated false and Source == backendName; TestRFC8907ErrorIsNotProcessedAsFail is the discriminating pair: ERROR must not be ErrAuthRejected.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ErrorIsNotProcessedAsFail`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC8907RestartIsProcessedAsFail`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L75) | unit/verify | revert, verified |

### [`RFC8907-6.1-1`](#rfc8907-6.1-1)

The user_len MUST indicate the length of the user field, in bytes. (§6.1, The Authorization REQUEST Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Positive TestRFC8907AuthorUserLenCountsBytes: 'Alicé' writes user_len 6 (a rune count writes 5) and a 255-byte multibyte user writes 255, with the user bytes read back at that length. Negative TestRFC8907AuthorUserLenRefusesRuneCount: 128 runes / 256 bytes is refused, which a rune-count producer would accept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AuthorUserLenRefusesRuneCount`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L190) | unit/verify | revert, verified |
| negative | [`TestRFC8907AuthorRequestRefusesUserItCannotMeasure`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthorUserLenCountsBytes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthorRequestUserLenMatchesUser`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L114) | unit/verify | revert, verified |

### [`RFC8907-6.1-2`](#rfc8907-6.1-2)

An argument name MUST NOT contain either of the separators. (§6.1, The Authorization REQUEST Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an argument whose name contains = or *. (b) TestRFC8907ArgumentNamesCarryNoSeparator requires every built name to be service, cmd or cmd-arg; TestRFC8907ArgumentValueKeepsSeparatorOutOfName requires splitTacacsArgs("set key=a*b value=*") to equal exactly service=shell, cmd=set, cmd-arg=key=a*b, cmd-arg=value=*, so a token carried as a name goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ArgumentValueKeepsSeparatorOutOfName`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestRFC8907ArgumentNamesCarryNoSeparator`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L141) | unit/verify | revert, verified |

### [`RFC8907-6.2-1`](#rfc8907-6.2-1)

If the status equals TAC_PLUS_AUTHOR_STATUS_PASS_ADD, then the arguments specified in the request are authorized and the arguments in the response MUST be applied according to the rules described above. (§6.2, The Authorization REPLY Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: under PASS_ADD, dropping the requested arguments or not applying the returned ones. (b) TestRFC8907PassAddRetainsCommand fails if the requested command is lost; TestRFC8907PassAddCannotIgnoreCommandExtension fails if an added mandatory cmd-arg is ignored or falls back to local policy; TestRFC8907SessionPrivilegeApplied fails unless the returned priv-lvl selects its mapped profile.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907PassAddCannotIgnoreCommandExtension`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC8907PassAddRetainsCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L89) | unit/verify | unproven |
| positive | [`TestRFC8907SessionPrivilegeApplied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L169) | unit/verify | unproven |

### [`RFC8907-6.2-2`](#rfc8907-6.2-2)

If the status equals TAC_PLUS_AUTHOR_STATUS_PASS_REPL, then the client MUST use the authorization argument-value pairs (if any) in the response instead of the authorization argument- value pairs from the request. (§6.2, The Authorization REPLY Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: under PASS_REPL, using the request arguments instead of the reply ones. (b) TestRFC8907PassReplCannotAuthorizeOriginalCommand fails if an empty, incomplete, changed, reordered or re-split replacement authorizes the original command or consults the local fallback; TestRFC8907IncompleteSessionReplacementDenied fails if session replacement inherits service/cmd from the request; positives TestRFC8907PassReplPreservesExactCommand and TestRFC8907SessionReplacementMapsZero (reply priv-lvl 0 applied).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907IncompleteSessionReplacementDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L223) | unit/verify | unproven |
| negative | [`TestRFC8907PassReplCannotAuthorizeOriginalCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L118) | unit/verify | unproven |
| positive | [`TestRFC8907PassReplPreservesExactCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC8907SessionReplacementMapsZero`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L200) | unit/verify | unproven |

### [`RFC8907-6.2-3`](#rfc8907-6.2-3)

If the status equals TAC_PLUS_AUTHOR_STATUS_FAIL, then the requested authorization MUST be denied. (§6.2, The Authorization REPLY Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: allowing a request the server answered with FAIL. (b) TestRFC8907AuthorizationFailDenies requires Authorize false and zero calls to a permissive local policy; TestRFC8907AuthorizationPassAddIsNotDenied requires PASS_ADD to allow.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AuthorizationPassAddIsNotDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthorizationFailDenies`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L113) | unit/verify | revert, verified |

### [`RFC8907-7.2-1`](#rfc8907-7.2-1)

The STOP flag MUST NOT be set in conjunction with the WATCHDOG flag. (§7.2, The Accounting REPLY Packet Body)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: STOP with WATCHDOG set. (b) TestRFC8907AccountingStopWatchdogCombinationRefused requires errRequestInvalid for STOP|WATCHDOG; TestRFC8907StopRecordNeverCarriesWatchdog requires Flags&WATCHDOG == 0 on the queued STOP; TestRFC8907StopRecordCarriesStopFlagOnly requires Flags == STOP.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907StopRecordNeverCarriesWatchdog`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L71) | unit/verify | revert, verified |
| negative | [`TestRFC8907AccountingStopWatchdogCombinationRefused`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_validation_test.go#L299) | unit/verify | unproven |
| positive | [`TestRFC8907StopRecordCarriesStopFlagOnly`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L63) | unit/verify | revert, verified |

### [`RFC8907-8-1`](#rfc8907-8-1)

Clients MUST use these arguments when supporting the corresponding use cases. (§8, Argument-Value Pairs)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Every request kind Ze sends is now asserted: authorization (command request exactly service/cmd/cmd-arg, session request service=shell cmd=) in TestRFC8907AuthorizationRequestsUseDictionaryArguments and TestRFC8907AuthorizationRequestsCarryNoForeignArgument, accounting in the old units. Exact equality catches a private name substituted for a dictionary one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907CommandRecordsCarryNoForeignArgument`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestRFC8907AuthorizationRequestsCarryNoForeignArgument`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L285) | unit/verify | revert, verified |
| positive | [`TestRFC8907CommandRecordsUseDictionaryArguments`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthorizationRequestsUseDictionaryArguments`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L278) | unit/verify | revert, verified |

### [`RFC8907-8.1-1`](#rfc8907-8.1-1)

TACACS+ implementations MUST verify that they can accommodate the lengths of numeric arguments before attempting to process them. If the length cannot be accommodated, then the argument MUST be regarded as not handled and the logic in "Authorization" (Section 6.1) regarding the processing of arguments MUST be applied. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: processing a numeric argument whose length Ze cannot accommodate, or not applying Section 6.1 to it. (b) TestRFC8907InvalidMandatoryPrivilegeDenied asserts ErrAuthRejected and zero profiles for priv-lvl=18446744073709551631 and a 242-digit value (mandatory -> authorization failed); TestRFC8907OverlongOptionalPrivilegeCannotElevate asserts the overlong optional priv-lvl is disregarded (profile ops); TestRFC8907SessionPrivilegeApplied is the positive (priv-lvl=15 -> admin). priv-lvl is the only numeric argument Ze processes; timeout and others are refused as unsupported mandatory.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907InvalidMandatoryPrivilegeDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L178) | unit/verify | unproven |
| positive | [`TestRFC8907OverlongOptionalPrivilegeCannotElevate`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L191) | unit/verify | unproven |
| positive | [`TestRFC8907SessionPrivilegeApplied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L168) | unit/verify | unproven |

### [`RFC8907-8.1-2`](#rfc8907-8.1-2)

Absolute date/times are specified in seconds since the epoch, 12:00am, January 1, 1970. The time zone MUST be UTC unless a time zone argument is specified. (§8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. stop_time, the missing absolute time, is now proven: TestRFC8907StopTimeIsEpochSeconds bounds it to [before, after] Unix seconds and TestRFC8907StopTimeIgnoresLocalZone sets time.Local +5h and requires no shift and no timezone argument. start_time stays proven by the old units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907StartTimeIgnoresLocalZone`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L132) | unit/verify | revert, verified |
| negative | [`TestRFC8907StopTimeIgnoresLocalZone`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L358) | unit/verify | revert, verified |
| positive | [`TestRFC8907StartTimeIsEpochSeconds`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestRFC8907StopTimeIsEpochSeconds`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L344) | unit/verify | revert, verified |

### [`RFC8907-8.2-1`](#rfc8907-8.2-1)

Specifying a service argument indicates that this is a request for authorization or accounting of that service. For example: "shell", "tty-server", "connection", "system" and "firewall"; others may be chosen for the required application. This argument MUST always be included. (§8.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Service is now asserted on every request kind: command and session authorization (service=shell first) and accounting START/STOP (TestRFC8907EveryRequestKindLeadsWithService); TestRFC8907NoRequestKindOmitsService requires exactly one service in each, including the session request with empty cmd and records for an argless command.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907NoRequestKindOmitsService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L306) | unit/verify | revert, verified |
| negative | [`TestRFC8907ServiceArgumentPresentForEmptyCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L167) | unit/verify | revert, verified |
| positive | [`TestRFC8907EveryRequestKindLeadsWithService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestRFC8907ServiceArgumentAlwaysFirst`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L161) | unit/verify | revert, verified |

### [`RFC8907-8.2-2`](#rfc8907-8.2-2)

The "cmd" argument MUST be specified if service equals "shell". (§8.2, Authorization Arguments)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. Every service=shell request now carries cmd under a tagged unit: command request cmd=show, session request cmd=, accounting START/STOP cmd=show (TestRFC8907EveryShellRequestCarriesCmd); TestRFC8907ShellRequestNeverLacksCmd covers the edge inputs (empty session cmd, argless 'show' records); old units cover the empty command in splitTacacsArgs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907ShellRequestNeverLacksCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L334) | unit/verify | revert, verified |
| negative | [`TestRFC8907CmdArgumentPresentForEmptyCommand`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestRFC8907EveryShellRequestCarriesCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L321) | unit/verify | revert, verified |
| positive | [`TestRFC8907CmdArgumentFollowsShellService`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L173) | unit/verify | revert, verified |

### [`RFC8907-8.3-1`](#rfc8907-8.3-1)

The following arguments are defined for TACACS+ accounting only. They MUST precede any argument-value pairs that are defined in "Authorization" (Section 6). (§8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a Section 8.2 argument (service, cmd, cmd-arg) before task_id/start_time/stop_time in an accounting record. (b) TestRFC8907AccountingArgumentsPrecedeAuthorizationOnes asserts task_id and start_time (START) and task_id and stop_time (STOP) index below service; TestRFC8907NoAuthorizationArgumentPrecedesAccountingOne fails on any service/cmd/cmd-arg seen before task_id or start_time in START. Both records share accountingArguments.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907NoAuthorizationArgumentPrecedesAccountingOne`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC8907AccountingArgumentsPrecedeAuthorizationOnes`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L155) | unit/verify | revert, verified |

### [`RFC8907-8.3-2`](#rfc8907-8.3-2)

Start and stop records for the same event MUST have matching task_id argument values. (§8.3, Accounting Arguments)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: START and STOP of one command with different task_id values. (b) TestRFC8907StartAndStopShareTaskID require.Equal(startID, stopID); TestRFC8907SecondCommandCarriesOtherTaskID require.NotEqual for a second command.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907SecondCommandCarriesOtherTaskID`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L207) | unit/verify | revert, verified |
| positive | [`TestRFC8907StartAndStopShareTaskID`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L191) | unit/verify | revert, verified |

### [`RFC8907-8.3-3`](#rfc8907-8.3-3)

The client MUST ensure that active task_ids are not duplicated; a client MUST NOT reuse a task_id in a start record until it has sent a stop record for that task_id. (§8.3, Accounting Arguments)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: two active START records sharing a task_id, or a task_id reused in START before its STOP. (b) TestRFC8907ActiveTaskIDsAreDistinct fails on any repeat among sixteen START records with no STOP; TestRFC8907TaskIDNeverReappears asserts a used task_id never reappears in eight later STARTs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907TaskIDNeverReappears`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L236) | unit/verify | revert, verified |
| positive | [`TestRFC8907ActiveTaskIDsAreDistinct`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L221) | unit/verify | revert, verified |

### [`RFC8907-8.3-4`](#rfc8907-8.3-4)

To support this mode of operation, TACACS+ client devices MUST be configured to send an accounting start packet for every command entered, irrespective of how the commands were authorized. (§8.3, Accounting Arguments)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: cmd/ze/hub/rfc8907_aaa_lifecycle_test.go::TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap, internal/component/plugin/server/rfc8907_command_test.go::TestDispatcherAccountingWithoutUsername, internal/component/plugin/server/rfc8907_command_test.go::TestDispatcherAccountsRefusedCommands, internal/component/tacacs/rfc8907_dispatch_wire_test.go::TestRFC8907OversizedCommandAccountingStillReachesWire, internal/component/tacacs/rfc8907_dispatch_wire_test.go::TestRFC8907TrustedDispatchIdentitiesReachWire moved. (a) forbidden: an entered command with no accounting START, including a command authorization denied or that is unknown or malformed. (b) TestDispatcherAccountsRefusedCommands asserts starts == [input] for a denied, an unknown and a malformed command; TestRFC8907TypedDispatchAccountsDeniedCommand asserts START and STOP for a denied typed dispatch; api_test.go denied-stream and TACACS wire tests (rfc8907_dispatch_wire_test.go) assert real START packets; TestRFC8907AccountingRequiresDestination refuses accounting with no destination at Build.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907BundleRetirementWaitsForAccountingPair`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_aaa_lifecycle_test.go#L325) | unit/verify | unproven |
| negative | [`TestAPIStreamSourceAuthorizesReadOnly`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_api_test.go#L355) | unit/verify | unproven |
| negative | [`TestDispatcherAccountsRefusedCommands`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_command_test.go#L1286) | unit/verify | unproven |
| negative | [`TestRFC8907AccountingStopDrainsAcceptedStarts`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accountant_test.go#L90) | unit/verify | unproven |
| negative | [`TestRFC8907OversizedCommandAccountingStillReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L300) | unit/verify | unproven |
| negative | [`TestRFC8907AccountingRequiresDestination`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L123) | unit/verify | unproven |
| positive | [`TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_aaa_lifecycle_test.go#L231) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestAPIStreamSourceRunsStreamingHandler`](https://github.com/ze-software/ze/blob/main/cmd/ze/hub/rfc8907_api_test.go#L289) | unit/verify | unproven |
| positive | [`TestRFC8907PluginDispatchAccountsCommand`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L111) | unit/verify | unproven |
| positive | [`TestRFC8907TypedDispatchAccountsDeniedCommand`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L33) | unit/verify | unproven |
| positive | [`TestDispatcherAccountingWithoutUsername`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_command_test.go#L1222) | unit/verify | unproven |
| positive | [`TestRFC8907AccountingSaturationRetainsEveryStart`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accountant_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC8907TrustedDispatchIdentitiesReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_dispatch_wire_test.go#L171) | unit/verify | unproven |

### [`RFC8907-8.3-5`](#rfc8907-8.3-5)

These "Command Accounting" packets MUST include the "service" and "cmd" arguments, and if needed, the "cmd-arg" arguments detailed in Section 8.2. (§8.3, Accounting Arguments)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a command accounting record without service or cmd, or dropping needed cmd-arg arguments. (b) TestRFC8907CommandRecordsCarryServiceCmdAndArgs asserts service=shell, cmd=show, cmd-arg=bgp, cmd-arg=summary in START and STOP; TestRFC8907EmptyCommandRecordStillCarriesServiceAndCmd asserts service=shell and cmd= for an empty command.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907EmptyCommandRecordStillCarriesServiceAndCmd`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC8907CommandRecordsCarryServiceCmdAndArgs`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_accounting_test.go#L254) | unit/verify | revert, verified |

### [`RFC8907-10.5-1`](#rfc8907-10.5-1)

New implementations, and upgrades of current implementations, MUST implement these recommendations. (§10.5, TACACS+ Best Practices)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-10.5-1, so no unit is bound to it.

### [`RFC8907-10.5.1-1`](#rfc8907-10.5.1-1)

TACACS+ servers and clients MUST treat shared secrets as sensitive data to be managed securely, as would be expected for other sensitive data such as identity credential information. (§10.5.1, Shared Secrets)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/plugin/server/rfc8907_accounting_test.go::TestRFC8907AccountingRedactsConfigSecretWithoutChangingExecution moved. (a) forbidden: exposing the shared secret in display, diagnostics, logs or accounting records. (b) TestRFC8907SharedSecretConfigBuildAndDisplay asserts the display tree masks the key while the live key still decodes a real exchange; TestRFC8907SharedSecretDiagnosticRedaction fails on the key, its base64 or byte list in fmt, %#v, JSON and slog output; TestRFC8907SharedSecretCommandRedaction fails on secret words in DisplayCommand; TestRFC8907AccountingRedactsConfigSecretWithoutChangingExecution asserts accounting tokens carry the placeholder.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907AccountingRedactsConfigSecretWithoutChangingExecution`](https://github.com/ze-software/ze/blob/main/internal/component/plugin/server/rfc8907_accounting_test.go#L86) | unit/verify | unproven |
| negative | [`TestRFC8907SharedSecretCommandRedaction`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L149) | unit/verify | unproven |
| negative | [`TestRFC8907SharedSecretDiagnosticRedaction`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L87) | unit/verify | unproven |
| positive | [`TestRFC8907SharedSecretConfigBuildAndDisplay`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L37) | unit/verify | unproven |

### [`RFC8907-10.5.1-2`](#rfc8907-10.5.1-2)

* TACACS+ servers and clients MUST support shared keys that are at least 32 characters long. (§10.5.1, Shared Secrets)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: rejecting or truncating a shared key of 32 characters or more. (b) TestRFC8907LongSharedKeysAuthenticate completes a PASS exchange with 32- and 300-character keys; TestRFC8907SharedKeyIsNotTruncated asserts the pad for 32 characters differs from the pad for the first 31; TestRFC8907SharedSecretConfigBuildAndDisplay drives a 32-character key through Build into a real accounting exchange.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907SharedKeyIsNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_body_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestRFC8907LongSharedKeysAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_connection_test.go#L407) | unit/verify | revert, verified |
| positive | [`TestRFC8907SharedSecretConfigBuildAndDisplay`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_security_config_test.go#L38) | unit/verify | unproven |

### [`RFC8907-10.5.4-1`](#rfc8907-10.5.4-1)

The cost of the flexibility is that administrators and implementers MUST ensure that the argument and value pairs shared between the clients and servers have consistent interpretation. (§10.5.4, Authorization)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29. The consistent interpretation of shared values Ze consumes is now proven: priv-lvl 0/1/15 select exactly their profiles, and 16, ' 15', '0x0f', '1.5' are rejected (ErrAuthRejected, no profile); full-width digits refused with no profile; a PASS_REPL naming the same cmd/cmd-arg in order authorizes, and a case or order change denies. Old units keep the unrecognized-mandatory-argument case.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907SharedArgumentsNeverReinterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L393) | unit/verify | revert, verified |
| negative | [`TestRFC8907MandatoryCommandPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestRFC8907MandatorySessionPolicyDenied`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L210) | unit/verify | revert, verified |
| positive | [`TestRFC8907SharedArgumentsKeepDefinedMeaning`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_arguments_test.go#L379) | unit/verify | revert, verified |
| positive | [`TestRFC8907MandatoryCommandPolicyAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_authorization_test.go#L51) | unit/verify | revert, verified |

### [`RFC8907-4.6-2`](#rfc8907-4.6-2)

A request MUST be dropped if TAC_PLUS_UNENCRYPTED_FLAG is set to true. (§4.5, Data Obfuscation; stable historical ID)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8907-4.6-2, so no unit is bound to it.

### [`RFC8907-x-2`](#rfc8907-x-2)

This document deprecates the redirection mechanism using the TAC_PLUS_AUTHEN_STATUS_FOLLOW option, which was included in "The Draft". As part of this process, the secret key for a new server was sent to the client. This public exchange of secret keys means that once one session is broken, it may be possible to leverage that key to attacking connections to other servers. This mechanism MUST NOT be used in modern deployments. It MUST NOT be used outside a secured deployment. (§10.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the client using the FOLLOW redirection, by contacting the named server or treating FOLLOW as success or fallback. (b) TestRFC8907FollowTargetIsNeverContacted fails if the FOLLOW target receives a connection; TestRFC8907FollowIsNotAuthenticated asserts ErrAuthRejected and not authenticated; TestRFC8907AuthorizationFollowDeniesWithoutFallback asserts denial with zero local fallback calls.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8907FollowTargetIsNeverContacted`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestRFC8907AuthorizationFollowDeniesWithoutFallback`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L135) | unit/verify | unproven |
| positive | [`TestRFC8907FollowIsNotAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/tacacs/rfc8907_status_test.go#L127) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc8907.txt |
| Source fingerprint | 8bc695c4b9949009 |
| Record | rfc/extraction/rfc8907.json |
| Mapped sentences | 50 |
| Declined as scope | 54 |
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
