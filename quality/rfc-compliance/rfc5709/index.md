# RFC 5709 - OSPFv2 HMAC-SHA Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5709, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 85.7% | 12 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.3% | 2 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 38 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 19 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 38 |
| Tagged units | 38 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Positive and negative tests | 12 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC5709-3-1`](#rfc5709-3-1), [`RFC5709-3.1-1`](#rfc5709-3.1-1), [`RFC5709-3.1-2`](#rfc5709-3.1-2), [`RFC5709-3.1-3`](#rfc5709-3.1-3), [`RFC5709-3.1-4`](#rfc5709-3.1-4), [`RFC5709-3.3-1`](#rfc5709-3.3-1), [`RFC5709-3.3-2`](#rfc5709-3.3-2), [`RFC5709-3.3-3`](#rfc5709-3.3-3), [`RFC5709-3.3-4`](#rfc5709-3.3-4), [`RFC5709-3.4-1`](#rfc5709-3.4-1), [`RFC5709-3.4-2`](#rfc5709-3.4-2), [`RFC5709-3.2-1`](#rfc5709-3.2-1)

**Annotated instead of tested (2):** [`RFC5709-3-5`](#rfc5709-3-5), [`RFC5709-3.2-2`](#rfc5709-3.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5709-3-1` | Implement HMAC-SHA-256 for OSPFv2 Cryptographic Authentication (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L87) |
| `RFC5709-3-2` | Implement HMAC-SHA-1 (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-3` | Implement Keyed-MD5 for backwards compatibility with RFC 2328 deployments (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-4` | Implement HMAC-SHA-384 and HMAC-SHA-512 (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-5` | An implementation of this specification MUST allow network operators to configure ANY authentication algorithm supported by that implementation for use with ANY given KeyID value that is configured into that OSPFv2 router. (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82). **negative:** no negative test. **{single-polarity}:** keyConfig binds KeyID and Algorithm independently and resolveChainKeys/signKey honor each per-key algorithm with no fixed algorithm-per-KeyID mapping, so this is a permissive config capability with no forbidden (supported-algorithm, KeyID) pairing to reject (internal/plugins/ospf/auth_keystore.go:239-253, :292-324) |
| `RFC5709-3.1-1` | Set AuType to 2 (Cryptographic Authentication) for SHA/HMAC-authenticated packets (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L56). **negative:** `unit/verify` [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L73) |
| `RFC5709-3.1-2` | Set the Authentication Data Length field to the hash length in bytes (20/32/48/64) (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L61). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L229) |
| `RFC5709-3.1-3` | Set the 32-bit Cryptographic Sequence Number per RFC 2328 Appendix D (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L63). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L140) |
| `RFC5709-3.1-4` | Append the computed digest after the OSPF packet (Authentication Trailer), not inside the 8-byte auth field (Section 3.1, Section 3.3) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L68). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L230) |
| `RFC5709-3.3-1` | Fill the Authentication Trailer with Apad (0x878FE1F3 repeated L/4 times) before computing the hash (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L78). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L96). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L431) |
| `RFC5709-3.3-2` | Derive Ko to length L: Ko = K, H(K), or K zero-padded to L (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L79). **positive:** `unit/verify` [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L398). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L88). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L432) |
| `RFC5709-3.3-3` | Compute First-Hash = H(Ko XOR Ipad \|\| OSPFv2 Packet) and Second-Hash = H(Ko XOR Opad \|\| First-Hash) (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L80). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L89). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L433) |
| `RFC5709-3.3-4` | Place Second-Hash as the Authentication Data of length L in the trailer (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L70). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L97). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L434) |
| `RFC5709-3.4-1` | On receive, save the wire digest, replace the trailer with Apad, recompute, and compare (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L137). **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L81). **positive:** `unit/verify` [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L399). **negative:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L138). **negative:** `unit/verify` [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L359). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L98). **negative:** `unit/verify` [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L435) |
| `RFC5709-3.4-2` | Select algorithm/key on receive implicitly from the packet's Key ID (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L135). **positive:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L250). **negative:** `unit/verify` [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L136). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L247) |
| `RFC5709-3.2-1` | When a new key replaces an old, the KeyStartGenerate time for the new key MUST be less than or equal to the KeyStopGenerate time of the old key. (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L559). **negative:** `unit/verify` [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L572) |
| `RFC5709-3.2-2` | Revert to an unauthenticated condition when the last key expires (Section 3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L301). **negative:** no negative test. **{single-polarity}:** selectSendKey returns the most-recently-starting key when every send-lifetime has expired and signKey never yields AuTypeNull for a resolved chain, so the forbidden revert-to-unauthenticated transition is structurally absent and there is no packet-reject direction (internal/plugins/ospf/auth_keystore.go:263-287) |
| `RFC5709-3.2-3` | Set KeyStartAccept < KeyStartGenerate and KeyStopGenerate < KeyStopAccept (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3.2-4` | Never send the Authentication Key or Algorithm over the wire in cleartext; persist key storage across restart (Section 3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5709 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5709-3-1`](#rfc5709-3-1)

Implement HMAC-SHA-256 for OSPFv2 Cryptographic Authentication (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L87) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77) | unit/verify | unproven |

### [`RFC5709-3-5`](#rfc5709-3-5)

An implementation of this specification MUST allow network operators to configure ANY authentication algorithm supported by that implementation for use with ANY given KeyID value that is configured into that OSPFv2 router. (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82) | unit/verify | unproven |

### [`RFC5709-3.1-1`](#rfc5709-3.1-1)

Set AuType to 2 (Cryptographic Authentication) for SHA/HMAC-authenticated packets (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L73) | unit/verify | unproven |
| positive | [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L56) | unit/verify | unproven |

### [`RFC5709-3.1-2`](#rfc5709-3.1-2)

Set the Authentication Data Length field to the hash length in bytes (20/32/48/64) (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L229) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L61) | unit/verify | unproven |

### [`RFC5709-3.1-3`](#rfc5709-3.1-3)

Set the 32-bit Cryptographic Sequence Number per RFC 2328 Appendix D (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L140) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L63) | unit/verify | unproven |

### [`RFC5709-3.1-4`](#rfc5709-3.1-4)

Append the computed digest after the OSPF packet (Authentication Trailer), not inside the 8-byte auth field (Section 3.1, Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L230) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L68) | unit/verify | unproven |

### [`RFC5709-3.3-1`](#rfc5709-3.3-1)

Fill the Authentication Trailer with Apad (0x878FE1F3 repeated L/4 times) before computing the hash (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L96) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L431) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L78) | unit/verify | unproven |

### [`RFC5709-3.3-2`](#rfc5709-3.3-2)

Derive Ko to length L: Ko = K, H(K), or K zero-padded to L (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L88) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L432) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L79) | unit/verify | unproven |
| positive | [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L398) | unit/verify | unproven |

### [`RFC5709-3.3-3`](#rfc5709-3.3-3)

Compute First-Hash = H(Ko XOR Ipad || OSPFv2 Packet) and Second-Hash = H(Ko XOR Opad || First-Hash) (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L89) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L433) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L80) | unit/verify | unproven |

### [`RFC5709-3.3-4`](#rfc5709-3.3-4)

Place Second-Hash as the Authentication Data of length L in the trailer (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L97) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L434) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L70) | unit/verify | unproven |

### [`RFC5709-3.4-1`](#rfc5709-3.4-1)

On receive, save the wire digest, replace the trailer with Apad, recompute, and compare (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L138) | unit/verify | unproven |
| negative | [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L359) | unit/verify | unproven |
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L98) | unit/verify | unproven |
| negative | [`TestRFC5709ReceiveRejectsWrongHashConstruction`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L435) | unit/verify | unproven |
| positive | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L137) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L81) | unit/verify | unproven |
| positive | [`TestRFC5709ReceiveIndependentDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L399) | unit/verify | unproven |

### [`RFC5709-3.4-2`](#rfc5709-3.4-2)

Select algorithm/key on receive implicitly from the packet's Key ID (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L136) | unit/verify | unproven |
| negative | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L247) | unit/verify | unproven |
| positive | [`TestEngineReceiveSelectsSecurityAssociation`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L135) | unit/verify | unproven |
| positive | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L250) | unit/verify | unproven |

### [`RFC5709-3.2-1`](#rfc5709-3.2-1)

When a new key replaces an old, the KeyStartGenerate time for the new key MUST be less than or equal to the KeyStopGenerate time of the old key. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L572) | unit/verify | unproven |
| positive | [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L559) | unit/verify | unproven |

### [`RFC5709-3.2-2`](#rfc5709-3.2-2)

Revert to an unauthenticated condition when the last key expires (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L301) | unit/verify | unproven |

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
