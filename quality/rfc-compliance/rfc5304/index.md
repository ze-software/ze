# RFC 5304 - IS-IS Cryptographic Authentication

IS-IS HMAC-MD5 authentication. Every requirement listed for RFC 5304, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 16 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | IS-IS HMAC-MD5 authentication |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tests linked to requirements | 16 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5304.md` |
| Requirement table | `rfc/requirements/rfc5304.md` |
| RFC text | `rfc/full/rfc5304.txt` |

## Automatic checks

Included in automatic checks: IS-IS Cryptographic Authentication (HMAC-MD5, Authentication TLV type 54): nine MUST-level requirements. Eight are met with positive+negative tags in internal/plugins/isis: 2-1 and 2-2 (Level-1 area and Level-2 domain authentication with the correct key chain), 2-3 (Hello/IIH link-level authentication), 2-6 (reject a PDU whose HMAC does not match), 2-7 (a signed purge carries only the Authentication TLV), 2-8 (do not accept an unauthenticated purge under configured auth), and 2-9 (reject a purge carrying extra TLVs). 2-4 (sign over the padded PDU) is {single-polarity: positive} with a new test capturing the bytes handed to the signer. 2-5 (do not include the auth value in the per-PDU Checksum TLV) is a gap in proof: ze implements no optional IS-IS per-PDU Checksum TLV (no TLV 12 codec), so no PDU carries one, and no test asserts it.

## Published support status

**Area:** IS-IS HMAC-MD5 authentication

**Test verification:** 0 of 9 MUST proven

**Evidence:** -

**Unmet:** RFC5304-2-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC5304-2-1`](#rfc5304-2-1), [`RFC5304-2-2`](#rfc5304-2-2), [`RFC5304-2-3`](#rfc5304-2-3), [`RFC5304-2-6`](#rfc5304-2-6), [`RFC5304-2-7`](#rfc5304-2-7), [`RFC5304-2-8`](#rfc5304-2-8), [`RFC5304-2-9`](#rfc5304-2-9)

**Annotated (including scoped evidence) (2):** [`RFC5304-2-4`](#rfc5304-2-4), [`RFC5304-2-5`](#rfc5304-2-5)

## Figures

- **MUST bar**: Tests awaiting verification 8, Not built 1
- **Overview bar**: Tests awaiting verification 8, Not assessed 5, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 8 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
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

Source: [data/rfc/rfc5304.json](../../../data/rfc/rfc5304.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5304-2-1` | Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L106). **negative:** `unit/verify` [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L188) |
| `RFC5304-2-2` | Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L109). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L267) |
| `RFC5304-2-3` | IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L150). **negative:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158) |
| `RFC5304-2-4` | The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L69). **negative:** no negative test. **{single-polarity}:** the LAN/P2P hello is padded then signed in one straight-line path (internal/plugins/isis/circuit/runtime.go:273-276), and there is no unpadded-sign code path to drive a negative |
| `RFC5304-2-5` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/isis/packet meets it, since ze implements no optional Checksum TLV (TLV 12) codec, so no SNP or IIH it builds carries one; no test asserts it |
| `RFC5304-2-6` | An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) |
| `RFC5304-2-7` | ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) |
| `RFC5304-2-8` | ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) |
| `RFC5304-2-9` | ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) |
| `RFC5304-2.1-1` | One possible solution to this problem is for the IS-IS process to detect if any inbound LSP with an authentication failure has the local System ID and also has a higher Sequence Number than the IS-IS process has. In this event, the IS-IS process SHOULD increase its own LSP Sequence Number accordingly and re-flood the LSPs. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-10` | IS-IS Hello PDUs SHALL use the Link Level Authentication String, which MAY be different from that of Link State PDUs. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-11` | An implementation MAY have a transition mode where it includes HMAC- MD5 Authentication Information in PDUs but does not verify the HMAC- MD5 Authentication Information. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-12` | An implementation MAY check a set of passwords when verifying the Authentication Value (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-13` | An implementation that does not implement HMAC-MD5 authentication MAY accept a PDU that contains the HMAC-MD5 Authentication Type (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5304-2-5`](#rfc5304-2-5) Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | {gap}, no test | gap in proof: internal/plugins/isis/packet meets it, since ze implements no optional Checksum TLV (TLV 12) codec, so no SNP or IIH it builds carries one; no test asserts it |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5304-2-1`](#rfc5304-2-1)

Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L188) | unit/verify | not verified |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L106) | unit/verify | not verified |

### [`RFC5304-2-2`](#rfc5304-2-2)

Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L267) | unit/verify | not verified |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L109) | unit/verify | not verified |

### [`RFC5304-2-3`](#rfc5304-2-3)

IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158) | unit/verify | not verified |
| positive | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L150) | unit/verify | not verified |

### [`RFC5304-2-4`](#rfc5304-2-4)

The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L69) | unit/verify | not verified |

### [`RFC5304-2-5`](#rfc5304-2-5)

Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5304-2-5, so there is no linked test.

### [`RFC5304-2-6`](#rfc5304-2-6)

An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608) | unit/verify | not verified |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) | unit/verify | not verified |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131) | unit/verify | not verified |

### [`RFC5304-2-7`](#rfc5304-2-7)

ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) | unit/verify | not verified |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455) | unit/verify | not verified |

### [`RFC5304-2-8`](#rfc5304-2-8)

ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) | unit/verify | not verified |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447) | unit/verify | not verified |

### [`RFC5304-2-9`](#rfc5304-2-9)

ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) | unit/verify | not verified |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449) | unit/verify | not verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5304.txt |
| Source fingerprint | 125142e8956ce6f3 |
| Record | rfc/extraction/rfc5304.json |
| Mapped sentences | 9 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 9 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 5304 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 5304, so its obligations are stated where they were written.
