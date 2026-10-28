# RFC 8221 - Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH)

Partial. Every requirement this repository extracted from RFC 8221, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 16 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 16 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 32 of 32 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 19 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 19 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 32 |
| Tagged units | 32 |
| Recorded audit verdicts | 0 |
| Discrimination records | 32 |
| Summary | `rfc/short/rfc8221.md` |
| Requirement shard | `rfc/requirements/rfc8221.md` |
| RFC text | `rfc/full/rfc8221.txt` |

## Enrolment

Enrolled: ESP and AH algorithm requirements: sixteen MUST-level rows, every one classified and none a gap. ze's ESP registries (internal/component/ike/crypto/transform.go encryptionRegistry, espOnlyEncryptionRegistry, integrityRegistry, espOnlyIntegrityRegistry) offer ENCR_NULL, ENCR_AES_CBC and ENCR_AES_GCM_16 with 128- and 256-bit keys, AUTH_HMAC_SHA1_96 and AUTH_HMAC_SHA2_256_128, and none of the MUST NOT algorithms. Every row owes a tagged test. The IPCOMP_OUI row is conditional on compression, which this RFC leaves optional and ze does not offer (feature-out-of-scope in the extraction).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- ESP encryption ENCR_NULL, ENCR_AES_CBC, ENCR_AES_GCM_16, ENCR_CHACHA20_POLY1305 and ENCR_3DES
- integrity AUTH_HMAC_SHA1_96, AUTH_HMAC_SHA2_256_128, 384_192 and 512_256. No DES, Blowfish, 3IDEA, MD5 or DES-MAC.


**What the ledger says remains:**

ENCR_AES_CCM_8 for ESP, AUTH_AES_XCBC_96, the GMAC algorithms and IPComp are not offered. Requirement tests are owed for every MUST-level row.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC8221-3-1`](#rfc8221-3-1), [`RFC8221-4-1`](#rfc8221-4-1), [`RFC8221-5-1`](#rfc8221-5-1), [`RFC8221-5-2`](#rfc8221-5-2), [`RFC8221-5-3`](#rfc8221-5-3), [`RFC8221-5-4`](#rfc8221-5-4), [`RFC8221-5-5`](#rfc8221-5-5), [`RFC8221-5-6`](#rfc8221-5-6), [`RFC8221-5-7`](#rfc8221-5-7), [`RFC8221-5-8`](#rfc8221-5-8), [`RFC8221-6-1`](#rfc8221-6-1), [`RFC8221-6-2`](#rfc8221-6-2), [`RFC8221-6-3`](#rfc8221-6-3), [`RFC8221-6-4`](#rfc8221-6-4), [`RFC8221-6-5`](#rfc8221-6-5), [`RFC8221-6-6`](#rfc8221-6-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8221-3-1` | If manual keying is used regardless, Counter Mode algorithms such as ENCR_AES_CTR, ENCR_AES_CCM, ENCR_AES_GCM, and ENCR_CHACHA20_POLY1305 MUST NOT be used, as it is incompatible with a secure and persistent handling of the counter (as explained in the Security Considerations section of [RFC3686]). (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L64). **negative:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L45) |
| `RFC8221-4-1` | Encryption without authentication is not effective and MUST NOT be used. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L48). **negative:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L35) |
| `RFC8221-5-1` | For that purpose, the requirement level for 128-bit keys and 256-bit keys is MUST (when applicable). (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L69). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L100) |
| `RFC8221-5-2` | These algorithms were not mentioned in [RFC7321], and this document clarifies that such algorithms MUST NOT be implemented for IPsec communications. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L126). **negative:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L98) |
| `RFC8221-5-3` | As a result, these algorithms MUST NOT be implemented. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L128). **negative:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L104) |
| `RFC8221-5-4` | Various ciphers that are older, not well tested, and never widely implemented have been changed to MUST NOT. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L130). **negative:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L114) |
| `RFC8221-5-5` | ENCR_BLOWFISH has been downgraded to MUST NOT as it has been deprecated for years by TWOFISH, which is not standardized for ESP and therefore not listed in this document. (§5) | MUST NOT | 5 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L132). **negative:** `unit/verify` [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L121) |
| `RFC8221-5-6` | ENCR_NULL status was set to MUST in [RFC7321] and remains a MUST to enable the use of ESP with only authentication, which is preferred over AH due to NAT traversal. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L92). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L102) |
| `RFC8221-5-7` | ENCR_AES_CBC MUST be implemented in order to enable interoperability between implementations that followed [RFC7321]. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L72). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L104) |
| `RFC8221-5-8` | ENCR_AES_GCM_16 status has been updated from SHOULD+ to MUST in order to favor the use of authenticated encryption and AEAD algorithms. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L82). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L106) |
| `RFC8221-6-1` | AUTH_NONE has been downgraded from MAY in [RFC7321] to MUST NOT. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L50). **negative:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L37) |
| `RFC8221-6-2` | In all other cases, AUTH_NONE MUST NOT be selected. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L52). **negative:** `unit/verify` [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L39) |
| `RFC8221-6-3` | As MD5 is known to be vulnerable to collisions, these algorithms MUST NOT be used. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L177). **negative:** `unit/verify` [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L168) |
| `RFC8221-6-4` | AUTH_HMAC_SHA1_96 has been downgraded from MUST in [RFC7321] to MUST- as there is an industry-wide trend to deprecate its usage. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L130). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L142) |
| `RFC8221-6-5` | As DES is known to be vulnerable, it MUST NOT be used. (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L180). **negative:** `unit/verify` [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L173) |
| `RFC8221-6-6` | AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L136). **negative:** `unit/verify` [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L144) |
| `RFC8221-5-9` | Its status has been set to SHOULD and is expected to become MUST in the long term. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8221-6-7` | AUTH_HMAC_SHA2_512_256 SHOULD be implemented as a future replacement of AUTH_HMAC_SHA2_256_128 or when stronger security is required. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8221-7-1` | As it is not widely deployed, it remains optional and at the MAY level. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8221 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8221-3-1`](#rfc8221-3-1)

If manual keying is used regardless, Counter Mode algorithms such as ENCR_AES_CTR, ENCR_AES_CCM, ENCR_AES_GCM, and ENCR_CHACHA20_POLY1305 MUST NOT be used, as it is incompatible with a secure and persistent handling of the counter (as explained in the Security Considerations section of [RFC3686]). (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L64) | unit/verify | revert, verified |

### [`RFC8221-4-1`](#rfc8221-4-1)

Encryption without authentication is not effective and MUST NOT be used. (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L48) | unit/verify | revert, verified |

### [`RFC8221-5-1`](#rfc8221-5-1)

For that purpose, the requirement level for 128-bit keys and 256-bit keys is MUST (when applicable). (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L69) | unit/verify | revert, verified |

### [`RFC8221-5-2`](#rfc8221-5-2)

These algorithms were not mentioned in [RFC7321], and this document clarifies that such algorithms MUST NOT be implemented for IPsec communications. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L126) | unit/verify | revert, verified |

### [`RFC8221-5-3`](#rfc8221-5-3)

As a result, these algorithms MUST NOT be implemented. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L128) | unit/verify | revert, verified |

### [`RFC8221-5-4`](#rfc8221-5-4)

Various ciphers that are older, not well tested, and never widely implemented have been changed to MUST NOT. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L130) | unit/verify | revert, verified |

### [`RFC8221-5-5`](#rfc8221-5-5)

ENCR_BLOWFISH has been downgraded to MUST NOT as it has been deprecated for years by TWOFISH, which is not standardized for ESP and therefore not listed in this document. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L132) | unit/verify | revert, verified |

### [`RFC8221-5-6`](#rfc8221-5-6)

ENCR_NULL status was set to MUST in [RFC7321] and remains a MUST to enable the use of ESP with only authentication, which is preferred over AH due to NAT traversal. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L92) | unit/verify | revert, verified |

### [`RFC8221-5-7`](#rfc8221-5-7)

ENCR_AES_CBC MUST be implemented in order to enable interoperability between implementations that followed [RFC7321]. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L72) | unit/verify | revert, verified |

### [`RFC8221-5-8`](#rfc8221-5-8)

ENCR_AES_GCM_16 status has been updated from SHOULD+ to MUST in order to favor the use of authenticated encryption and AEAD algorithms. (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L82) | unit/verify | revert, verified |

### [`RFC8221-6-1`](#rfc8221-6-1)

AUTH_NONE has been downgraded from MAY in [RFC7321] to MUST NOT. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L50) | unit/verify | revert, verified |

### [`RFC8221-6-2`](#rfc8221-6-2)

In all other cases, AUTH_NONE MUST NOT be selected. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L52) | unit/verify | revert, verified |

### [`RFC8221-6-3`](#rfc8221-6-3)

As MD5 is known to be vulnerable to collisions, these algorithms MUST NOT be used. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L177) | unit/verify | revert, verified |

### [`RFC8221-6-4`](#rfc8221-6-4)

AUTH_HMAC_SHA1_96 has been downgraded from MUST in [RFC7321] to MUST- as there is an industry-wide trend to deprecate its usage. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L142) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L130) | unit/verify | revert, verified |

### [`RFC8221-6-5`](#rfc8221-6-5)

As DES is known to be vulnerable, it MUST NOT be used. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L180) | unit/verify | revert, verified |

### [`RFC8221-6-6`](#rfc8221-6-6)

AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L136) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc8221.txt |
| Source fingerprint | 71e978851ccbe2c5 |
| Record | rfc/extraction/rfc8221.json |
| Mapped sentences | 16 |
| Declined as scope | 16 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 2 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 4 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 13 | walked | not stated |
| `6` | not stated | 8 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `10` | not stated | 1 | walked | not stated |
| `11` | references | 0 | skipped (references) | references |
| `11.1` | normative references | 0 | skipped (references) | normative references |
| `11.2` | informative references | 0 | skipped (references) | informative references |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | Unless there are strong security reasons, an algorithm is expected to be downgraded from MUST to MUST- or SHOULD, instead of MUST NOT (see Section 2). |
| `1.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | Similarly, an algorithm that has not been mentioned as mandatory-to-implement is expected to be introduced with a SHOULD instead of a MUST. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | However, it is likely that an algorithm marked as SHOULD+ will be promoted at some future time to be a MUST. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | MUST- This term means the same as MUST. |
| `2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | However, we expect at some point that this algorithm will no longer be a MUST in a future document. |
| `2:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | defines the requirement levels this document uses (Section 2) or states the authors' expectation for future revisions (Section 1.2); it obliges no implementer | Although its status will be determined at a later time, it is reasonable to expect that if a future revision of a document alters the status of a MUST- algorithm, it will remain at least a SHOULD or a SHOULD- level. |
| `5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 5 table; its MUST NOT rows are the older ciphers graded by the prose sentence that follows it | +-------------------------+------------+---------+----------------+ \| Name \| Status \| AEAD \| Comment \| +-------------------------+------------+---------+----------------+ \| ENCR_DES_IV64 \| MUST NOT \| No \| UNSPECIFIED \| \| ENCR_DES \| MUST NOT \| No \| [RFC2405] \| \| ENCR_3DES \| SHOULD NOT \| No \| [RFC2451] \| \| ENCR_BLOWFISH \| MUST NOT \| No \| [RFC2451] \| \| ENCR_3IDEA \| MUST NOT \| No \| UNSPECIFIED \| \| ENCR_DES_IV32 \| MUST NOT \| No \| UNSPECIFIED \| \| ENCR_NULL \| MUST \| No \| [RFC2410] \| \| ENCR_AES_CBC \| MUST \| No \| [RFC3602][1] \| \| ENCR_AES_CCM_8 \| SHOULD \| Yes \| [RFC4309](IoT) \| \| ENCR_AES_GCM_16 \| MUST \| Yes \| [RFC4106][1] \| \| ENCR_CHACHA20_POLY1305 \| SHOULD \| Yes \| [RFC7634] \| +-------------------------+------------+---------+----------------+ |
| `5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates for 256-bit keys the key-length MUST of the previous sentence | In that sense, the status for 256-bit keys has been raised from MAY in [RFC7321] to MUST. |
| `5:9` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | predicts that ENCR_NULL stays MUST; the grade itself is the previous sentence | ENCR_NULL is expected to remain MUST by protocol requirements. |
| `5:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the ENCR_AES_CBC MUST is graded by the next sentence of Section 5 | ENCR_AES_CBC status remains at MUST. |
| `5:13` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | the enclosing construction is "Its status has been set to SHOULD and is expected to become MUST in the long term": the capitalised MUST names the level the algorithm held in the obsoleted RFC or the level it may reach later; the grade the sentence assigns today is advisory | Its status has been set to SHOULD and is expected to become MUST in the long term. |
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 4 rule that a non-AEAD cipher needs an integrity algorithm | o Communications that are encrypted with a non-AEAD algorithm that MUST be combined with an authentication algorithm. |
| `6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 6 table; each graded row is captured by the prose sentences that follow it | +------------------------+----------------+-------------------------+ \| Name \| Status \| Comment \| +------------------------+----------------+-------------------------+ \| AUTH_NONE \| MUST / \| [RFC7296][RFC5282] \| \| \| MUST NOT \| AEAD-only \| \| AUTH_HMAC_MD5_96 \| MUST NOT \| [RFC2403][RFC7296] \| \| AUTH_HMAC_SHA1_96 \| MUST- \| [RFC2404][RFC7296] \| \| AUTH_DES_MAC \| MUST NOT \| UNSPECIFIED \| \| AUTH_KPDK_MD5 \| MUST NOT \| UNSPECIFIED \| \| AUTH_AES_XCBC_96 \| SHOULD / MAY \| [RFC3566][RFC7296] \| \| \| \| (IoT) \| \| AUTH_AES_128_GMAC \| MAY \| [RFC4543] \| \| AUTH_AES_256_GMAC \| MAY \| [RFC4543] \| \| AUTH_HMAC_SHA2_256_128 \| MUST \| [RFC4868] \| \| AUTH_HMAC_SHA2_512_256 \| SHOULD \| [RFC4868] \| +------------------------+----------------+-------------------------+ |
| `7:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | "As it is not widely deployed, it remains optional and at the MAY level.": the IPCOMP_OUI MUST NOT grades a compression transform, and ze negotiates no IPComp transform for a Child SA (internal/component/ike/crypto/transform.go holds no compression registry). The absent compression is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc8221.md | +----------------+----------+-------------+ \| Name \| Status \| Comment \| +----------------+----------+-------------+ \| IPCOMP_OUI \| MUST NOT \| UNSPECIFIED \| \| IPCOMP_DEFLATE \| MAY \| [RFC3173] \| \| IPCOMP_LZS \| MAY \| [RFC2395] \| \| IPCOMP_LZJH \| MAY \| [RFC3051] \| +----------------+----------+-------------+ |
| `8:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Section 8 change summary restates the levels graded in Sections 5 and 6 | +-------------------+----------+-----------------+ \| Algorithm \| RFC 7321 \| RFC 8221 \| +-------------------+----------+-----------------+ \| ENCR_AES_GCM_16 \| SHOULD+ \| MUST \| \| ENCR_AES_CCM_8 \| MAY \| SHOULD \| \| ENCR_AES_CTR \| MAY \| MAY(*) \| \| ENCR_3DES \| MAY \| SHOULD NOT \| \| AUTH_HMAC_SHA1_96 \| MUST \| MUST- \| \| AUTH_AES_128_GMAC \| SHOULD+ \| MAY \| \| AUTH_NONE \| MAY \| MUST / MUST NOT \| +-------------------+----------+-----------------+ |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations describe the algorithms graded MUST or SHOULD in quotation; no obligation is stated | The algorithms identified in this document as "MUST implement" or "SHOULD implement" are not known to be broken at the current time, and cryptographic research to date leads us to believe that they will likely remain secure into the foreseeable future. |

## Superseded

No document obsoletes RFC 8221, so its obligations are stated where they were written.
