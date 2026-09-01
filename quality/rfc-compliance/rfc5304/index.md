# RFC 5304 - IS-IS Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5304, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 87.5% | 7 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.5% | 1 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5304.md` |
| Requirement shard | `rfc/requirements/rfc5304.md` |
| RFC text | `rfc/full/rfc5304.txt` |

## Enrolment

Enrolled: IS-IS Cryptographic Authentication (HMAC-MD5, Authentication TLV type 54): nine MUST-level requirements. Eight are met with positive+negative tags in internal/plugins/isis: 2-1 and 2-2 (Level-1 area and Level-2 domain authentication with the correct key chain), 2-3 (Hello/IIH link-level authentication), 2-6 (reject a PDU whose HMAC does not match), 2-7 (a signed purge carries only the Authentication TLV), 2-8 (do not accept an unauthenticated purge under configured auth), and 2-9 (reject a purge carrying extra TLVs). 2-4 (sign over the padded PDU) is {single-polarity: positive} with a new test capturing the bytes handed to the signer. 2-5 (do not include the auth value in the per-PDU Checksum TLV) is {not-applicable}: ze implements no optional IS-IS per-PDU Checksum TLV (no TLV 12 codec), so the conditional MUST NOT is vacuous.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Interface and level key-chain authentication.

**What the ledger says remains:**

Same IS-IS experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC5304-2-1`](#rfc5304-2-1), [`RFC5304-2-2`](#rfc5304-2-2), [`RFC5304-2-3`](#rfc5304-2-3), [`RFC5304-2-6`](#rfc5304-2-6), [`RFC5304-2-7`](#rfc5304-2-7), [`RFC5304-2-8`](#rfc5304-2-8), [`RFC5304-2-9`](#rfc5304-2-9)

**Annotated instead of tested (2):** [`RFC5304-2-4`](#rfc5304-2-4), [`RFC5304-2-5`](#rfc5304-2-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5304-2-1` | Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L108). **negative:** `unit/verify` [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L201) |
| `RFC5304-2-2` | Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L111). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L283) |
| `RFC5304-2-3` | IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L156). **negative:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L168) |
| `RFC5304-2-4` | The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2) | SHALL | 2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L68). **negative:** no negative test. **{single-polarity}:** the LAN/P2P hello is padded then signed in one straight-line path (internal/plugins/isis/circuit/runtime.go:273-276), and there is no unpadded-sign code path to drive a negative |
| `RFC5304-2-5` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent is false -- ze implements no optional IS-IS per-PDU/SNP Checksum TLV (internal/plugins/isis/packet has no TLV 12 codec; only the LSP header Fletcher checksum field exists), so this conditional MUST NOT has no applicable code path |
| `RFC5304-2-6` | An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131). **negative:** `unit/verify` [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) |
| `RFC5304-2-7` | ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2) | MUST | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) |
| `RFC5304-2-8` | ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) |
| `RFC5304-2-9` | ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449). **negative:** `unit/verify` [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) |
| `RFC5304-2.1-1` | If an inbound LSP with an authentication failure has the local System ID and a higher Sequence Number than the IS-IS process has, the process SHOULD increase its own LSP Sequence Number accordingly and re-flood the LSPs (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-10` | The Link Level Authentication String used by IS-IS Hello PDUs MAY be different from that of Link State PDUs (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-11` | An implementation MAY have a transition mode where it includes HMAC-MD5 Authentication Information in PDUs but does not verify the HMAC-MD5 Authentication Information (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-12` | An implementation MAY check a set of passwords when verifying the Authentication Value (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5304-2-13` | An implementation that does not implement HMAC-MD5 authentication MAY accept a PDU that contains the HMAC-MD5 Authentication Type (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5304-2-5`](#rfc5304-2-5) Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent is false -- ze implements no optional IS-IS per-PDU/SNP Checksum TLV (internal/plugins/isis/packet has no TLV 12 codec; only the LSP header Fletcher checksum field exists), so this conditional MUST NOT has no applicable code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5304-2-1`](#rfc5304-2-1)

Level 1 Sequence Number PDUs SHALL use the Area Authentication string as in Level 1 Link State PDUs (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L201) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L108) | unit/verify | unproven |

### [`RFC5304-2-2`](#rfc5304-2-2)

Level 2 Sequence Number PDUs SHALL use the domain authentication string as in Level 2 Link State PDUs (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L283) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L111) | unit/verify | unproven |

### [`RFC5304-2-3`](#rfc5304-2-3)

IS-IS Hello PDUs SHALL use the Link Level Authentication String (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L168) | unit/verify | unproven |
| positive | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L156) | unit/verify | unproven |

### [`RFC5304-2-4`](#rfc5304-2-4)

The HMAC-MD5 result for the IS-IS Hello PDUs SHALL be calculated after the packet is padded to the MTU size, if padding is not disabled (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L68) | unit/verify | unproven |

### [`RFC5304-2-5`](#rfc5304-2-5)

Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS Hello PDUs MUST NOT include the Checksum TLV (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5304-2-5, so no unit is bound to it.

### [`RFC5304-2-6`](#rfc5304-2-6)

An implementation that implements HMAC-MD5 authentication and receives HMAC-MD5 Authentication Information MUST discard the PDU if the Authentication Value is incorrect (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthConstantTimeCompare`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L608) | unit/verify | unproven |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L635) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACMD5`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L131) | unit/verify | unproven |

### [`RFC5304-2-7`](#rfc5304-2-7)

ISes (routers) that implement HMAC-MD5 authentication and initiate LSP purges MUST remove the body of the LSP and add the authentication TLV (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L466) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L455) | unit/verify | unproven |

### [`RFC5304-2-8`](#rfc5304-2-8)

ISes implementing HMAC-MD5 authentication MUST NOT accept unauthenticated purges (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L464) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L447) | unit/verify | unproven |

### [`RFC5304-2-9`](#rfc5304-2-9)

ISes MUST NOT accept purges that contain TLVs other than the authentication TLV (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L483) | unit/verify | unproven |
| positive | [`TestISISAuthPurge`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L449) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5304, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5304, so its obligations are stated where they were written.
