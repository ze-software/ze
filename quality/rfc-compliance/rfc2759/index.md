# RFC 2759 - Microsoft PPP CHAP Extensions, Version 2

Supported. Every requirement this repository extracted from RFC 2759, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 88.9% | 8 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 1 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 85.3% | 29 of 34 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 11 | of 9 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 11 |
| Discrimination records | 29 |
| Summary | `rfc/short/rfc2759.md` |
| Requirement shard | `rfc/requirements/rfc2759.md` |
| RFC text | `rfc/full/rfc2759.txt` |

## Enrolment

Enrolled: MS-CHAPv2 (EAP inside IKEv2): 6 MET (Response field validation, DOMAIN-strip, UTF-16LE hash) + 3 single-polarity positive (uppercase S=, 16-octet random challenges) + 3 gap (no MS-CHAPv2 Failure/C= packet, peer skips authenticator-response check, no E=691)

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Mutual authentication on the PPP path and the IPsec EAP path, with MPPE/MSK key derivation on the IPsec EAP path only. NtPasswordHash (UTF-16LE + MD4), ChallengeHash with DOMAIN-prefix stripping, ChallengeResponse (DES), GenerateAuthenticatorResponse, and authenticator-side Response validation (Value-Size=49, zero Reserved/Flags, uppercase S=). The EAP authenticator and peer roles are both implemented (internal/core/eap). The peer recomputes the expected Authenticator Response and compares it in constant time, and refuses the session when it does not match, so a Success packet is a claim the peer checks rather than one it trusts (handleMSCHAPv2Success, eap/peer.go). A refused credential draws an MS-CHAPv2 Failure packet (OpCode 4) carrying E=691, R=0, a fresh 32-digit C= challenge, V= and M=, rather than a bare EAP-Failure (sendFailure, eap/eap_mschapv2.go).

**What the ledger says remains:**

No MUST gap remains gated in [`rfc/short/rfc2759.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2759.md). The three that stood on 2026-08-30 are closed: x-6 the Failure packet and its C= field, x-7 the peer-side Authenticator Response check, and x-12 the E=691 error code.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC2759-x-1`](#rfc2759-x-1), [`RFC2759-x-2`](#rfc2759-x-2), [`RFC2759-x-3`](#rfc2759-x-3), [`RFC2759-x-4`](#rfc2759-x-4), [`RFC2759-x-6`](#rfc2759-x-6), [`RFC2759-x-7`](#rfc2759-x-7), [`RFC2759-x-10`](#rfc2759-x-10), [`RFC2759-x-12`](#rfc2759-x-12)

**Annotated (including scoped evidence) (1):** [`RFC2759-x-5`](#rfc2759-x-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2759-x-1` | 8 octets: Reserved, must be zero (§4) | MUST | 4 - Response Packet | **positive:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L43). **positive:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L92). **negative:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L63). **negative:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L114) |
| `RFC2759-x-2` | The Flag field is reserved for future use and MUST be zero. (§4) | MUST | 4 - Response Packet | **positive:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L45). **positive:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L99). **negative:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L76). **negative:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L122) |
| `RFC2759-x-3` | However, the Value field is sub-formatted differently as follows: 16 octets: Peer-Challenge 8 octets: Reserved, must be zero 24 octets: NT-Response 1 octet : Flags (§4) | MUST | 4 - Response Packet | **positive:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L47). **positive:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L72). **negative:** `unit/verify` [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L89). **negative:** `unit/verify` [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L106) |
| `RFC2759-x-4` | When computing the NT-Response field contents, only the user name is used, without any associated Windows NT domain name. This is true regardless of whether a Windows NT domain name is present in the Name field (see below). (§4) | MUST | 4 - Response Packet | **positive:** `unit/verify` [`TestChallengeHashExcludesDomainPrefix`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L164). **positive:** `unit/verify` [`TestRFC2759NTResponseUsesTheBareUserName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L132). **negative:** `unit/verify` [`TestChallengeHashExcludesDomainPrefix`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L175). **negative:** `unit/verify` [`TestRFC2759NTResponseUsesTheBareUserName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L150) |
| `RFC2759-x-5` | The hexadecimal digits A-F (if present) MUST be uppercase. (§5) | MUST | 5 - Success Packet | **positive:** `unit/verify` [`TestMSCHAPv2SuccessUppercaseHex`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L108). **negative:** no negative test. **{single-polarity}:** the authenticator only emits S= and forces uppercase via strings.ToUpper, and no code path can emit lowercase, so only the positive assertion is reachable (internal/core/eap/eap_mschapv2.go:148) |
| `RFC2759-x-6` | The "cccccccccccccccccccccccccccccccc" is the ASCII representation of a hexadecimal challenge value. This field MUST be exactly 32 octets long and MUST be present. (§6) | MUST | 6 - Failure Packet | **positive:** `unit/verify` [`TestRFC2759FailureCarriesFreshChallenge`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L197). **negative:** `unit/verify` [`TestRFC2759PeerRefusesFailureWithoutConformantChallenge`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L277) |
| `RFC2759-x-7` | If the authenticator response is either missing or incorrect, the peer MUST end the session. (§5) | MUST | 5 - Success Packet | **positive:** `unit/verify` [`TestRFC2759PeerAcceptsCorrectAuthenticatorResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_authenticator_response_test.go#L93). **negative:** `unit/verify` [`TestRFC2759PeerEndsSessionOnBadAuthenticatorResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_authenticator_response_test.go#L122) |
| `RFC2759-x-8` | MS-CHAP-V2 authenticators send an 16-octet challenge Value field. Peers need not duplicate Microsoft's algorithm for selecting the 16- octet value, but the standard guidelines on randomness [1,2,7] SHOULD be observed. (§3) | SHOULD | 3 - Challenge Packet | **positive:** `unit/verify` [`TestMSCHAPv2AuthChallengeRandom16`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L140). **positive:** `unit/verify` [`TestRFC2759AuthenticatorChallengeComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L219). **negative:** no negative test. **{single-polarity}:** the authenticator fills a [16]byte from crypto/rand, so length and source are assertable but randomness quality has no falsifying negative test (internal/core/eap/eap_mschapv2.go:49) |
| `RFC2759-x-9` | The Peer-Challenge field is a 16-octet random number. As the name implies, it is generated by the peer and is used in the calculation of the NT-Response field, below. Peers need not duplicate Microsoft's algorithm for selecting the 16-octet value, but the standard guidelines on randomness [1,2,7] SHOULD be observed. (§4) | SHOULD | 4 - Response Packet | **positive:** `unit/verify` [`TestMSCHAPv2PeerChallengeRandom16`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L330). **positive:** `unit/verify` [`TestRFC2759PeerChallengeComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_peer_challenge_test.go#L37). **positive:** `unit/verify` [`TestRFC2759PeerChallengeEntersTheNTResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L191). **negative:** no negative test. **{single-polarity}:** the peer fills a [16]byte from crypto/rand, assertable for length and source but not falsifiable for randomness quality (internal/core/eap/peer.go:195) |
| `RFC2759-x-10` | 0-to-256-unicode-char Password: 63 00 6C 00 69 00 65 00 6E 00 74 00 50 00 61 00 73 00 73 00 (§9.2) | MUST | 9.2 - Hash Example | **positive:** `unit/verify` [`TestNtPasswordHash`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L29). **positive:** `unit/verify` [`TestRFC2759PasswordHashVector`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L161). **negative:** `unit/verify` [`TestNtPasswordHash`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L37). **negative:** `unit/verify` [`TestRFC2759PasswordHashVector`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L176) |
| `RFC2759-x-12` | Peer Response/Challenge -> <- Failure (E=691 R=0) (Authenticator disconnects) (§9.1.3) | MUST | 9.1.3 - Failed authentication with no retry allowed | **positive:** `unit/verify` [`TestRFC2759AuthenticatorRefusesWithErrorCode691`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L104). **positive:** `unit/verify` [`TestRFC2759FailureInvitesNoRetry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L238). **negative:** `unit/verify` [`TestRFC2759AuthenticatorAcceptsMatchingNTResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L171). **negative:** `unit/verify` [`TestRFC2759FailureInvitesNoRetry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L255) |
| `RFC2759-x-13` | The "vvvvvvvvvv" is the ASCII representation of a decimal version code (need not be 10 digits) indicating the password changing protocol version supported on the server. For MS-CHAP-V2, this value SHOULD always be 3. (§6) | SHOULD | 6 - Failure Packet | **positive:** no positive test. **negative:** no negative test |
| `RFC2759-x-14` | As an implementation detail, the authenticator SHOULD limit the number of password retries allowed to make brute-force password guessing attacks more difficult. (§10) | SHOULD | 10 - Security Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2759 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2759-x-1`](#rfc2759-x-1)

8 octets: Reserved, must be zero (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender: the Response Value ze's peer emits has 8 zero Reserved octets (td[21:29]). Receiver: each of the 8 Reserved octets set to 1, each on a fresh copy of the authenticator at the Challenge, is refused through mschapv2Method.Process, so a check of the first octet only goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L114) | unit/verify | revert, verified |
| negative | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L43) | unit/verify | revert, verified |

### [`RFC2759-x-2`](#rfc2759-x-2)

The Flag field is reserved for future use and MUST be zero. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender: the Flags octet ze's peer emits is 0 (td[53]). Receiver: Flags 1 on a fresh authenticator copy is refused through Process.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L122) | unit/verify | revert, verified |
| negative | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L45) | unit/verify | revert, verified |

### [`RFC2759-x-3`](#rfc2759-x-3)

However, the Value field is sub-formatted differently as follows: 16 octets: Peer-Challenge 8 octets: Reserved, must be zero 24 octets: NT-Response 1 octet : Flags (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The layout ze's peer emits is read field by field: Value-Size 49, octets 0-15 equal the peer's Peer-Challenge, octets 24-47 the NT-Response over the authenticator's real Start challenge, Name after the 49 octets. Negative: Value-Size 48 and 50 each refused on a fresh copy.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L106) | unit/verify | revert, verified |
| negative | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L89) | unit/verify | revert, verified |
| positive | [`TestRFC2759PeerSendsTheResponseValueLayout`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestMSCHAPv2ResponseFieldValidation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L47) | unit/verify | revert, verified |

### [`RFC2759-x-4`](#rfc2759-x-4)

When computing the NT-Response field contents, only the user name is used, without any associated Windows NT domain name. This is true regardless of whether a Windows NT domain name is present in the Name field (see below). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Real path both roles: with Name DOMAIN\user the peer sends the full Name and an NT-Response equal to the one over the bare 'user', and the authenticator accepts it; the same Response with the NT-Response computed over the domain-qualified Name is refused through Process. Removing stripDomain from either side goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759NTResponseUsesTheBareUserName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L150) | unit/verify | revert, verified |
| negative | [`TestChallengeHashExcludesDomainPrefix`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestRFC2759NTResponseUsesTheBareUserName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestChallengeHashExcludesDomainPrefix`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L164) | unit/verify | revert, verified |

### [`RFC2759-x-5`](#rfc2759-x-5)

The hexadecimal digits A-F (if present) MUST be uppercase. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} carried on the row. Forbidden behaviour: lowercase a-f in the S= authenticator response. TestMSCHAPv2SuccessUppercaseHex forces hex letters (0xab bytes, guarded by ContainsAny "ABCDEF" so it is not vacuous) and fails on strings.ContainsAny(sHex,"abcdef") and on sHex!=ToUpper(sHex); dropping strings.ToUpper in sendSuccess turns it red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestMSCHAPv2SuccessUppercaseHex`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L108) | unit/verify | unproven |

### [`RFC2759-x-6`](#rfc2759-x-6)

The "cccccccccccccccccccccccccccccccc" is the ASCII representation of a hexadecimal challenge value. This field MUST be exactly 32 octets long and MUST be present. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clauses: present, exactly 32 octets, ASCII hex. Sender: TestRFC2759FailureCarriesFreshChallenge fails if Ze Failure has no C= (failureField !ok), len!=32, or a non-hex digit. Receiver negative: TestRFC2759PeerRefusesFailureWithoutConformantChallenge feeds absent, short (30), long (34), non-hex and empty C= and requires errors.Is(errFailureChallenge), no ack, no success; the conformant control is read as a refusal, so the cases discriminate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PeerRefusesFailureWithoutConformantChallenge`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L277) | unit/verify | unproven |
| positive | [`TestRFC2759FailureCarriesFreshChallenge`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L197) | unit/verify | unproven |

### [`RFC2759-x-7`](#rfc2759-x-7)

If the authenticator response is either missing or incorrect, the peer MUST end the session. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the peer continuing after a missing or incorrect authenticator response. Negative walks wrong S= (one digit flipped, same length and charset), absent Message, no S= prefix, short and non-hex S=; each fails on res.Err==nil or an acknowledgement, and the session end is asserted (later EAP-Success Discarded, not Done, not Succeeded). Positive: the correct S= is acknowledged with OpCode Success.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PeerEndsSessionOnBadAuthenticatorResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_authenticator_response_test.go#L122) | unit/verify | unproven |
| positive | [`TestRFC2759PeerAcceptsCorrectAuthenticatorResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_authenticator_response_test.go#L93) | unit/verify | unproven |

### [`RFC2759-x-8`](#rfc2759-x-8)

MS-CHAP-V2 authenticators send an 16-octet challenge Value field. Peers need not duplicate Microsoft's algorithm for selecting the 16- octet value, but the standard guidelines on randomness [1,2,7] SHOULD be observed. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row re-levelled MUST->SHOULD (only keyword in the span is SHOULD; correction 2026-09-28 in rfc/corrections/rfc2759.md). Single-polarity positive: with crypto/rand.Reader substituted, the Challenge from mschapv2Method.Start carries Value-Size 16 and exactly the substituted octets, so a math/rand or time-seeded source goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2759AuthenticatorChallengeComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L219) | unit/verify | revert, verified |
| positive | [`TestMSCHAPv2AuthChallengeRandom16`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_eap_mschapv2_test.go#L140) | unit/verify | revert, verified |

### [`RFC2759-x-9`](#rfc2759-x-9)

The Peer-Challenge field is a 16-octet random number. As the name implies, it is generated by the peer and is used in the calculation of the NT-Response field, below. Peers need not duplicate Microsoft's algorithm for selecting the 16-octet value, but the standard guidelines on randomness [1,2,7] SHOULD be observed. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Row re-levelled MUST->SHOULD (same correction). The source clause is on the crypto/rand.Reader substitution unit; the used-in-NT-Response clause now on TestRFC2759PeerChallengeEntersTheNTResponse: the wire NT-Response equals the one over the sent Peer-Challenge, and flipping one Peer-Challenge octet makes the authenticator refuse it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestMSCHAPv2PeerChallengeRandom16`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L330) | unit/verify | revert, verified |
| positive | [`TestRFC2759PeerChallengeEntersTheNTResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L191) | unit/verify | revert, verified |
| positive | [`TestRFC2759PeerChallengeComesFromCryptoRand`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_peer_challenge_test.go#L37) | unit/verify | revert, verified |

### [`RFC2759-x-10`](#rfc2759-x-10)

0-to-256-unicode-char Password: 63 00 6C 00 69 00 65 00 6E 00 74 00 50 00 61 00 73 00 73 00 (§9.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: ntPasswordHash("clientPass") equals the Section 9.2 PasswordHash vector and the MD4 of the exact 9.2 unicode octets. Negative: ntPasswordHash("é") is the MD4 of E9 00, not of the UTF-8 C3 A9, exercising the producer. The 0-to-256-unicode-char label is a parameter type with no behaviour to assert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759PasswordHashVector`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L176) | unit/verify | revert, verified |
| negative | [`TestNtPasswordHash`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC2759PasswordHashVector`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestNtPasswordHash`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_mschapv2_test.go#L29) | unit/verify | revert, verified |

### [`RFC2759-x-12`](#rfc2759-x-12)

Peer Response/Challenge -> <- Failure (E=691 R=0) (Authenticator disconnects) (§9.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: a wrong password draws an MS-CHAPv2 Failure whose E= is 691 and R= is 0 (both fields read), and the peer's answer draws EAP-Failure (the disconnect). Negative: a verified NT-Response ends in EAP-Success, so the Failure is the verdict, not a constant.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2759FailureInvitesNoRetry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L255) | unit/verify | revert, verified |
| negative | [`TestRFC2759AuthenticatorAcceptsMatchingNTResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L171) | unit/verify | revert, verified |
| positive | [`TestRFC2759FailureInvitesNoRetry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_clause_test.go#L238) | unit/verify | revert, verified |
| positive | [`TestRFC2759AuthenticatorRefusesWithErrorCode691`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc2759_failure_packet_test.go#L104) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement walk agent, spec-rfcgate-6-supported-extraction-signoff phase 2 (Tier 1) |
| Signed off | 2026-08-30 |
| Register | prose |
| Source | rfc/full/rfc2759.txt |
| Source fingerprint | 0b123d1db323bc28 |
| Record | rfc/extraction/rfc2759.json |
| Mapped sentences | 5 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Copyright Notice, Abstract and Table of Contents. The Abstract names the document as a description of MS-CHAP-V2 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. A bulleted list of the six differences from MS-CHAP-V1: CHAP Algorithm 0x81, mutual authentication by piggybacking a peer challenge and an authenticator response, the changed NT-Response calculation, the Peer-Challenge replacing the LAN Manager response, the changed Failure Message format, and the single Change-Password packet. Every sentence is indicative, and each obligation it previews is stated normatively in sections 3 to 7. |
| `2` | LCP Configuration | 0 | walked | LCP Configuration. Assigns the CHAP Algorithm field the value 0x81 and observes that a PPP implementation which answers with LCP Config-Rej has no problem. A value assignment is not a directive, which is the reading the RFC 4486 sign-off took of its own subcode table. The obligation to USE the value would belong to a PPP speaker, and the ledger scopes this summary to MS-CHAPv2 carried in EAP inside IKEv2, where no LCP option is negotiated at all. |
| `3` | Challenge Packet | 0 | walked | Challenge Packet. Two sentences carry content: 'MS-CHAP-V2 authenticators send an 16-octet challenge Value field', which is indicative, and 'the standard guidelines on randomness [1,2,7] SHOULD be observed', which is advisory. The site scan reads no site from either, and RFC2759-x-8 is read from the pair. |
| `4` | Response Packet | 2 | walked | Response Packet. The Value sub-format list is site 4:1 and the Flag rule is site 4:2. Three further ids are read from prose here. RFC2759-x-3 states the Value-Size as 49, which the RFC never writes as a number: it is the sum of the sub-format list, 16 + 8 + 24 + 1. RFC2759-x-9 is read from 'The Peer-Challenge field is a 16-octet random number' together with the same randomness advisory as section 3. RFC2759-x-11 is the receiver-side counterpart of the two sender rules the sites carry: the RFC states that Reserved and Flags must be zero and never states the duty to reject a packet in which they are not. The domain-stripping rule is stated here too, in 'only the user name is used, without any associated Windows NT domain name'; it is attributed to section 8.2, where the pseudocode states it as a constraint on the hash input. |
| `5` | Success Packet | 3 | walked | Success Packet. Three sites: the uppercase-hex rule (5:1), the peer's duty to verify the authenticator response (5:2), and the peer's duty to end the session when that response is missing or incorrect (5:3). The summary carries one row for the verify-and-disconnect duty, RFC2759-x-7, so site 5:3 maps it and site 5:2 is excluded as its duplicate. The method the section points at is section 8.8. |
| `6` | Failure Packet | 1 | walked | Failure Packet. The C= field rule is site 6:1. The error-code table, the retry flag and the <msg> text are indicative. One lowercase advisory sits here that the summary does not declare, 'implementations should deal with codes not on this list gracefully', and it binds a reader of a Failure packet; ze's IKEv2 EAP path never parses one. RFC2759-x-12 is read from the 691 ERROR_AUTHENTICATION_FAILURE entry together with the flow in section 9.1.3, and RFC2759-x-13 from 'For MS-CHAP-V2, this value SHOULD always be 3'. |
| `7` | Change-Password Packet | 1 | walked | Change-Password Packet. The packet is 586 octets and is sent by a peer whose password the authenticator reported expired. Its Reserved field rule is site 7:1, excluded below: ze implements the password-change exchange in neither direction. The Peer-Challenge and NT-Response fields are defined by reference to the Response packet and add no obligation of their own, and the Flags field is 'Reserved, always clear (0)', an indicative bit-field description. |
| `8` | Pseudocode | 0 | walked | Pseudocode. One sentence naming what the subsections describe. No obligation. |
| `8.1` | GenerateNTResponse() | 0 | walked | GenerateNTResponse(). Pseudocode composing ChallengeHash, NtPasswordHash and ChallengeResponse into the 24-octet NT-Response. Indicative throughout. GenerateNTResponse (internal/core/eap/mschapv2.go) is the same composition in the same order. |
| `8.2` | ChallengeHash() | 0 | walked | ChallengeHash(). SHA-1 over PeerChallenge, AuthenticatorChallenge and UserName, truncated to 8 octets. Its comment states the constraint RFC2759-x-4 renders: 'Only the user name (as presented by the peer and excluding any prepended domain name) is used as input to SHAUpdate()'. A pseudocode comment carries no capitalised keyword, so the scan reads no site from it. |
| `8.3` | NtPasswordHash() | 0 | walked | NtPasswordHash(). MD4 over the password, with the comment 'Only the password is hashed without including any terminating 0'. RFC2759-x-4's sibling RFC2759-x-10 is read here: the UTF-16LE encoding is stated by the declared input type '0-to-256-unicode-char Password' and pinned by the worked vector in section 9.2, where the password 'clientPass' appears as 63 00 6C 00 69 00 ... with no terminator. Neither statement is a keyword site. |
| `8.4` | HashNtPasswordHash() | 0 | walked | HashNtPasswordHash(). MD4 over the 16-octet PasswordHash. Indicative. |
| `8.5` | ChallengeResponse() | 0 | walked | ChallengeResponse(). Zero-pads the PasswordHash to 21 octets, splits it into three 7-octet DES keys and encrypts the 8-octet Challenge under each. Indicative; the summary renders it in its Crypto Operations table. |
| `8.6` | DesEncrypt() | 0 | walked | DesEncrypt(). DES in ECB mode, with the note that the caller inserts the parity bits itself because the algorithm ignores them. Indicative. |
| `8.7` | GenerateAuthenticatorResponse() | 0 | walked | GenerateAuthenticatorResponse(). The Magic1 and Magic2 constants and the two SHA-1 passes that produce the 20-octet value the S= field carries. Indicative. GenerateAuthenticatorResponse (internal/core/eap/mschapv2.go) implements it and the authenticator calls it in handleResponse. |
| `8.8` | CheckAuthenticatorResponse() | 0 | walked | CheckAuthenticatorResponse(). The procedure section 5 points at: recompute the authenticator response from the peer's own inputs and compare. Pseudocode, so no site; the obligation to RUN it is stated in section 5 and mapped there as RFC2759-x-7. Ze has no implementation of this routine: the peer's handleMSCHAPv2Success (internal/core/eap/peer.go) hex-decodes the S= field and never calls GenerateAuthenticatorResponse. |
| `8.9` | NewPasswordEncryptedWithOldNtPasswordHash() | 0 | walked | NewPasswordEncryptedWithOldNtPasswordHash(). Change-Password crypto, building the PWBLOCK. Indicative pseudocode for the exchange section 7 defines and ze does not implement. |
| `8.10` | EncryptPwBlockWithPasswordHash() | 0 | walked | EncryptPwBlockWithPasswordHash(). Change-Password crypto. Indicative pseudocode for the exchange ze does not implement. |
| `8.11` | Rc4Encrypt() | 0 | walked | Rc4Encrypt(). Change-Password crypto, naming RC4 as a licensed proprietary algorithm. Indicative pseudocode for the exchange ze does not implement. |
| `8.12` | OldNtPasswordHashEncryptedWithNewNtPasswordHash() | 0 | walked | OldNtPasswordHashEncryptedWithNewNtPasswordHash(). Change-Password crypto. Indicative pseudocode for the exchange ze does not implement. |
| `8.13` | NtPasswordHashEncryptedWithBlock() | 0 | walked | NtPasswordHashEncryptedWithBlock(). Change-Password crypto, two DES blocks over the password hash. Indicative pseudocode for the exchange ze does not implement. |
| `9` | Examples | 0 | walked | Examples. One sentence naming what the subsections show. No obligation. |
| `9.1` | Negotiation Examples | 0 | walked | Negotiation Examples. States indicatively that the packet sequence ID increments on each retry response and on the change-password response, that retry is never allowed after a password change, and that a password change may follow a retry. Descriptions of the flows below, not directives. |
| `9.1.1` | Successful authentication | 0 | walked | Successful authentication. A three-message flow diagram. Non-normative. |
| `9.1.2` | Authenticator authentication failure | 0 | walked | Authenticator authentication failure. The flow diagram for the case RFC2759-x-7 governs: 'Authenticator Response verification fails, peer disconnects'. A worked example of the section 5 obligation, not a second statement of it. |
| `9.1.3` | Failed authentication with no retry allowed | 0 | walked | Failed authentication with no retry allowed. The flow diagram showing Failure (E=691 R=0) followed by an authenticator disconnect. RFC2759-x-12 is read from it together with the section 6 error table. |
| `9.1.4` | Successful authentication after retry | 0 | walked | Successful authentication after retry. A flow diagram. Non-normative. |
| `9.1.5` | Failed hack attack with 3 attempts allowed | 0 | walked | Failed hack attack with 3 attempts allowed. A flow diagram illustrating the retry limit that section 10 states as an advisory. Non-normative. |
| `9.1.6` | Successful authentication with password change | 0 | walked | Successful authentication with password change. A flow diagram for the exchange section 7 defines. Non-normative. |
| `9.1.7` | Successful authentication with retry and password change | 0 | walked | Successful authentication with retry and password change. A flow diagram. Non-normative. |
| `9.2` | Hash Example | 0 | walked | Hash Example. The known-answer vectors for user name 'User' and password 'clientPass', from the UTF-16LE password bytes through to 'S=407A5589115FD0D6209F510FE9C04566932CDA56'. No obligation; these are the vectors the RFC2759-x-10 and RFC2759-x-4 tests drive (internal/core/eap/mschapv2_test.go). |
| `9.3` | Example of DES Key Generation | 0 | walked | Example of DES Key Generation. Shows the two parity-corrected DES keys derived from the password 'MyPw', and notes that many DES engines strip the parity bits rather than check them. A worked example of section 8.6, with no obligation of its own. |
| `10` | Security Considerations | 0 | walked | Security Considerations. One sentence, an advisory: 'As an implementation detail, the authenticator SHOULD limit the number of password retries allowed to make brute-force password guessing attacks more difficult.' RFC2759-x-14 renders it. The scan reads no site because the sentence is SHOULD-level. |
| `11` | References | 0 | skipped (references) | References. Twelve citations, including RFC 2119 at [3]. No obligation of this document. |
| `12` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. Credits the reviewers of the document. |
| `13` | Author's Address | 0 | skipped (front-matter) | Author's Address. Document furniture: postal address, telephone number and e-mail address for the author. |
| `14` | Full Copyright Statement | 1 | walked | Full Copyright Statement. Walked rather than skipped because the prose scan attributes its one site here; that site is the Internet Society boilerplate and is excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Success-packet verification duty is stated in two sentences: this one names the act and site 5:3 names the consequence. rfc/short/rfc2759.md carries them as one row, RFC2759-x-7, whose declared text is the consequence sentence and whose annotation covers both halves ('never computes the expected Authenticator Response to compare or disconnect on mismatch'). Site 5:3 maps that row. Ze meets neither half: handleMSCHAPv2Success (internal/core/eap/peer.go) hex-decodes the S= field and never calls GenerateAuthenticatorResponse. Raised as an ask under AC-8 of plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md, not annotated here. | The authenticating peer MUST verify the authenticator response when a Success packet is received. |
| `7:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the peer that performs the MS-CHAPv2 password change, a role ze plays in neither direction. Section 7 makes the role optional in its own text: 'This packet type is supported by recent versions of Windows NT 4.0, Windows 95 and Windows 98. It is not supported by Windows NT 3.5, Windows NT 3.51, or early versions', and the packet 'should be sent only if the authenticator reports ERROR_PASSWD_EXPIRED (E=648)'. Ze never reports E=648, because sendFailure (internal/core/eap/eap_mschapv2.go) ends the method with ErrMethodFailed and builds no MS-CHAPv2 Failure packet. Ze never sends or accepts Code 7 either: the authenticator's Process and the peer's handleMSCHAPv2Request (internal/core/eap/peer.go) each switch on Challenge, Response and Success alone and refuse every other opcode, and the PPP path states the same scope at internal/component/l2tp/ppp/mschapv2.go:25. Nothing in ze builds or parses the packet whose Reserved field this sentence constrains. | Reserved 8 octets, must be zero. |
| `14:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the site scan does not strip: the Internet Society Full Copyright Statement. Its 'must be followed' governs whoever republishes or translates the document under the Internet Standards process, not a speaker of MS-CHAPv2. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2759, so its obligations are stated where they were written.
