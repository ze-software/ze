# RFC 5709 - OSPFv2 HMAC-SHA Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5709, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 86.7% | 13 of 15 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 13.3% | 2 of 15 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 15 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 28 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 15 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 15 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 15 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 15 |
| Obligations that bind Ze | 15 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 28 |
| Tagged units | 28 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5709.md` |
| Requirement shard | `rfc/requirements/rfc5709.md` |
| RFC text | `rfc/full/rfc5709.txt` |

## Enrolment

Enrolled: OSPFv2 HMAC-SHA Cryptographic Authentication (RFC 5709, AuType 2): 13 MET (HMAC-SHA-256, AuType=2, Auth-Data-Length, crypto sequence, trailer digest, Apad fill, Ko derivation, Ipad/Opad, checksum-zero, receive recompute, Key-ID selection, key rollover) + 2 single-polarity positive (per-KeyID algorithm config, no revert-to-unauthenticated on expiry). Shares RFC 7474 Sign/Verify backend

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
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **15** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC5709-3-1`](#rfc5709-3-1), [`RFC5709-3.1-1`](#rfc5709-3.1-1), [`RFC5709-3.1-2`](#rfc5709-3.1-2), [`RFC5709-3.1-3`](#rfc5709-3.1-3), [`RFC5709-3.1-4`](#rfc5709-3.1-4), [`RFC5709-3.3-1`](#rfc5709-3.3-1), [`RFC5709-3.3-2`](#rfc5709-3.3-2), [`RFC5709-3.3-3`](#rfc5709-3.3-3), [`RFC5709-3.3-4`](#rfc5709-3.3-4), [`RFC5709-3.3-5`](#rfc5709-3.3-5), [`RFC5709-3.4-1`](#rfc5709-3.4-1), [`RFC5709-3.4-2`](#rfc5709-3.4-2), [`RFC5709-3.2-1`](#rfc5709-3.2-1)

**Annotated instead of tested (2):** [`RFC5709-3-5`](#rfc5709-3-5), [`RFC5709-3.2-2`](#rfc5709-3.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5709-3-1` | Implement HMAC-SHA-256 for OSPFv2 Cryptographic Authentication (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L72). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82) |
| `RFC5709-3-2` | Implement HMAC-SHA-1 (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-3` | Implement Keyed-MD5 for backwards compatibility with RFC 2328 deployments (Section 3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-4` | Implement HMAC-SHA-384 and HMAC-SHA-512 (Section 3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5709-3-5` | Allow operators to configure any supported algorithm for any given Key ID value (Section 3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77). **negative:** no negative test. **{single-polarity}:** keyConfig binds KeyID and Algorithm independently and resolveChainKeys/signKey honor each per-key algorithm with no fixed algorithm-per-KeyID mapping, so this is a permissive config capability with no forbidden (supported-algorithm, KeyID) pairing to reject (internal/plugins/ospf/auth_keystore.go:239-253, :292-324) |
| `RFC5709-3.1-1` | Set AuType to 2 (Cryptographic Authentication) for SHA/HMAC-authenticated packets (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L56). **negative:** `unit/verify` [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L68) |
| `RFC5709-3.1-2` | Set the Authentication Data Length field to the hash length in bytes (20/32/48/64) (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L60). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L224) |
| `RFC5709-3.1-3` | Set the 32-bit Cryptographic Sequence Number per RFC 2328 Appendix D (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L62). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L135) |
| `RFC5709-3.1-4` | Append the computed digest after the OSPF packet (Authentication Trailer), not inside the 8-byte auth field (Section 3.1, Section 3.3) | MUST | 3.1 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L67). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L225) |
| `RFC5709-3.3-1` | Fill the Authentication Trailer with Apad (0x878FE1F3 repeated L/4 times) before computing the hash (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L73). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L91) |
| `RFC5709-3.3-2` | Derive Ko to length L: Ko = K, H(K), or K zero-padded to L (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L74). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L83) |
| `RFC5709-3.3-3` | Compute First-Hash = H(Ko XOR Ipad \|\| OSPFv2 Packet) and Second-Hash = H(Ko XOR Opad \|\| First-Hash) (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L75). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L84) |
| `RFC5709-3.3-4` | Place Second-Hash as the Authentication Data of length L in the trailer (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L69). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L92) |
| `RFC5709-3.3-5` | Set the OSPF header Checksum field to 0 for AuType 2 packets, per RFC 2328 D.4.3 (Section 3.3, RFC 2328 D.4.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L65). **negative:** `unit/verify` [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L354) |
| `RFC5709-3.4-1` | On receive, save the wire digest, replace the trailer with Apad, recompute, and compare (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L76). **negative:** `unit/verify` [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L93) |
| `RFC5709-3.4-2` | Select algorithm/key on receive implicitly from the packet's Key ID (Section 3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L245). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L242) |
| `RFC5709-3.2-1` | Ensure a new key's KeyStartGenerate <= the old key's KeyStopGenerate on rollover (Section 3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L558). **negative:** `unit/verify` [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L571) |
| `RFC5709-3.2-2` | Revert to an unauthenticated condition when the last key expires (Section 3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L296). **negative:** no negative test. **{single-polarity}:** selectSendKey returns the most-recently-starting key when every send-lifetime has expired and signKey never yields AuTypeNull for a resolved chain, so the forbidden revert-to-unauthenticated transition is structurally absent and there is no packet-reject direction (internal/plugins/ospf/auth_keystore.go:263-287) |
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
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L82) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L72) | unit/verify | unproven |

### [`RFC5709-3-5`](#rfc5709-3-5)

Allow operators to configure any supported algorithm for any given Key ID value (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L77) | unit/verify | unproven |

### [`RFC5709-3.1-1`](#rfc5709-3.1-1)

Set AuType to 2 (Cryptographic Authentication) for SHA/HMAC-authenticated packets (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthStoreSignVerify`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L68) | unit/verify | unproven |
| positive | [`TestEngineSignPacketCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_wiring_test.go#L56) | unit/verify | unproven |

### [`RFC5709-3.1-2`](#rfc5709-3.1-2)

Set the Authentication Data Length field to the hash length in bytes (20/32/48/64) (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L224) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L60) | unit/verify | unproven |

### [`RFC5709-3.1-3`](#rfc5709-3.1-3)

Set the 32-bit Cryptographic Sequence Number per RFC 2328 Appendix D (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L135) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L62) | unit/verify | unproven |

### [`RFC5709-3.1-4`](#rfc5709-3.1-4)

Append the computed digest after the OSPF packet (Authentication Trailer), not inside the 8-byte auth field (Section 3.1, Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L225) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L67) | unit/verify | unproven |

### [`RFC5709-3.3-1`](#rfc5709-3.3-1)

Fill the Authentication Trailer with Apad (0x878FE1F3 repeated L/4 times) before computing the hash (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L91) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L73) | unit/verify | unproven |

### [`RFC5709-3.3-2`](#rfc5709-3.3-2)

Derive Ko to length L: Ko = K, H(K), or K zero-padded to L (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L83) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L74) | unit/verify | unproven |

### [`RFC5709-3.3-3`](#rfc5709-3.3-3)

Compute First-Hash = H(Ko XOR Ipad || OSPFv2 Packet) and Second-Hash = H(Ko XOR Opad || First-Hash) (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L84) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L75) | unit/verify | unproven |

### [`RFC5709-3.3-4`](#rfc5709-3.3-4)

Place Second-Hash as the Authentication Data of length L in the trailer (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L92) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L69) | unit/verify | unproven |

### [`RFC5709-3.3-5`](#rfc5709-3.3-5)

Set the OSPF header Checksum field to 0 for AuType 2 packets, per RFC 2328 D.4.3 (Section 3.3, RFC 2328 D.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoChecksumOctetAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L354) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L65) | unit/verify | unproven |

### [`RFC5709-3.4-1`](#rfc5709-3.4-1)

On receive, save the wire digest, replace the trailer with Apad, recompute, and compare (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L93) | unit/verify | unproven |
| positive | [`TestOSPFAuthSignVerifyCrypto`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L76) | unit/verify | unproven |

### [`RFC5709-3.4-2`](#rfc5709-3.4-2)

Select algorithm/key on receive implicitly from the packet's Key ID (Section 3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L242) | unit/verify | unproven |
| positive | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L245) | unit/verify | unproven |

### [`RFC5709-3.2-1`](#rfc5709-3.2-1)

Ensure a new key's KeyStartGenerate <= the old key's KeyStopGenerate on rollover (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyRolloverGapRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L571) | unit/verify | unproven |
| positive | [`TestKeyRolloverOverlapAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/config_test.go#L558) | unit/verify | unproven |

### [`RFC5709-3.2-2`](#rfc5709-3.2-2)

Revert to an unauthenticated condition when the last key expires (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSignKeyNoRevertWhenAllExpired`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L296) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5709, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5709, so its obligations are stated where they were written.
