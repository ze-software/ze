# RFC 8221 - Cryptographic Algorithm Implementation Requirements and Usage Guidance for Encapsulating Security Payload (ESP) and Authentication Header (AH)

ESP algorithms. Every requirement listed for RFC 8221, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16 | of 16 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 30 | 30 of 32 tests linked to requirements, 0 without a fault test and 2 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 19 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | ESP algorithms |
| Automatic checks | Included |
| Requirements | 19 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 32 |
| Tests linked to requirements | 32 |
| Recorded audit verdicts | 0 |
| Fault test records | 32 |
| Summary | `rfc/short/rfc8221.md` |
| Requirement table | `rfc/requirements/rfc8221.md` |
| RFC text | `rfc/full/rfc8221.txt` |

## Automatic checks

Included in automatic checks: ESP and AH algorithm requirements: sixteen MUST-level rows, every one classified and none a gap. ze's ESP registries (internal/component/ike/crypto/transform.go encryptionRegistry, espOnlyEncryptionRegistry, integrityRegistry, espOnlyIntegrityRegistry) offer ENCR_NULL, ENCR_AES_CBC and ENCR_AES_GCM_16 with 128- and 256-bit keys, AUTH_HMAC_SHA1_96 and AUTH_HMAC_SHA2_256_128, and none of the MUST NOT algorithms. Every row owes a tagged test. The IPCOMP_OUI row is conditional on compression, which this RFC leaves optional and ze does not offer (feature-out-of-scope in the extraction).

## Published support status

**Area:** ESP algorithms

**Proof:** 15 of 16 MUST proven

**Evidence:** unit 15

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (16):** [`RFC8221-3-1`](#rfc8221-3-1), [`RFC8221-4-1`](#rfc8221-4-1), [`RFC8221-5-1`](#rfc8221-5-1), [`RFC8221-5-2`](#rfc8221-5-2), [`RFC8221-5-3`](#rfc8221-5-3), [`RFC8221-5-4`](#rfc8221-5-4), [`RFC8221-5-5`](#rfc8221-5-5), [`RFC8221-5-6`](#rfc8221-5-6), [`RFC8221-5-7`](#rfc8221-5-7), [`RFC8221-5-8`](#rfc8221-5-8), [`RFC8221-6-1`](#rfc8221-6-1), [`RFC8221-6-2`](#rfc8221-6-2), [`RFC8221-6-3`](#rfc8221-6-3), [`RFC8221-6-4`](#rfc8221-6-4), [`RFC8221-6-5`](#rfc8221-6-5), [`RFC8221-6-6`](#rfc8221-6-6)

## Figures

- **MUST bar**: Ze proven 15, Proof pending 1
- **Overview bar**: Ze MUST proven 15, Proof pending 1, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze proven | 15 |
| OS verified | 0 |
| Proof pending | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8221.json](../../../data/rfc/rfc8221.json).

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

RFC 8221 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC8221-3-1`](#rfc8221-3-1)

If manual keying is used regardless, Counter Mode algorithms such as ENCR_AES_CTR, ENCR_AES_CCM, ENCR_AES_GCM, and ENCR_CHACHA20_POLY1305 MUST NOT be used, as it is incompatible with a secure and persistent handling of the counter (as explained in the Security Considerations section of [RFC3686]). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L64) | unit/verify | revert, verified |

### [`RFC8221-4-1`](#rfc8221-4-1)

Encryption without authentication is not effective and MUST NOT be used. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L48) | unit/verify | revert, verified |

### [`RFC8221-5-1`](#rfc8221-5-1)

For that purpose, the requirement level for 128-bit keys and 256-bit keys is MUST (when applicable). (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L100) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L69) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC8221-5-2`](#rfc8221-5-2)

These algorithms were not mentioned in [RFC7321], and this document clarifies that such algorithms MUST NOT be implemented for IPsec communications. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L126) | unit/verify | revert, verified |

### [`RFC8221-5-3`](#rfc8221-5-3)

As a result, these algorithms MUST NOT be implemented. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L128) | unit/verify | revert, verified |

### [`RFC8221-5-4`](#rfc8221-5-4)

Various ciphers that are older, not well tested, and never widely implemented have been changed to MUST NOT. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L130) | unit/verify | revert, verified |

### [`RFC8221-5-5`](#rfc8221-5-5)

ENCR_BLOWFISH has been downgraded to MUST NOT as it has been deprecated for years by TWOFISH, which is not standardized for ESP and therefore not listed in this document. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesForbiddenCiphers`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L132) | unit/verify | revert, verified |

### [`RFC8221-5-6`](#rfc8221-5-6)

ENCR_NULL status was set to MUST in [RFC7321] and remains a MUST to enable the use of ESP with only authentication, which is preferred over AH due to NAT traversal. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L92) | unit/verify | revert, verified |

### [`RFC8221-5-7`](#rfc8221-5-7)

ENCR_AES_CBC MUST be implemented in order to enable interoperability between implementations that followed [RFC7321]. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L72) | unit/verify | revert, verified |

### [`RFC8221-5-8`](#rfc8221-5-8)

ENCR_AES_GCM_16 status has been updated from SHOULD+ to MUST in order to favor the use of authenticated encryption and AEAD algorithms. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryCiphersInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L82) | unit/verify | revert, verified |

### [`RFC8221-6-1`](#rfc8221-6-1)

AUTH_NONE has been downgraded from MAY in [RFC7321] to MUST NOT. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L50) | unit/verify | revert, verified |

### [`RFC8221-6-2`](#rfc8221-6-2)

In all other cases, AUTH_NONE MUST NOT be selected. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPEncryptionIsAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/component/ike/ipsec/rfc8221_esp_auth_none_test.go#L52) | unit/verify | revert, verified |

### [`RFC8221-6-3`](#rfc8221-6-3)

As MD5 is known to be vulnerable to collisions, these algorithms MUST NOT be used. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L168) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L177) | unit/verify | revert, verified |

### [`RFC8221-6-4`](#rfc8221-6-4)

AUTH_HMAC_SHA1_96 has been downgraded from MUST in [RFC7321] to MUST- as there is an industry-wide trend to deprecate its usage. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L142) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L130) | unit/verify | revert, verified |

### [`RFC8221-6-5`](#rfc8221-6-5)

As DES is known to be vulnerable, it MUST NOT be used. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPRefusesMD5AndDESMACIntegrity`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc8221_esp_algorithms_test.go#L180) | unit/verify | revert, verified |

### [`RFC8221-6-6`](#rfc8221-6-6)

AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace AUTH_HMAC_SHA1_96. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC8221ESPMandatoryIntegrityInstall`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc8221_esp_install_linux_test.go#L136) | unit/verify | revert, verified |

## Requirement list review

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
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 2 | reviewed | not stated |
| `1.3` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 4 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 13 | reviewed | not stated |
| `6` | not stated | 8 | reviewed | not stated |
| `7` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `10` | not stated | 1 | reviewed | not stated |
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
