# RFC 5282 - Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol

Supported. Every requirement this repository extracted from RFC 5282, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 78.9% | 15 of 19 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 19 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 19 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 30 of 30 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 19 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 19 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 19 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 21.1% | 4 of 19 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 19 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 30 |
| Gated MUST-level | 19 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 4 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tagged units | 30 |
| Recorded audit verdicts | 0 |
| Discrimination records | 30 |
| Summary | `rfc/short/rfc5282.md` |
| Requirement shard | `rfc/requirements/rfc5282.md` |
| RFC text | `rfc/full/rfc5282.txt` |

## Enrolment

Enrolled: Using AEAD Algorithms with the IKEv2 Encrypted Payload (RFC 5282): ze speaks one AEAD cipher for the IKE SA, ENCR_AES_GCM_16 (Transform ID 20) at 128 and 256 bits. buildSKMessageAEADWithMsgID (engine/auth.go) seals an 8-octet IV, a salt-then-IV 12-octet nonce and the IKE-header-through-SK-generic-header associated data; decryptSKPayload and crypto.DecryptIKEAEAD (crypto/cipher.go) open it. encKeyMaterialLen (crypto/keys.go) gives SK_ei and SK_er the RFC 4106 KEYMAT layout (cipher key then 4-octet salt) and leaves SK_ai and SK_ar at zero octets. No AES CCM: aeadSaltBytes holds one entry and specifiedEncryption (crypto/proposal.go) accepts no CCM transform off the wire. Of the 19 gated rows, every row that binds an AES GCM implementation now carries a both-polarity tagged test and a discrimination record. The AES CCM rows (RFC5282-3.2-3, RFC5282-3.2-4, RFC5282-4-3, RFC5282-4-4) carry neither test nor annotation: ze implements no AES CCM, so no code produces the behavior, and ai/rules/rfc-compliance.md reserves gap, not-applicable and feature-declined for an owner answer.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

AES-GCM IKEv2 AEAD encryption and decryption framing: the eight-octet IV, the salt-then-IV twelve-octet nonce, the full-length sixteen-octet ICV, the associated data from the fixed header through the Encrypted payload's own generic header, any Padding up to 255 octets on receipt, the KEYMAT layout of SK_ei and SK_er, and the negotiation rules that carry the Key Length attribute and refuse an integrity transform.

**What the ledger says remains**

AES CCM is not implemented. specifiedEncryption (crypto/proposal.go) accepts no CCM transform off the wire and aeadSaltBytes (crypto/transform.go) holds one entry, so the AES CCM rows of the checklist name behavior no layer performs and carry no test. Whether they are conformance debt or an optional feature ze declined is an owner answer, not this summary's.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 4 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC5282-3-1`](#rfc5282-3-1), [`RFC5282-3.1-1`](#rfc5282-3.1-1), [`RFC5282-3.1-2`](#rfc5282-3.1-2), [`RFC5282-3.2-1`](#rfc5282-3.2-1), [`RFC5282-3.2-2`](#rfc5282-3.2-2), [`RFC5282-4-1`](#rfc5282-4-1), [`RFC5282-4-2`](#rfc5282-4-2), [`RFC5282-5.1-1`](#rfc5282-5.1-1), [`RFC5282-5.1-2`](#rfc5282-5.1-2), [`RFC5282-7.1-1`](#rfc5282-7.1-1), [`RFC5282-7.1-2`](#rfc5282-7.1-2), [`RFC5282-7.3-1`](#rfc5282-7.3-1), [`RFC5282-7.3-2`](#rfc5282-7.3-2), [`RFC5282-8-1`](#rfc5282-8-1), [`RFC5282-8-2`](#rfc5282-8-2)

**No test and no annotation (4):** [`RFC5282-3.2-3`](#rfc5282-3.2-3), [`RFC5282-3.2-4`](#rfc5282-3.2-4), [`RFC5282-4-3`](#rfc5282-4-3), [`RFC5282-4-4`](#rfc5282-4-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5282-3-1` | The recipient MUST accept any amount of Padding up to 255 octets (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5282AEADReceiveAcceptsAnyPaddingTo255`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L255). **negative:** `unit/verify` [`TestRFC5282AEADReceiveRefusesATruncatedInnerPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L291) |
| `RFC5282-3.1-1` | The Initialization Vector MUST be eight octets (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5282AEADSendIVIsEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L314). **negative:** `unit/verify` [`TestRFC5282AEADReceiveRefusesAnIVThatIsNotEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L358) |
| `RFC5282-3.1-2` | The IV MUST be chosen by the encryptor in a manner that ensures the same IV value is used only once for a given key (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5282AEADSendIVIsUniquePerKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L381). **negative:** `unit/verify` [`TestRFC5282AEADSendIVDoesNotRepeatForARepeatedMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L413) |
| `RFC5282-3.2-1` | AES GCM implementations MUST support a full-length 16 octet ICV (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5282AEADSendICVIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L440). **negative:** `unit/verify` [`TestRFC5282AEADReceiveRefusesADamagedICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L467) |
| `RFC5282-3.2-2` | AES GCM implementations MUST NOT support ICV lengths other than 16, 8 and 12 octets (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC5282AEADRefusesAForbiddenICVLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L53). **negative:** `unit/verify` [`TestRFC5282AEADOpensTheFullLengthICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L81) |
| `RFC5282-3.2-3` | AES CCM implementations MUST support ICV sizes of 8 octets and 16 octets (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-3.2-4` | AES CCM implementations MUST NOT support ICV lengths other than 8, 16 and 12 octets (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-4-1` | For AES GCM the default nonce format MUST be used, the salt concatenated with the IV in that order (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5282AEADNonceIsSaltThenIV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L496). **negative:** `unit/verify` [`TestRFC5282AEADRefusesAReversedNonce`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L519) |
| `RFC5282-4-2` | For AES GCM a 12 octet nonce MUST be used (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC5282AEADNonceIsTwelveOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L537). **negative:** `unit/verify` [`TestRFC5282AEADRefusesANonceThatIsNotTwelveOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L564) |
| `RFC5282-4-3` | For AES CCM the default nonce format MUST be used, the salt concatenated with the IV in that order (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-4-4` | For AES CCM an 11 octet nonce MUST be used (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-5.1-1` | The associated data MUST consist of the message from the first octet of the Fixed IKE Header through the last octet of the Encrypted Payload's Payload Header, including any payloads between them (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC5282AEADAssociatedDataCoversAnInterveningPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L585). **negative:** `unit/verify` [`TestRFC5282AEADRefusesAnAlteredInterveningPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L617) |
| `RFC5282-5.1-2` | The Initialization Vector and Ciphertext fields MUST NOT be included in the associated data (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestRFC5282AEADAssociatedDataExcludesTheIVAndCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L641). **negative:** `unit/verify` [`TestRFC5282AEADRefusesAssociatedDataCoveringTheIV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L662) |
| `RFC5282-7.1-1` | With an AEAD cipher the SK_ai and SK_ar integrity keys are unused, and each MUST be treated as having a size of zero octets (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC5282AEADIntegrityKeysAreZeroOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L123). **negative:** `unit/verify` [`TestRFC5282NonAEADIntegrityKeysAreDerived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L153) |
| `RFC5282-7.1-2` | Each of SK_ei and SK_er MUST have the size and format of the KEYMAT for the AES key size in use, the cipher key followed by the salt (§7.1) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC5282AEADEncryptionKeysCarryTheirSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L169). **negative:** `unit/verify` [`TestRFC5282NonAEADEncryptionKeysCarryNoSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L230) |
| `RFC5282-7.3-1` | The Key Length attribute MUST be specified whenever an AES GCM or AES CCM encryption transform identifier is used (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC5282AEADProposalCarriesTheKeyLengthAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L677). **negative:** `unit/verify` [`TestRFC5282AEADOfferWithoutAKeyLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L258) |
| `RFC5282-7.3-2` | The Key Length attribute MUST have a value of 128, 192, or 256 (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC5282AEADKeyLengthAcceptsTheThreeValues`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L277). **negative:** `unit/verify` [`TestRFC5282AEADKeyLengthRefusesEveryOtherValue`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L297) |
| `RFC5282-8-1` | When an authenticated encryption algorithm is selected as the encryption algorithm for any SA, an integrity algorithm MUST NOT be selected for that SA (§8) | MUST NOT | 8 | **positive:** `unit/verify` [`TestRFC5282AEADSelectionCarriesNoIntegrityAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L739). **negative:** `unit/verify` [`TestRFC5282AEADOfferWithIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L773) |
| `RFC5282-8-2` | If all of the encryption algorithms in a proposal are authenticated encryption algorithms, the proposal MUST NOT propose any integrity transforms (§8) | MUST NOT | 8 | **positive:** `unit/verify` [`TestRFC5282AEADIKEProposalCarriesNoIntegrityTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_proposal_test.go#L37). **negative:** `unit/verify` [`TestRFC5282NonAEADIKEProposalKeepsItsIntegrityTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_proposal_test.go#L74) |
| `RFC5282-4-5` | Specific authenticated encryption algorithms SHOULD use the default nonce format (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-7.2-1` | A 16-octet ICV size SHOULD be used with IKEv2 (§7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-7.2-2` | The use of 12-octet ICVs, transform identifiers 15 and 19, is discouraged (§7.2) | NOT RECOMMENDED | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-7.2-3` | If an ICV size larger than 8 octets is appropriate, 16-octet ICVs SHOULD be used (§7.2) | SHOULD | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-7.3-3` | The use of the Key Length value 192 is discouraged (§7.3) | NOT RECOMMENDED | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-7.3-4` | If an AES key larger than 128 bits is appropriate, a 256-bit AES key SHOULD be used (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-3-2` | Padding MAY contain any value chosen by the sender (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-3.1-3` | The encryptor MAY generate the IV in any manner that ensures uniqueness (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-3.2-5` | AES GCM implementations MAY support 8 or 12 octet ICVs (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-3.2-6` | AES CCM implementations MAY also support 12 octet ICVs (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5282-4-6` | Specific authenticated encryption algorithms MAY use different nonce formats (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5282-3.2-3`](#rfc5282-3.2-3) AES CCM implementations MUST support ICV sizes of 8 octets and 16 octets (§3.2) | no test | no test carries this requirement id |
| [`RFC5282-3.2-4`](#rfc5282-3.2-4) AES CCM implementations MUST NOT support ICV lengths other than 8, 16 and 12 octets (§3.2) | no test | no test carries this requirement id |
| [`RFC5282-4-3`](#rfc5282-4-3) For AES CCM the default nonce format MUST be used, the salt concatenated with the IV in that order (§4) | no test | no test carries this requirement id |
| [`RFC5282-4-4`](#rfc5282-4-4) For AES CCM an 11 octet nonce MUST be used (§4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5282-3-1`](#rfc5282-3-1)

The recipient MUST accept any amount of Padding up to 255 octets (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADReceiveRefusesATruncatedInnerPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L291) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADReceiveAcceptsAnyPaddingTo255`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L255) | unit/verify | revert, verified |

### [`RFC5282-3.1-1`](#rfc5282-3.1-1)

The Initialization Vector MUST be eight octets (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADReceiveRefusesAnIVThatIsNotEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L358) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADSendIVIsEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L314) | unit/verify | revert, verified |

### [`RFC5282-3.1-2`](#rfc5282-3.1-2)

The IV MUST be chosen by the encryptor in a manner that ensures the same IV value is used only once for a given key (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADSendIVDoesNotRepeatForARepeatedMessage`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L413) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADSendIVIsUniquePerKey`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L381) | unit/verify | revert, verified |

### [`RFC5282-3.2-1`](#rfc5282-3.2-1)

AES GCM implementations MUST support a full-length 16 octet ICV (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADReceiveRefusesADamagedICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L467) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADSendICVIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L440) | unit/verify | revert, verified |

### [`RFC5282-3.2-2`](#rfc5282-3.2-2)

AES GCM implementations MUST NOT support ICV lengths other than 16, 8 and 12 octets (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADOpensTheFullLengthICV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L81) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADRefusesAForbiddenICVLength`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L53) | unit/verify | revert, verified |

### [`RFC5282-3.2-3`](#rfc5282-3.2-3)

AES CCM implementations MUST support ICV sizes of 8 octets and 16 octets (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5282-3.2-3, so no unit is bound to it.

### [`RFC5282-3.2-4`](#rfc5282-3.2-4)

AES CCM implementations MUST NOT support ICV lengths other than 8, 16 and 12 octets (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5282-3.2-4, so no unit is bound to it.

### [`RFC5282-4-1`](#rfc5282-4-1)

For AES GCM the default nonce format MUST be used, the salt concatenated with the IV in that order (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADRefusesAReversedNonce`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L519) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADNonceIsSaltThenIV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L496) | unit/verify | revert, verified |

### [`RFC5282-4-2`](#rfc5282-4-2)

For AES GCM a 12 octet nonce MUST be used (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADRefusesANonceThatIsNotTwelveOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L564) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADNonceIsTwelveOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L537) | unit/verify | revert, verified |

### [`RFC5282-4-3`](#rfc5282-4-3)

For AES CCM the default nonce format MUST be used, the salt concatenated with the IV in that order (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5282-4-3, so no unit is bound to it.

### [`RFC5282-4-4`](#rfc5282-4-4)

For AES CCM an 11 octet nonce MUST be used (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5282-4-4, so no unit is bound to it.

### [`RFC5282-5.1-1`](#rfc5282-5.1-1)

The associated data MUST consist of the message from the first octet of the Fixed IKE Header through the last octet of the Encrypted Payload's Payload Header, including any payloads between them (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADRefusesAnAlteredInterveningPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L617) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADAssociatedDataCoversAnInterveningPayload`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L585) | unit/verify | revert, verified |

### [`RFC5282-5.1-2`](#rfc5282-5.1-2)

The Initialization Vector and Ciphertext fields MUST NOT be included in the associated data (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADRefusesAssociatedDataCoveringTheIV`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L662) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADAssociatedDataExcludesTheIVAndCiphertext`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L641) | unit/verify | revert, verified |

### [`RFC5282-7.1-1`](#rfc5282-7.1-1)

With an AEAD cipher the SK_ai and SK_ar integrity keys are unused, and each MUST be treated as having a size of zero octets (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282NonAEADIntegrityKeysAreDerived`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L153) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADIntegrityKeysAreZeroOctets`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L123) | unit/verify | revert, verified |

### [`RFC5282-7.1-2`](#rfc5282-7.1-2)

Each of SK_ei and SK_er MUST have the size and format of the KEYMAT for the AES key size in use, the cipher key followed by the salt (§7.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282NonAEADEncryptionKeysCarryNoSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L230) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADEncryptionKeysCarryTheirSalt`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L169) | unit/verify | revert, verified |

### [`RFC5282-7.3-1`](#rfc5282-7.3-1)

The Key Length attribute MUST be specified whenever an AES GCM or AES CCM encryption transform identifier is used (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADOfferWithoutAKeyLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADProposalCarriesTheKeyLengthAttribute`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L677) | unit/verify | revert, verified |

### [`RFC5282-7.3-2`](#rfc5282-7.3-2)

The Key Length attribute MUST have a value of 128, 192, or 256 (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADKeyLengthRefusesEveryOtherValue`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L297) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADKeyLengthAcceptsTheThreeValues`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/rfc5282_aead_test.go#L277) | unit/verify | revert, verified |

### [`RFC5282-8-1`](#rfc5282-8-1)

When an authenticated encryption algorithm is selected as the encryption algorithm for any SA, an integrity algorithm MUST NOT be selected for that SA (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282AEADOfferWithIntegrityIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L773) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADSelectionCarriesNoIntegrityAlgorithm`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_sk_test.go#L739) | unit/verify | revert, verified |

### [`RFC5282-8-2`](#rfc5282-8-2)

If all of the encryption algorithms in a proposal are authenticated encryption algorithms, the proposal MUST NOT propose any integrity transforms (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5282NonAEADIKEProposalKeepsItsIntegrityTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_proposal_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC5282AEADIKEProposalCarriesNoIntegrityTransform`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc5282_aead_proposal_test.go#L37) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude-opus-5 (ipsecwalk), spec-rfcgate-6-supported-extraction-signoff |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc5282.txt |
| Source fingerprint | e4b60d44518cbcd7 |
| Record | rfc/extraction/rfc5282.json |
| Mapped sentences | 16 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Status of This Memo, Abstract and Table of Contents | 0 | skipped (front-matter) | Status of This Memo, Abstract and Table of Contents. |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 2 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 2 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 2 | walked | not stated |
| `8` | not stated | 3 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.1.1` | not stated | 0 | walked | not stated |
| `10.1.2` | not stated | 0 | walked | not stated |
| `10.1.3` | not stated | 0 | walked | not stated |
| `10.1.4` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `10.2.1` | not stated | 0 | walked | not stated |
| `10.2.2` | not stated | 0 | walked | not stated |
| `10.2.3` | not stated | 0 | walked | not stated |
| `10.2.4` | not stated | 0 | walked | not stated |
| `10.2.5` | not stated | 0 | walked | not stated |
| `10.2.6` | not stated | 0 | walked | not stated |
| `10.3` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 1 | skipped (iana) | IANA Considerations: the transform identifiers were already assigned for ESP, and the AEAD registry rows are a registry action rather than an implementation obligation. |
| `13` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `14` | References | 0 | skipped (references) | References. |
| `14.1` | Normative References | 0 | skipped (references) | Normative References. |
| `14.2` | Informative References | 1 | skipped (references) | Informative References. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence states what RFC 4306 Section 3.3.3 requires, so the obligation it carries belongs to that document; it is quoted here only as the rule the next two sentences update. Sites 8:2 and 8:3 carry the update this document makes, and both are mapped. | IKEv2 (Section 3.3.3 of [RFC4306]) specifies that both an encryption algorithm and an integrity checking algorithm are required for an IKE SA (Security Association). |
| `12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA statement that no registry action is needed for the transform identifiers this document reuses. It imposes nothing on an implementation, and the lowercase 'required' the prose scan matched is the report of an absence. | No IANA actions are required for this usage extension. |
| `14.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IPR boilerplate reproduced from the informative reference section. It addresses 'any interested party' about patent disclosure and states no protocol behavior. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 5282, so its obligations are stated where they were written.
