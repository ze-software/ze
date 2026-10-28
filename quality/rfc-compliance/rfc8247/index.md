# RFC 8247 - Algorithm Implementation Requirements and Usage Guidance for the Internet Key Exchange Protocol Version 2 (IKEv2)

Partial. Every requirement this repository extracted from RFC 8247, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 16 of 20 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 20 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 20 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 20 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 32 of 32 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 20 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 20 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 20 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 20 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 20 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 4 of 20 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 20 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 25 |
| Gated MUST-level | 20 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 32 |
| Tagged units | 32 |
| Recorded audit verdicts | 0 |
| Discrimination records | 32 |
| Summary | `rfc/short/rfc8247.md` |
| Requirement shard | `rfc/requirements/rfc8247.md` |
| RFC text | `rfc/full/rfc8247.txt` |

## Enrolment

Enrolled: IKEv2 algorithm requirements: twenty MUST-level rows, every one classified. Three are {gap}: PRF_HMAC_SHA1 (RFC8247-2.2-2) and AUTH_HMAC_SHA1_96 (RFC8247-2.3-2) are MUST- and the IKE SA registries in internal/component/ike/crypto/transform.go admit neither, and RSASSA-PSS (RFC8247-3.2-2) is absent from internal/component/ike/engine/auth.go hashFromAlgID. The others are met by those registries and by auth.go and owe tagged tests. The groups 23 and 24 check is conditional on groups ze does not offer (feature-out-of-scope in the extraction).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- ENCR_AES_CBC, ENCR_AES_GCM_16 and ENCR_AES_CCM for the IKE SA
- PRF and integrity HMAC-SHA2-256/384/512
- DH groups 14, 19 and 20
- PSK, RSA Digital Signature (verify) and Digital Signature (method 14) authentication with SHA2 hashes. No DES, NULL, MD5, DSS or SHA-1 signature algorithm.


**What the ledger says remains**

MUST- algorithms PRF_HMAC_SHA1 and AUTH_HMAC_SHA1_96 are not offered for the IKE SA (AUTH_HMAC_SHA1_96 is ESP-only); RSASSA-PSS is not supported, so RSA signatures use RSASSA-PKCS1-v1.5 only; ze verifies AUTH method 1 but always signs with method 14. ENCR_CHACHA20_POLY1305 is ESP-only. Groups 23 and 24 are not offered. Requirement tests are owed for every met row.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated (including scoped evidence) | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **20** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC8247-2.1-1`](#rfc8247-2.1-1), [`RFC8247-2.1-2`](#rfc8247-2.1-2), [`RFC8247-2.1-3`](#rfc8247-2.1-3), [`RFC8247-2.1-4`](#rfc8247-2.1-4), [`RFC8247-2.2-1`](#rfc8247-2.2-1), [`RFC8247-2.2-3`](#rfc8247-2.2-3), [`RFC8247-2.3-1`](#rfc8247-2.3-1), [`RFC8247-2.3-3`](#rfc8247-2.3-3), [`RFC8247-2.3-4`](#rfc8247-2.3-4), [`RFC8247-2.4-1`](#rfc8247-2.4-1), [`RFC8247-2.4-2`](#rfc8247-2.4-2), [`RFC8247-2.4-3`](#rfc8247-2.4-3), [`RFC8247-3.1-2`](#rfc8247-3.1-2), [`RFC8247-3.1.1-1`](#rfc8247-3.1.1-1), [`RFC8247-3.2-1`](#rfc8247-3.2-1), [`RFC8247-3.2-3`](#rfc8247-3.2-3)

**Annotated (including scoped evidence) (4):** [`RFC8247-2.2-2`](#rfc8247-2.2-2), [`RFC8247-2.3-2`](#rfc8247-2.3-2), [`RFC8247-3.1-1`](#rfc8247-3.1-1), [`RFC8247-3.2-2`](#rfc8247-3.2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8247-2.1-1` | Algorithms that are not AEAD MUST be used in conjunction with one of the integrity algorithms in Section 2.3. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC8247NonAEADCipherNeedsIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L71). **negative:** `unit/verify` [`TestRFC8247NonAEADCipherNeedsIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L64) |
| `RFC8247-2.1-2` | ENCR_AES_CBC is raised from SHOULD+ for 128-bit keys and MAY for 256-bit keys in [RFC4307] to MUST. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC8247AESCBCKeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L95). **negative:** `unit/verify` [`TestRFC8247AESCBCKeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L127) |
| `RFC8247-2.1-3` | It provides no meaningful security whatsoever and, therefore, MUST NOT be implemented. (§2.1) | MUST NOT | 2.1 | **positive:** `unit/verify` [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L175). **negative:** `unit/verify` [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L159) |
| `RFC8247-2.1-4` | This was fixed and this document now lists ENCR_NULL as MUST NOT. (§2.1) | MUST NOT | 2.1 | **positive:** `unit/verify` [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L177). **negative:** `unit/verify` [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L167) |
| `RFC8247-2.2-1` | PRF_HMAC_SHA2_256 MUST be implemented in order to replace SHA1 and PRF_HMAC_SHA1. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC8247PRFHMACSHA2256`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L189). **negative:** `unit/verify` [`TestRFC8247PRFHMACSHA2256`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L217) |
| `RFC8247-2.2-2` | PRF_HMAC_SHA1 has been downgraded from MUST in RFC 4307 to MUST-, as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** MUST- means the same as MUST (Section 1.1), and ze's IKE PRF registry holds HMAC-SHA2-256, 384 and 512 only; internal/component/ike/crypto/transform.go prfRegistry |
| `RFC8247-2.2-3` | PRF_HMAC_MD5 has been downgraded from MAY in RFC 4307 to MUST NOT. (§2.2) | MUST NOT | 2.2 | **positive:** `unit/verify` [`TestRFC8247IKERefusesPRFHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L253). **negative:** `unit/verify` [`TestRFC8247IKERefusesPRFHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L234) |
| `RFC8247-2.3-1` | AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC8247IntegrityHMACSHA2256128`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L266). **negative:** `unit/verify` [`TestRFC8247IntegrityHMACSHA2256128`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L294) |
| `RFC8247-2.3-2` | AUTH_HMAC_SHA1_96 has been downgraded from MUST in RFC 4307 to MUST- as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** MUST- means the same as MUST (Section 1.1), and AUTH_HMAC_SHA1_96 sits in the ESP-only registry, so an IKE SA can neither offer nor accept it; internal/component/ike/crypto/transform.go espOnlyIntegrityRegistry |
| `RFC8247-2.3-3` | These have been downgraded to MUST NOT. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L344). **negative:** `unit/verify` [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L335) |
| `RFC8247-2.3-4` | AUTH_HMAC_MD5_96 is also demoted to MUST NOT. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L346). **negative:** `unit/verify` [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L340) |
| `RFC8247-2.4-1` | Group 14 or the 2048-bit MODP Group is raised from SHOULD+ in RFC 4307 to MUST as a replacement for the 1024-bit MODP Group. (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestRFC8247DHGroup14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L358). **negative:** `unit/verify` [`TestRFC8247DHGroup14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L400) |
| `RFC8247-2.4-2` | It has, therefore, been downgraded to MUST NOT. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L438). **negative:** `unit/verify` [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L430) |
| `RFC8247-2.4-3` | This has resulted in group 22 to be demoted to MUST NOT. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L440). **negative:** `unit/verify` [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L434) |
| `RFC8247-3.1-1` | RSA Digital Signature is widely deployed and, therefore, kept for interoperability. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never sends AUTH method 1, and computeX509Auth refuses to sign at all when the peer sent no SIGNATURE_HASH_ALGORITHMS notify, so a peer that supports only RSA Digital Signature cannot authenticate ze; verifyLegacyRSAAuth verifies method 1 with a SHA-256 DigestInfo only, while RFC 7296 Section 3.8 has method 1 implementations default to SHA-1; internal/component/ike/engine/auth.go computeX509Auth, verifyLegacyRSAAuth |
| `RFC8247-3.1-2` | Shared Key Message Integrity Code is widely deployed and mandatory to implement in the IKEv2 in RFC 7296. The status remains MUST. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC8247SharedKeyAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L39). **negative:** `unit/verify` [`TestRFC8247SharedKeyAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L56) |
| `RFC8247-3.1.1-1` | IKEv2 [RFC7296] mandates support for the RSA keys of the bit size 1024 or 2048 (§3.1.1) | MUST | 3.1.1 | **positive:** `unit/verify` [`TestRFC8247RSAKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L78). **negative:** `unit/verify` [`TestRFC8247RSAKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L88) |
| `RFC8247-3.2-1` | When a Digital Signature authentication method is implemented, the following recommendations are applied for hash functions: (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L125). **negative:** `unit/verify` [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L133) |
| `RFC8247-3.2-2` | When the Digital Signature authentication method is used with RSA signature algorithm, RSASSA-PSS MUST be supported and RSASSA- PKCS1-v1.5 MAY be supported. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze recognises only the sha256WithRSAEncryption, sha384WithRSAEncryption and sha512WithRSAEncryption identifiers and verifies with rsa.VerifyPKCS1v15, so no RSASSA-PSS signature is produced or accepted; internal/component/ike/engine/auth.go hashFromAlgID |
| `RFC8247-3.2-3` | These recommendations are applied only if the Digital Signature authentication method is implemented. (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L169). **negative:** `unit/verify` [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L140) |
| `RFC8247-2.4-6` | Group 2 or the 1024-bit MODP Group has been downgraded from MUST- in RFC 4307 to SHOULD NOT. (§2.4) | SHOULD NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8247-2.4-7` | Groups 23 and 24 have been demoted to SHOULD NOT and are expected to be further downgraded in the near future to MUST NOT. (§2.4) | SHOULD NOT | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8247-3.1-3` | They are currently at SHOULD NOT and are expected to be downgraded to MUST NOT in the future. (§3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8247-3.1.1-2` | Since these signatures only have value in real time and need no future protection, smaller keys were kept at SHOULD NOT instead of MUST NOT. (§3.1.1) | SHOULD NOT | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8247-2.1-5` | ENCR_3DES has been downgraded from RFC 4307 MUST- to MAY. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8247-2.2-2`](#rfc8247-2.2-2) PRF_HMAC_SHA1 has been downgraded from MUST in RFC 4307 to MUST-, as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.2) | {gap}, no test | MUST- means the same as MUST (Section 1.1), and ze's IKE PRF registry holds HMAC-SHA2-256, 384 and 512 only; internal/component/ike/crypto/transform.go prfRegistry |
| [`RFC8247-2.3-2`](#rfc8247-2.3-2) AUTH_HMAC_SHA1_96 has been downgraded from MUST in RFC 4307 to MUST- as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.3) | {gap}, no test | MUST- means the same as MUST (Section 1.1), and AUTH_HMAC_SHA1_96 sits in the ESP-only registry, so an IKE SA can neither offer nor accept it; internal/component/ike/crypto/transform.go espOnlyIntegrityRegistry |
| [`RFC8247-3.1-1`](#rfc8247-3.1-1) RSA Digital Signature is widely deployed and, therefore, kept for interoperability. (§3.1) | {gap}, no test | ze never sends AUTH method 1, and computeX509Auth refuses to sign at all when the peer sent no SIGNATURE_HASH_ALGORITHMS notify, so a peer that supports only RSA Digital Signature cannot authenticate ze; verifyLegacyRSAAuth verifies method 1 with a SHA-256 DigestInfo only, while RFC 7296 Section 3.8 has method 1 implementations default to SHA-1; internal/component/ike/engine/auth.go computeX509Auth, verifyLegacyRSAAuth |
| [`RFC8247-3.2-2`](#rfc8247-3.2-2) When the Digital Signature authentication method is used with RSA signature algorithm, RSASSA-PSS MUST be supported and RSASSA- PKCS1-v1.5 MAY be supported. (§3.2) | {gap}, no test | ze recognises only the sha256WithRSAEncryption, sha384WithRSAEncryption and sha512WithRSAEncryption identifiers and verifies with rsa.VerifyPKCS1v15, so no RSASSA-PSS signature is produced or accepted; internal/component/ike/engine/auth.go hashFromAlgID |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8247-2.1-1`](#rfc8247-2.1-1)

Algorithms that are not AEAD MUST be used in conjunction with one of the integrity algorithms in Section 2.3. (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247NonAEADCipherNeedsIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC8247NonAEADCipherNeedsIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L71) | unit/verify | revert, verified |

### [`RFC8247-2.1-2`](#rfc8247-2.1-2)

ENCR_AES_CBC is raised from SHOULD+ for 128-bit keys and MAY for 256-bit keys in [RFC4307] to MUST. (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247AESCBCKeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestRFC8247AESCBCKeysTheIKESA`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L95) | unit/verify | revert, verified |

### [`RFC8247-2.1-3`](#rfc8247-2.1-3)

It provides no meaningful security whatsoever and, therefore, MUST NOT be implemented. (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L175) | unit/verify | revert, verified |

### [`RFC8247-2.1-4`](#rfc8247-2.1-4)

This was fixed and this document now lists ENCR_NULL as MUST NOT. (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L167) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesENCRDESAndENCRNULL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L177) | unit/verify | revert, verified |

### [`RFC8247-2.2-1`](#rfc8247-2.2-1)

PRF_HMAC_SHA2_256 MUST be implemented in order to replace SHA1 and PRF_HMAC_SHA1. (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247PRFHMACSHA2256`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L217) | unit/verify | revert, verified |
| positive | [`TestRFC8247PRFHMACSHA2256`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L189) | unit/verify | revert, verified |

### [`RFC8247-2.2-2`](#rfc8247-2.2-2)

PRF_HMAC_SHA1 has been downgraded from MUST in RFC 4307 to MUST-, as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8247-2.2-2, so no unit is bound to it.

### [`RFC8247-2.2-3`](#rfc8247-2.2-3)

PRF_HMAC_MD5 has been downgraded from MAY in RFC 4307 to MUST NOT. (§2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesPRFHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L234) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesPRFHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L253) | unit/verify | revert, verified |

### [`RFC8247-2.3-1`](#rfc8247-2.3-1)

AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96. (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IntegrityHMACSHA2256128`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L294) | unit/verify | revert, verified |
| positive | [`TestRFC8247IntegrityHMACSHA2256128`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L266) | unit/verify | revert, verified |

### [`RFC8247-2.3-2`](#rfc8247-2.3-2)

AUTH_HMAC_SHA1_96 has been downgraded from MUST in RFC 4307 to MUST- as cryptographic attacks against SHA1 are increasing, resulting in an industry-wide trend to deprecate its usage. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8247-2.3-2, so no unit is bound to it.

### [`RFC8247-2.3-3`](#rfc8247-2.3-3)

These have been downgraded to MUST NOT. (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L335) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L344) | unit/verify | revert, verified |

### [`RFC8247-2.3-4`](#rfc8247-2.3-4)

AUTH_HMAC_MD5_96 is also demoted to MUST NOT. (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L340) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesDESMACAndMD5Integrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L346) | unit/verify | revert, verified |

### [`RFC8247-2.4-1`](#rfc8247-2.4-1)

Group 14 or the 2048-bit MODP Group is raised from SHOULD+ in RFC 4307 to MUST as a replacement for the 1024-bit MODP Group. (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247DHGroup14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L400) | unit/verify | revert, verified |
| positive | [`TestRFC8247DHGroup14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L358) | unit/verify | revert, verified |

### [`RFC8247-2.4-2`](#rfc8247-2.4-2)

It has, therefore, been downgraded to MUST NOT. (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L430) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L438) | unit/verify | revert, verified |

### [`RFC8247-2.4-3`](#rfc8247-2.4-3)

This has resulted in group 22 to be demoted to MUST NOT. (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L434) | unit/verify | revert, verified |
| positive | [`TestRFC8247IKERefusesDHGroups1And22`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8247_ike_algorithms_test.go#L440) | unit/verify | revert, verified |

### [`RFC8247-3.1-1`](#rfc8247-3.1-1)

RSA Digital Signature is widely deployed and, therefore, kept for interoperability. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8247-3.1-1, so no unit is bound to it.

### [`RFC8247-3.1-2`](#rfc8247-3.1-2)

Shared Key Message Integrity Code is widely deployed and mandatory to implement in the IKEv2 in RFC 7296. The status remains MUST. (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247SharedKeyAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC8247SharedKeyAuth`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L39) | unit/verify | revert, verified |

### [`RFC8247-3.1.1-1`](#rfc8247-3.1.1-1)

IKEv2 [RFC7296] mandates support for the RSA keys of the bit size 1024 or 2048 (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247RSAKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC8247RSAKeySizes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L78) | unit/verify | revert, verified |

### [`RFC8247-3.2-1`](#rfc8247-3.2-1)

When a Digital Signature authentication method is implemented, the following recommendations are applied for hash functions: (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L125) | unit/verify | revert, verified |

### [`RFC8247-3.2-2`](#rfc8247-3.2-2)

When the Digital Signature authentication method is used with RSA signature algorithm, RSASSA-PSS MUST be supported and RSASSA- PKCS1-v1.5 MAY be supported. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8247-3.2-2, so no unit is bound to it.

### [`RFC8247-3.2-3`](#rfc8247-3.2-3)

These recommendations are applied only if the Digital Signature authentication method is implemented. (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestRFC8247SignatureHashes`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L169) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc8247.txt |
| Source fingerprint | 1faf9eb5ca639556 |
| Record | rfc/extraction/rfc8247.json |
| Mapped sentences | 20 |
| Declined as scope | 21 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 5 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 2 | walked | not stated |
| `1.4` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 6 | walked | not stated |
| `2.2` | not stated | 4 | walked | not stated |
| `2.3` | not stated | 5 | walked | not stated |
| `2.4` | not stated | 8 | walked | not stated |
| `2.5` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.1.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `7` | references | 0 | skipped (references) | references |
| `7.1` | normative references | 0 | skipped (references) | normative references |
| `7.2` | informative references | 0 | skipped (references) | informative references |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | When used in the tables in this document, these terms indicate that the listed algorithm MUST, MUST NOT, SHOULD, SHOULD NOT, or MAY be implemented as part of an IKEv2 implementation. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | However, it is likely that an algorithm marked as SHOULD+ will be promoted at some future time to be a MUST. |
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | MUST- This term means the same as MUST. |
| `1.1:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | However, it is expected at some point that this algorithm will no longer be a MUST in a future document. |
| `1.1:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | Although its status will be determined at a later time, it is reasonable to expect that if a future revision of a document alters the status of a MUST- algorithm, it will remain at least a SHOULD or a SHOULD- level. |
| `1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | Unless there are strong security reasons, an algorithm is expected to be downgraded from MUST to MUST- or SHOULD, instead of MUST NOT. |
| `1.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 1.1) or states the authors' expectation for future revisions (Section 1.3); it obliges no implementer | Similarly, an algorithm that has not been mentioned as mandatory-to- implement is expected to be introduced with a SHOULD instead of a MUST. |
| `2.1:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "ENCR_3DES has been downgraded from RFC 4307 MUST- to MAY": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | ENCR_3DES has been downgraded from RFC 4307 MUST- to MAY. |
| `2.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the ENCR_NULL MUST NOT that the next sentence of Section 2.1 grades | ENCR_NULL was incorrectly specified as MAY in RFC 4307, even when [RFC7296], Section 5 clearly states that it MUST NOT be used. |
| `2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 2.2 table; each graded row is captured by the prose sentences that follow it | +-------------------+----------+---------+ \| Name \| Status \| Comment \| +-------------------+----------+---------+ \| PRF_HMAC_SHA2_256 \| MUST \| \| \| PRF_HMAC_SHA2_512 \| SHOULD+ \| \| \| PRF_HMAC_SHA1 \| MUST- \| \| \| PRF_AES128_XCBC \| SHOULD \| (IoT) \| \| PRF_HMAC_MD5 \| MUST NOT \| \| +-------------------+----------+---------+ |
| `2.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 2.3 table; each graded row is captured by the prose sentences that follow it | +------------------------+----------+---------+ \| Name \| Status \| Comment \| +------------------------+----------+---------+ \| AUTH_HMAC_SHA2_256_128 \| MUST \| \| \| AUTH_HMAC_SHA2_512_256 \| SHOULD \| \| \| AUTH_HMAC_SHA1_96 \| MUST- \| \| \| AUTH_AES_XCBC_96 \| SHOULD \| (IoT) \| \| AUTH_HMAC_MD5_96 \| MUST NOT \| \| \| AUTH_DES_MAC \| MUST NOT \| \| \| AUTH_KPDK_MD5 \| MUST NOT \| \| +------------------------+----------+---------+ |
| `2.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 2.4 table; each graded group is captured by the prose sentences that follow it | This differs from attacks that need to be executed during the authentication that must be performed online and in near real time. +--------+---------------------------------------------+------------+ \| Number \| Description \| Status \| +--------+---------------------------------------------+------------+ \| 14 \| 2048-bit MODP Group \| MUST \| \| 19 \| 256-bit random ECP group \| SHOULD \| \| 5 \| 1536-bit MODP Group \| SHOULD NOT \| \| 2 \| 1024-bit MODP Group \| SHOULD NOT \| \| 1 \| 768-bit MODP Group \| MUST NOT \| \| 22 \| 1024-bit MODP Group with 160-bit Prime \| MUST NOT \| \| \| Order Subgroup \| \| \| 23 \| 2048-bit MODP Group with 224-bit Prime \| SHOULD NOT \| \| \| Order Subgroup \| \| \| 24 \| 2048-bit MODP Group with 256-bit Prime \| SHOULD NOT \| \| \| Order Subgroup \| \| +--------+---------------------------------------------+------------+ |
| `2.4:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "downgraded from MUST- in RFC 4307 to SHOULD NOT": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | Group 2 or the 1024-bit MODP Group has been downgraded from MUST- in RFC 4307 to SHOULD NOT. |
| `2.4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | predicts a future downgrade of group 2 and grades nothing today | It is expected in the near future to be downgraded to MUST NOT. |
| `2.4:7` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "Groups 23 and 24 have been demoted to SHOULD NOT": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | Groups 23 and 24 have been demoted to SHOULD NOT and are expected to be further downgraded in the near future to MUST NOT. |
| `2.4:8` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "Groups 23 and 24 have been demoted to SHOULD NOT": the RFC 6989 small-subgroup check is owed only when those groups are used, and ze offers neither (internal/component/ike/crypto/transform.go dhGroupRegistry holds 14, 19 and 20). The absent groups are disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc8247.md | Since groups 23 and 24 have small subgroups, the checks specified in the first bullet point of Section 2.2 of "Additional Diffie-Hellman Tests for the Internet Key Exchange Protocol Version 2 (IKEv2)" [RFC6989] MUST be done when these groups are used. |
| `2.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 2.5 change summary restates the levels graded in Sections 2.1 to 2.4 | +---------------------+--------------------------+------------+ \| Algorithm \| RFC 4307 \| RFC 8247 \| +---------------------+--------------------------+------------+ \| ENCR_3DES \| MUST- \| MAY \| \| ENCR_NULL \| MUST NOT (per [Err1937]) \| MUST NOT \| \| ENCR_AES_CBC \| SHOULD+ \| MUST \| \| ENCR_AES_CTR \| SHOULD \| MAY(*) \| \| PRF_HMAC_MD5 \| MAY \| MUST NOT \| \| PRF_HMAC_SHA1 \| MUST \| MUST- \| \| PRF_AES128_XCBC \| SHOULD+ \| SHOULD \| \| AUTH_HMAC_MD5_96 \| MAY \| MUST NOT \| \| AUTH_HMAC_SHA1_96 \| MUST \| MUST- \| \| AUTH_AES_XCBC_96 \| SHOULD+ \| SHOULD \| \| Group 2 (1024-bit) \| MUST- \| SHOULD NOT \| \| Group 14 (2048-bit) \| SHOULD+ \| MUST \| +---------------------+--------------------------+------------+ |
| `3.1:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "They are currently at SHOULD NOT": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | They are currently at SHOULD NOT and are expected to be downgraded to MUST NOT in the future. |
| `3.1.1:2` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "smaller keys were kept at SHOULD NOT instead of MUST NOT": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | Since these signatures only have value in real time and need no future protection, smaller keys were kept at SHOULD NOT instead of MUST NOT. |
| `3.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the footnote to the Section 3.2 table restates its MUST NOT rows for the SHA-1 based algorithms | (*) Empty or Default parameters means it is using SHA1, which is at the MUST NOT level. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations describe the algorithms graded MUST or SHOULD in quotation; no obligation is stated | The algorithms identified in this document as "MUST implement" or "SHOULD implement" are not known to be broken at the current time, and cryptographic research so far leads us to believe that they will likely remain secure into the foreseeable future. |

## Superseded

No document obsoletes RFC 8247, so its obligations are stated where they were written.
