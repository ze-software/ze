# RFC 5709 - OSPFv2 HMAC-SHA Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5709, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 92.9% | 13 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.1% | 1 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 21.7% | 10 of 46 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 15 | of 14 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 46 |
| Tagged units | 46 |
| Recorded audit verdicts | 15 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc5709.md` |
| Requirement shard | `rfc/requirements/rfc5709.md` |
| RFC text | `rfc/full/rfc5709.txt` |

## Enrolment

Enrolled: OSPFv2 HMAC-SHA Cryptographic Authentication (AuType 2), including key selection, digest construction and receive verification. RFC 7474 uses the same Sign/Verify backend with extended sequence numbers, source-bound Apad and a protocol-ID key suffix.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

OSPFv2 cryptographic authentication path.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC5709-3-1`](#rfc5709-3-1), [`RFC5709-3.1-1`](#rfc5709-3.1-1), [`RFC5709-3.1-2`](#rfc5709-3.1-2), [`RFC5709-3.1-3`](#rfc5709-3.1-3), [`RFC5709-3.1-4`](#rfc5709-3.1-4), [`RFC5709-3.3-1`](#rfc5709-3.3-1), [`RFC5709-3.3-2`](#rfc5709-3.3-2), [`RFC5709-3.3-3`](#rfc5709-3.3-3), [`RFC5709-3.3-4`](#rfc5709-3.3-4), [`RFC5709-3.4-1`](#rfc5709-3.4-1), [`RFC5709-3.4-2`](#rfc5709-3.4-2), [`RFC5709-3.2-1`](#rfc5709-3.2-1), [`RFC5709-3.2-2`](#rfc5709-3.2-2)

**Annotated (including scoped evidence) (1):** [`RFC5709-3-5`](#rfc5709-3-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5709-3-1` | Of the above, implementations of this specification MUST include support for at least: HMAC-SHA-256 (§3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L87) |
| `RFC5709-3-2` | and SHOULD include support for: HMAC-SHA-1 (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-3` | and SHOULD also (for backwards compatibility with existing implementations and deployments) include support for: Keyed-MD5 (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-4` | and MAY also include support for: HMAC-SHA-384 HMAC-SHA-512 (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-5` | An implementation of this specification MUST allow network operators to configure ANY authentication algorithm supported by that implementation for use with ANY given KeyID value that is configured into that OSPFv2 router. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82). **positive:** `unit/verify` [`TestRFC5709AnyAlgorithmUnderAnyKeyID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_any_algorithm_any_keyid_test.go#L36). **negative:** no negative test. **{single-polarity}:** keyConfig binds KeyID and Algorithm independently and resolveChainKeys/signKey honor each per-key algorithm with no fixed algorithm-per-KeyID mapping, so this is a permissive config capability with no forbidden (supported-algorithm, KeyID) pairing to reject (internal/plugins/ospf/auth_keystore.go:239-253, :292-324) |
| `RFC5709-3.1-1` | Second, set the Authentication Type to Cryptographic Authentication (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L56). **negative:** `unit/verify` [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L73) |
| `RFC5709-3.1-2` | set the Authentication Data Length field to the length (measured in bytes, not bits) of the cryptographic hash that will be used. When any NIST SHS algorithm is used in HMAC mode with OSPFv2 Cryptographic Authentication, the Authentication Data Length is equal to the normal hash output length (measured in bytes) for the specific NIST SHS algorithm in use. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L61). **positive:** `unit/verify` [`TestRFC5709AuthDataLengthIsHashOutputLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L21). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L229). **negative:** `unit/verify` [`TestRFC5709WrongAuthDataLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L74) |
| `RFC5709-3.1-3` | Third, the 32-bit cryptographic sequence number is set in accordance with the procedures in RFC 2328, Appendix D that are applicable to the Cryptographic Authentication type. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L63). **positive:** `unit/verify` [`TestRFC5709CryptoSequenceNonDecreasingOnInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_crypto_sequence_test.go#L50). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L140) |
| `RFC5709-3.1-4` | Fourth, the message digest is then calculated and appended to the OSPF packet, as described below in Section 3.3. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L68). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L230) |
| `RFC5709-3.3-1` | First, the OSPFv2 packet's Authentication Trailer (which is the appendage described in RFC 2328, Section D.4.3, Page 233, items (6)(a) and (6)(d)) is filled with the value Apad, and the Authentication Type field is set to 2. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L78). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L96). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L431) |
| `RFC5709-3.3-2` | In this application, Ko is always L octets long. If the Authentication Key (K) is L octets long, then Ko is equal to K. If the Authentication Key (K) is more than L octets long, then Ko is set to H(K). If the Authentication Key (K) is less than L octets long, then Ko is set to the Authentication Key (K) with zeros appended to the end of the Authentication Key (K), such that Ko is L octets long. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L79). **positive:** `unit/verify` [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L398). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L88). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L432) |
| `RFC5709-3.3-3` | Then, a First-Hash, also known as the inner hash, is computed as follows: First-Hash = H(Ko XOR Ipad \|\| (OSPFv2 Packet)) Implementation Notes: Note that the First-Hash above includes the Authentication Trailer containing the Apad value, as well as the OSPF packet, as per RFC 2328, Section D.4.3. The definition of Apad (above) ensures it is always the same length as the hash output. This is consistent with RFC 2328. The "(OSPFv2 Packet)" mentioned in the First-Hash (above) does include the OSPF Authentication Trailer. The digest length for SHA-1 is 20 bytes; for SHA-256, 32 bytes; for SHA-384, 48 bytes; and for SHA-512, 64 bytes. (3) SECOND-HASH Then a Second-Hash, also known as the outer hash, is computed as follows: Second-Hash = H(Ko XOR Opad \|\| First-Hash) (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L80). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L89). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L433) |
| `RFC5709-3.3-4` | The resulting Second-Hash becomes the Authentication Data that is sent in the Authentication Trailer of the OSPFv2 packet. The length of the Authentication Trailer is always identical to the message digest size of the specific hash function H that is being used. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L70). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L97). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L434) |
| `RFC5709-3.4-1` | the cryptographic calculation of the message digest follows the procedure in Section 3.3 above when any NIST SHS algorithm in the HMAC mode is in use. Kindly recall that the cryptographic algorithm/mode in use is indicated implicitly by the KeyID of the received OSPFv2 packet. Implementation Notes: One must save the received digest value before calculating the expected digest value, so that after that calculation the received value can be compared with the expected value to determine whether to accept that OSPF packet. RFC 2328, Section D.4.3 (6) (c) should be read very closely prior to implementing the above. With SHA algorithms in HMAC mode, Apad is placed where the MD5 key would be put if Keyed-MD5 were in use. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L137). **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L81). **positive:** `unit/verify` [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L399). **negative:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L138). **negative:** `unit/verify` [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L359). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L98). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L435) |
| `RFC5709-3.4-2` | Kindly recall that the cryptographic algorithm/mode in use is indicated implicitly by the KeyID of the received OSPFv2 packet. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L135). **positive:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L250). **negative:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L136). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L247) |
| `RFC5709-3.2-1` | When a new key replaces an old, the KeyStartGenerate time for the new key MUST be less than or equal to the KeyStopGenerate time of the old key. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_config_test.go#L561). **negative:** `unit/verify` [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_config_test.go#L574) |
| `RFC5709-3.2-2` | In the event that the last key associated with an interface expires, it is unacceptable to revert to an unauthenticated condition (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC5709LastKeyExpiredStillSigns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L48). **positive:** `unit/verify` [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L344). **negative:** `unit/verify` [`TestRFC5709LastKeyExpiredRefusesUnauthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L70) |
| `RFC5709-3.2-5` | Therefore, the router should send a "last Authentication Key expiration" notification to the network manager and treat the key as having an infinite lifetime until the lifetime is extended, the key is deleted by network management, or a new key is configured. (§3.2) | SHOULD | 3.2 | **positive:** `unit/verify` [`TestRFC5709LastKeyExpiredKeepsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L103). **negative:** `unit/verify` [`TestRFC5709ExpiredKeyRefusedOnceNotLast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L150) |
| `RFC5709-3.2-3` | In order to achieve smooth key transition, KeyStartAccept SHOULD be less than KeyStartGenerate and KeyStopGenerate SHOULD be less than KeyStopAccept. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3.2-4` | This information SHOULD never be sent over the wire in cleartext form. At present, valid values are Keyed-MD5, HMAC-SHA-1, HMAC-SHA-256, HMAC-SHA- 384, and HMAC-SHA-512. Authentication Key This is the cryptographic key used for cryptographic authentication with this OSPFv2 SA. This value SHOULD never be sent over the wire in cleartext form. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5709 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5709-3-1`](#rfc5709-3-1)

Of the above, implementations of this specification MUST include support for at least: HMAC-SHA-256 (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: an implementation without a correct HMAC-SHA-256; (b) TestOSPFAuthSignVerifyCrypto/hmac-sha-256 asserts Sign succeeds and the trailer equals rfc5709ReferenceDigest, an independent test-side HMAC-SHA-256 construction, and that Verify accepts it; the negative rejects a wrong-key digest

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L87) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77) | unit/verify | unproven |

### [`RFC5709-3-5`](#rfc5709-3-5)

An implementation of this specification MUST allow network operators to configure ANY authentication algorithm supported by that implementation for use with ANY given KeyID value that is configured into that OSPFv2 router. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5709AnyAlgorithmUnderAnyKeyID drives operator config: five key-chain texts rotate md5 and hmac-sha-1/256/384/512 over Key IDs 1,77,200,254,255 through parseOSPFConfig, validateConfig and the running engine's verifyPacket, auth failures 0. Judge overlay fixing the algorithm per Key ID in resolveChainKeys turns every shift red; Key ID 0 (not sampled) also passes under a judge overlay. {single-polarity: positive} on the row; record observed red on resolveChainKeys.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82) | unit/verify | unproven |
| positive | [`TestRFC5709AnyAlgorithmUnderAnyKeyID`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_any_algorithm_any_keyid_test.go#L36) | unit/verify | revert, verified |

### [`RFC5709-3.1-1`](#rfc5709-3.1-1)

Second, set the Authentication Type to Cryptographic Authentication (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: sending an HMAC-SHA packet whose Authentication Type is not Cryptographic Authentication (2); (b) TestEngineSignPacketCrypto asserts the on-wire AuType of eng.signPacket output equals AuTypeCryptographic; negative TestOSPFAuthStoreSignVerify asserts a non-crypto AuType packet is refused under crypto config

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L73) | unit/verify | unproven |
| positive | [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L56) | unit/verify | unproven |

### [`RFC5709-3.1-2`](#rfc5709-3.1-2)

set the Authentication Data Length field to the length (measured in bytes, not bits) of the cryptographic hash that will be used. When any NIST SHS algorithm is used in HMAC mode with OSPFv2 Cryptographic Authentication, the Authentication Data Length is equal to the normal hash output length (measured in bytes) for the specific NIST SHS algorithm in use. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Positive: TestRFC5709AuthDataLengthIsHashOutputLength signs AuType 2 under HMAC-SHA-1/256/384/512 and asserts af[3] == the literal 20/32/48/64 (not the production table), the trailer is that many octets and equals rfc5709ReferenceDigest (independent Ko, Apad and HMAC); overlay authDigestLen SHA-256 -> 31 reds it (scratch/c23/a1). Negative (owner ruling 2, handling of a peer packet with the wrong field): TestRFC5709WrongAuthDataLengthRejected presents an HMAC-SHA-1 packet with af[3]=32 whose 20-octet digest is computed independently over that very header, so only the length field differs from the verifying control (af[3]=20); Verify refuses it. Overlay removing Verify's int(af[3]) != l guard reds only this unit while TestOSPFAuthSignVerifyCrypto and TestOSPFAuthCryptoRejectsExtraTrailerBytes stay green (a2), closing the earlier finding. Observed-red records on authDigestLen and Verify. HEAD units supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L229) | unit/verify | unproven |
| negative | [`TestRFC5709WrongAuthDataLengthRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L61) | unit/verify | unproven |
| positive | [`TestRFC5709AuthDataLengthIsHashOutputLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L21) | unit/verify | revert, verified |

### [`RFC5709-3.1-3`](#rfc5709-3.1-3)

Third, the 32-bit cryptographic sequence number is set in accordance with the procedures in RFC 2328, Appendix D that are applicable to the Cryptographic Authentication type. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 7, judge): the weak finding (no sender-side sequence assertion) is closed. TestRFC5709CryptoSequenceNonDecreasingOnInterface signs five Hellos through eng.signPacket on eth0 and reloads the key from Key ID 1 to 2 before the fourth; each packet's 32-bit sequence (auth offset 4) is not below the previous one (RFC 2328 D.4.3 (5): non-decreasing), Key ID checked per packet. Positive revert record on signKey re-recorded by the judge after continuation 8 changed signKey, observed red. Negative: TestOSPFAuthReplay-side replayedSequence refusal (revert record observed red). The packet-level round-trip positive in packet/auth_verify_test.go has no record (pre-existing).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC5709CryptoSequenceNonDecreasingOnInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_crypto_sequence_test.go#L50) | unit/verify | revert, verified |

### [`RFC5709-3.1-4`](#rfc5709-3.1-4)

Fourth, the message digest is then calculated and appended to the OSPF packet, as described below in Section 3.3. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a digest not appended after the OSPF packet (e.g. inside the 8-byte auth field or counted in Packet Length); (b) TestOSPFAuthSignVerifyCrypto asserts Packet Length == plen, len(signed) == plen+L and signed[plen:] equals the independent reference digest over signed[:plen]; negative TestOSPFAuthCryptoRejectsExtraTrailerBytes rejects a trailer that is not exactly the appended digest

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L230) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L68) | unit/verify | unproven |

### [`RFC5709-3.3-1`](#rfc5709-3.3-1)

First, the OSPFv2 packet's Authentication Trailer (which is the appendage described in RFC 2328, Section D.4.3, Page 233, items (6)(a) and (6)(d)) is filled with the value Apad, and the Authentication Type field is set to 2. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: hashing with a trailer not filled with Apad, or with the AuType field not 2; (b) TestOSPFAuthSignVerifyCrypto asserts the trailer equals rfc5709ReferenceDigest, which fills literal 0x878FE1F3 x L/4 over the AuType-2 packet; TestRFC5709ReceiveRejectsWrongHashConstruction/zero-apad asserts Verify rejects a zero-filled-trailer digest

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L96) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L431) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L78) | unit/verify | unproven |

### [`RFC5709-3.3-2`](#rfc5709-3.3-2)

In this application, Ko is always L octets long. If the Authentication Key (K) is L octets long, then Ko is equal to K. If the Authentication Key (K) is more than L octets long, then Ko is set to H(K). If the Authentication Key (K) is less than L octets long, then Ko is set to the Authentication Key (K) with zeros appended to the end of the Authentication Key (K), such that Ko is L octets long. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: Ko not equal to K at L, not H(K) above L, not zero-padded below L; (b) TestRFC5709ReceiveIndependentDigest asserts Sign equals an independent construction and Verify accepts for SHA-256 keys of 31, 32 and 33 octets, TestOSPFAuthSignVerifyCrypto covers a 28-octet key hashed for SHA-1; TestRFC5709ReceiveRejectsWrongHashConstruction/underived-long-key asserts the raw 33-octet key (standard HMAC, B=64) is rejected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L88) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L432) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L79) | unit/verify | unproven |
| positive | [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L398) | unit/verify | unproven |

### [`RFC5709-3.3-3`](#rfc5709-3.3-3)

Then, a First-Hash, also known as the inner hash, is computed as follows: First-Hash = H(Ko XOR Ipad || (OSPFv2 Packet)) Implementation Notes: Note that the First-Hash above includes the Authentication Trailer containing the Apad value, as well as the OSPF packet, as per RFC 2328, Section D.4.3. The definition of Apad (above) ensures it is always the same length as the hash output. This is consistent with RFC 2328. The "(OSPFv2 Packet)" mentioned in the First-Hash (above) does include the OSPF Authentication Trailer. The digest length for SHA-1 is 20 bytes; for SHA-256, 32 bytes; for SHA-384, 48 bytes; and for SHA-512, 64 bytes. (3) SECOND-HASH Then a Second-Hash, also known as the outer hash, is computed as follows: Second-Hash = H(Ko XOR Opad || First-Hash) (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a digest other than H(Ko XOR Opad || H(Ko XOR Ipad || packet-with-Apad-trailer)), or of the wrong length; (b) TestOSPFAuthSignVerifyCrypto asserts the trailer equals hmacWithPad (test-side Ipad/Opad construction) for all four SHA algorithms, fixing the 20/32/48/64 lengths; TestRFC5709ReceiveRejectsWrongHashConstruction/inner-hash-only asserts an inner hash alone is rejected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L89) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L433) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L80) | unit/verify | unproven |

### [`RFC5709-3.3-4`](#rfc5709-3.3-4)

The resulting Second-Hash becomes the Authentication Data that is sent in the Authentication Trailer of the OSPFv2 packet. The length of the Authentication Trailer is always identical to the message digest size of the specific hash function H that is being used. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a trailer carrying anything but Second-Hash, or of a length other than the digest size; (b) TestOSPFAuthSignVerifyCrypto asserts len(signed)==plen+L and the trailer equals the reference Second-Hash; TestRFC5709ReceiveRejectsWrongHashConstruction/inner-hash-only asserts a length-L inner hash is rejected, and a flipped trailer byte is rejected

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L97) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L434) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L70) | unit/verify | unproven |

### [`RFC5709-3.4-1`](#rfc5709-3.4-1)

the cryptographic calculation of the message digest follows the procedure in Section 3.3 above when any NIST SHS algorithm in the HMAC mode is in use. Kindly recall that the cryptographic algorithm/mode in use is indicated implicitly by the KeyID of the received OSPFv2 packet. Implementation Notes: One must save the received digest value before calculating the expected digest value, so that after that calculation the received value can be compared with the expected value to determine whether to accept that OSPF packet. RFC 2328, Section D.4.3 (6) (c) should be read very closely prior to implementing the above. With SHA algorithms in HMAC mode, Apad is placed where the MD5 key would be put if Keyed-MD5 were in use. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: verifying without the section 3.3 computation, overwriting the received digest before comparing, not placing Apad in the trailer, or not selecting by KeyID; (b) TestRFC5709ReceiveRejectsWrongHashConstruction asserts zero-Apad, underived-key and inner-only digests are rejected while the valid control is accepted; TestOSPFAuthSignVerifyCrypto rejects a flipped digest byte; TestRFC5709ReceiveIndependentDigest accepts an independent digest and asserts Verify left the wire bytes unchanged; TestEngineReceiveSelectsSecurityAssociation rejects packets outside the KeyID's association and counts them

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L359) | unit/verify | unproven |
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L98) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L435) | unit/verify | unproven |
| negative | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L138) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L81) | unit/verify | unproven |
| positive | [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L399) | unit/verify | unproven |
| positive | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L137) | unit/verify | unproven |

### [`RFC5709-3.4-2`](#rfc5709-3.4-2)

Kindly recall that the cryptographic algorithm/mode in use is indicated implicitly by the KeyID of the received OSPFv2 packet. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: choosing the algorithm or key other than by the received KeyID (trying other associations); (b) TestEngineReceiveSelectsSecurityAssociation asserts a KeyID 8 (SHA-512) packet signed with SHA-256 and a KeyID 9 packet signed with KeyID 7's secret are rejected even though another configured association verifies them, and unknown KeyID 99 is rejected; TestOSPFAuthCryptoRejectsKeyIDMismatch rejects a non-matching KeyID with the same secret; positives accept each KeyID under its own algorithm

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L247) | unit/verify | unproven |
| negative | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L136) | unit/verify | unproven |
| positive | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L250) | unit/verify | unproven |
| positive | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_auth_wiring_test.go#L135) | unit/verify | unproven |

### [`RFC5709-3.2-1`](#rfc5709-3.2-1)

When a new key replaces an old, the KeyStartGenerate time for the new key MUST be less than or equal to the KeyStopGenerate time of the old key. (Section 3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a new key whose KeyStartGenerate is after the old key's KeyStopGenerate; (b) TestKeyRolloverGapRejected asserts validateConfig returns ErrKeyRolloverGap for start 2026-03-01 after end 2026-02-01, and TestKeyRolloverOverlapAccepted asserts an overlapping rollover validates. The equal-time boundary is not exercised

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_config_test.go#L574) | unit/verify | unproven |
| positive | [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_config_test.go#L561) | unit/verify | unproven |

### [`RFC5709-3.2-2`](#rfc5709-3.2-2)

In the event that the last key associated with an interface expires, it is unacceptable to revert to an unauthenticated condition (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 8, judge): row narrowed per R35 to the MUST NOT (no revert to an unauthenticated condition); the 'not advisable to disrupt routing' clause moved to RFC5709-3.2-5. Positive TestRFC5709LastKeyExpiredStillSigns: only key's send and accept lifetimes over, a Hello through eng.signPacket leaves AuType 2 Key ID 1; positive TestSignKeyNoRevertWhenAllExpired (store level). Negative TestRFC5709LastKeyExpiredRefusesUnauthenticated: AuType 0 Hello refused by verify with autype-mismatch after expiry. All three revert records (selectSendKey, verify) re-recorded by the judge after continuation 8 changed those producers, observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5709LastKeyExpiredRefusesUnauthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L344) | unit/verify | revert, verified |
| positive | [`TestRFC5709LastKeyExpiredStillSigns`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L48) | unit/verify | revert, verified |

### [`RFC5709-3.2-5`](#rfc5709-3.2-5)

Therefore, the router should send a "last Authentication Key expiration" notification to the network manager and treat the key as having an infinite lifetime until the lifetime is extended, the key is deleted by network management, or a new key is configured. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. New row 2026-09-30 (R35, D-8 fixed). auth_keystore.go: lastKeyIndex (latest sendStart, tie -> later accept end) is shared by signer and verifier; selectSendKey signs with it when no key is active; lastKeyExtendedIndex extends its accept window only when every window of the chain has closed and the last key's accept end passed (a not-yet-open chain is not expiry); noticeLocked notifies once per interface until configure installs a new chain; notification = slog Warn + ospf/last-key-expiration event. Positive TestRFC5709LastKeyExpiredKeepsAdjacency: two engines with the same expired one-key chain, two Hellos signed by one verify on the other, exactly one event each (eth0, key 1, send / receive). Negative TestRFC5709ExpiredKeyRefusedOnceNotLast: after a live Key ID 2 is configured the expired Key ID 1 is refused accept-lifetime, the signer moves to Key ID 2, no event. Revert records on lastKeyExtendedIndex observed red for both polarities. 'Lifetime extended' and 'key deleted' are the same configure path as 'new key configured' and are not separately asserted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5709ExpiredKeyRefusedOnceNotLast`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestRFC5709LastKeyExpiredKeepsAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5709_last_key_expiry_test.go#L103) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5709.txt |
| Source fingerprint | 5692692e75358feb |
| Record | rfc/extraction/rfc5709.json |
| Mapped sentences | 4 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate from the Copyright Notice, obliging whoever extracts code components from the document to carry the Simplified BSD License text. It is a licensing condition on republication, not a protocol obligation, and it appears before Section 1. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the BSD License. |
| `4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations advice about a different mechanism, stated as a hypothetical: 'If a stronger authentication were believed to be required, then the use of a full digital signature [RFC2154] would be an approach that should be seriously considered.' The 'should' governs how seriously a reader considers RFC 2154 digital signatures, which this document does not specify, so no OSPFv2 behavior answers it. | If a stronger authentication were believed to be required, then the use of a full digital signature [RFC2154] would be an approach that should be seriously considered. |

## Superseded

No document obsoletes RFC 5709, so its obligations are stated where they were written.
