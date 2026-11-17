# RFC 4106 - The Use of Galois/Counter Mode (GCM) in IPsec Encapsulating Security Payload (ESP)

ESP. Every requirement listed for RFC 4106, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 8 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 8 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 8 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 8 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 8 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 8 | 8 of 8 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 8 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 8 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 3 | of 8 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 8 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 8 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 8 | of 13 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 8 mandatory requirements: they add up to 8. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | ESP |
| Automatic checks | Included |
| Requirements | 13 |
| Mandatory requirements (MUST level) | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tests linked to requirements | 8 |
| Recorded audit verdicts | 0 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc4106.md` |
| Requirement table | `rfc/requirements/rfc4106.md` |
| RFC text | `rfc/full/rfc4106.txt` |

## Automatic checks

Included in automatic checks: AES-GCM for ESP: eight MUST-level requirements, every one classified. Three are performed by the ESP datapath on the association ze installs and carry {lower-layer} (IV size, IV uniqueness, nonce reuse). Five are ze's own: IKEv2 keying (RFC4106-2-2), the 16-octet ICV ze installs (RFC4106-6-1), the four-octet salt in the KEYMAT (RFC4106-8.1-1, RFC4106-10-1) and rekeying (RFC4106-10-2); internal/component/ike/crypto/aead.go and internal/component/ike/dataplane/xfrm_linux.go produce them and they owe tagged tests. The two Section 8.4 sites bind an IKEv1 Phase 2 negotiator, which ze is not.

## Published support status

**Area:** ESP

**Test verification:** 5 of 8 MUST proven

**Evidence:** unit 5

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 5 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **8** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC4106-2-2`](#rfc4106-2-2), [`RFC4106-6-1`](#rfc4106-6-1), [`RFC4106-8.1-1`](#rfc4106-8.1-1)

**Annotated (including scoped evidence) (5):** [`RFC4106-2-1`](#rfc4106-2-1), [`RFC4106-3.1-1`](#rfc4106-3.1-1), [`RFC4106-3.1-2`](#rfc4106-3.1-2), [`RFC4106-10-1`](#rfc4106-10-1), [`RFC4106-10-2`](#rfc4106-10-2)

## Figures

- **MUST bar**: Ze verified 5, OS testing gap 3
- **Overview bar**: Ze MUST verified 5, OS testing gap 3, Not assessed 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 8 |
| MUST rows kept (the MUST bar) | 8 |
| Ze verified | 5 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 3 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 5 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4106.json](../../../data/rfc/rfc4106.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4106-2-1` | The same nonce and key combination MUST NOT be used more than once. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)) and its seqiv IV generator, linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the datapath chooses every IV, so no value ze writes decides the nonce directly; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it from cycling is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) and rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) |
| `RFC4106-2-2` | For safety's sake, implementations MUST use an automated key management system, such as the Internet Key Exchange (IKE) [RFC2409], to ensure that this requirement is met. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L66). **negative:** `unit/verify` [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L49) |
| `RFC4106-3.1-1` | The AES-GCM-ESP IV field MUST be eight octets. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)), linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the algorithm and the kernel writes the IV field of every packet |
| `RFC4106-3.1-2` | For a given key, the IV MUST NOT repeat. (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc4106(gcm(aes)) and its seqiv IV generator, linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs no IV, so the kernel generates each one; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it from repeating is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) and rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) |
| `RFC4106-6-1` | Implementations MUST support a full-length 16-octet ICV, and MAY support 8 or 12 octet ICVs, and MUST NOT support other ICV lengths. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L40). **negative:** `unit/verify` [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L54) |
| `RFC4106-8.1-1` | The size of the KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the associated AES key. (§8.1) | MUST | 8.1 | **positive:** `unit/verify` [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L41). **negative:** `unit/verify` [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L54) |
| `RFC4106-10-1` | Thus, to avoid counter block collisions, ESP implementations that permit use of the same key for encrypting and decrypting packets with the same peer MUST ensure that the two peers assign different salt values to the security association (SA). (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4106DirectionsTakeDifferentSalts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L78). **negative:** no negative test. **{single-polarity}:** every AES-GCM Child SA takes its two keys from consecutive runs of one prf+ stream, so there is no input for which the two directions may share a salt and no contrasting case to test |
| `RFC4106-10-2` | To protect against this problem, implementations MUST generate a fresh key before encrypting 2^64 blocks of data with a given key. (§10) | MUST | 10 | **positive:** `unit/verify` [`TestRFC4106GCMKeyCannotOutliveTheSequenceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L85). **negative:** no negative test. **{single-polarity}:** the bound is a property of every state xfrmStateFromParams builds (sequence rollover forbidden, no ESN, so at most 2^32 packets of at most 4096 blocks per key), and no state exists without it to contrast |
| `RFC4106-3.2-1` | Implementations that do not seek to hide the length of the plaintext SHOULD use the minimum amount of padding required, which will be less than four octets. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-4-1` | The salt SHOULD be unpredictable (i.e., chosen at random) before it is selected, but need not be secret. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-8.2-1` | Implementations SHOULD use an IKE Phase 1 cipher that is at least as strong as AES-GCM. (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-8.2-2` | The use of AES CBC [RFC3602] with the same key size used by AES-GCM-ESP is RECOMMENDED. (§8.2) | RECOMMENDED | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4106-10-3` | Implementations SHOULD use as large a size as reasonable. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4106-2-1`](#rfc4106-2-1) The same nonce and key combination MUST NOT be used more than once. (§2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)) and its seqiv IV generator, linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the datapath chooses every IV, so no value ze writes decides the nonce directly; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it from cycling is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) and rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) |
| [`RFC4106-3.1-1`](#rfc4106-3.1-1) The AES-GCM-ESP IV field MUST be eight octets. (§3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)), linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams names the algorithm and the kernel writes the IV field of every packet |
| [`RFC4106-3.1-2`](#rfc4106-3.1-2) For a given key, the IV MUST NOT repeat. (§3.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc4106(gcm(aes)) and its seqiv IV generator, linux:CONFIG_CRYPTO_GCM; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs no IV, so the kernel generates each one; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it from repeating is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) and rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4106-2-1`](#rfc4106-2-1)

The same nonce and key combination MUST NOT be used more than once. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4106-2-1, so there is no linked test.

### [`RFC4106-2-2`](#rfc4106-2-2)

For safety's sake, implementations MUST use an automated key management system, such as the Internet Key Exchange (IKE) [RFC2409], to ensure that this requirement is met. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestManualKeyingRefusesCounterModeCiphers`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/ipsec_manual_counter_mode_test.go#L66) | unit/verify | revert, verified |

### [`RFC4106-3.1-1`](#rfc4106-3.1-1)

The AES-GCM-ESP IV field MUST be eight octets. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4106-3.1-1, so there is no linked test.

### [`RFC4106-3.1-2`](#rfc4106-3.1-2)

For a given key, the IV MUST NOT repeat. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4106-3.1-2, so there is no linked test.

### [`RFC4106-6-1`](#rfc4106-6-1)

Implementations MUST support a full-length 16-octet ICV, and MAY support 8 or 12 octet ICVs, and MUST NOT support other ICV lengths. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4106GCMStateCarriesTheFullICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L40) | unit/verify | revert, verified |

### [`RFC4106-8.1-1`](#rfc4106-8.1-1)

The size of the KEYMAT for the AES-GCM-ESP MUST be four octets longer than is needed for the associated AES key. (§8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC4106KeymatCarriesTheSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L41) | unit/verify | revert, verified |

### [`RFC4106-10-1`](#rfc4106-10-1)

Thus, to avoid counter block collisions, ESP implementations that permit use of the same key for encrypting and decrypting packets with the same peer MUST ensure that the two peers assign different salt values to the security association (SA). (§10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC4106DirectionsTakeDifferentSalts`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc4106_gcm_keymat_test.go#L78) | unit/verify | revert, verified |

### [`RFC4106-10-2`](#rfc4106-10-2)

To protect against this problem, implementations MUST generate a fresh key before encrypting 2^64 blocks of data with a given key. (§10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC4106GCMKeyCannotOutliveTheSequenceSpace`](https://github.com/ze-software/ze/blob/main/internal/component/ike/dataplane/rfc4106_gcm_install_linux_test.go#L85) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4106.txt |
| Source fingerprint | c1d66da1e4b757c6 |
| Record | rfc/extraction/rfc4106.json |
| Mapped sentences | 8 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 1 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `8.3` | not stated | 0 | reviewed | not stated |
| `8.4` | not stated | 2 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 2 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `13` | acknowledgements | 0 | skipped (acknowledgements) | acknowledgements |
| `14` | normative references | 0 | skipped (references) | normative references |
| `15` | informative references | 0 | skipped (references) | informative references |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IKEv1 Phase 2 negotiator: Section 8.4 states the Key Length attribute of the IKE Phase 2 exchange [RFC2407], which is IKEv1. ze implements IKEv2 only (internal/component/ike/engine), whose Key Length attribute RFC 7296 Section 3.3.5 governs | Because the AES supports three key lengths, the Key Length attribute MUST be specified in the IKE Phase 2 exchange [RFC2407]. |
| `8.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IKEv1 Phase 2 negotiator: Section 8.4 states the Key Length attribute of the IKE Phase 2 exchange [RFC2407], which is IKEv1. ze implements IKEv2 only (internal/component/ike/engine), whose Key Length attribute RFC 7296 Section 3.3.5 governs | The Key Length attribute MUST have a value of 128, 192, or 256. |

## Superseded

No document obsoletes RFC 4106, so its obligations are stated where they were written.
