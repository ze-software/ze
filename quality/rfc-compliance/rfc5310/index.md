# RFC 5310 - IS-IS Generic Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5310, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 4 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 4 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 12 |
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
| Summary | `rfc/short/rfc5310.md` |
| Requirement shard | `rfc/requirements/rfc5310.md` |
| RFC text | `rfc/full/rfc5310.txt` |

## Enrolment

Enrolled: IS-IS Generic Cryptographic Authentication (HMAC-SHA, Authentication TLV type 3): nine MUST-level requirements. Eight are met with tags in internal/plugins/isis (sharing the auth backend enrolled for RFC 5304): 3.2-1 and 3.2-2 (Level-1 area and Level-2 domain authentication with the correct key chain), 3.2-3 (Hello/IIH link-level authentication), and 4-2 (accept a PDU signed by any currently-valid key during a key rollover) carry positive+negative tags; 3.2-5 (sign over the padded PDU), 3.4-1 (the HMAC pre-image includes the auth-type byte and TLV length with Apad in the value region), 3.4-2 (pad then sign), and 4-1 (the LSP HMAC excludes the Checksum and Remaining Lifetime) are {single-polarity: positive} (no reject path exists for these construction properties). 3.2-6 (do not include the auth value in the optional per-PDU Checksum TLV) is {not-applicable}: ze implements no such optional Checksum TLV.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

HMAC-SHA authentication path.

**What the ledger says remains:**

Same IS-IS experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5310-3.2-1`](#rfc5310-3.2-1), [`RFC5310-3.2-2`](#rfc5310-3.2-2), [`RFC5310-3.2-3`](#rfc5310-3.2-3), [`RFC5310-4-2`](#rfc5310-4-2)

**Annotated instead of tested (5):** [`RFC5310-3.2-5`](#rfc5310-3.2-5), [`RFC5310-3.2-6`](#rfc5310-3.2-6), [`RFC5310-3.4-1`](#rfc5310-3.4-1), [`RFC5310-3.4-2`](#rfc5310-3.4-2), [`RFC5310-4-1`](#rfc5310-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5310-3.2-1` | Level 1 Sequence Number PDUs "SHALL use the Area Authentication string, as in Level 1 Link State PDUs" (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L114). **negative:** `unit/verify` [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L204) |
| `RFC5310-3.2-2` | Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L118). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L286) |
| `RFC5310-3.2-3` | "IS-IS HELLO PDUs SHALL use the Link Level Authentication string" (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158). **negative:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L170) |
| `RFC5310-3.2-5` | "The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled" (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L71). **negative:** no negative test. **{single-polarity}:** ze always pads the IIH before signing it (circuit/runtime.go:273-276 for LAN, :293-304 for P2P), so there is no sign-before-pad code path and no negative (unpadded-sign) behavior to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the bytes handed to the signer are already padded to MTU-LLC and carry Padding TLV 8 |
| `RFC5310-3.2-6` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs "MUST NOT include the Checksum TLV" (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent is false -- ze implements no optional IS-IS per-PDU Checksum TLV (internal/plugins/isis/packet/tlv.go has no TLV 12 codec), so this conditional MUST NOT has no applicable code path |
| `RFC5310-3.4-1` | "An implementation MUST fill the authentication type and the length before the authentication data is computed" (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISAuthHMACSHAApadPreimage`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L234). **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACSHA256`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L161). **negative:** no negative test. **{single-polarity}:** ze always builds the Authentication TLV with the auth-type byte and sets the TLV length before the digest is computed (auth_sign.go:47-52), then runs the HMAC over the full signed PDU (auth_sign.go:274-276), so no code path omits the type or length from the pre-image and there is no negative to assert. The positive is proven by the known-answer TestISISAuthHMACSHAApadPreimage (the re-hashed pre-image still carries the type byte and length) and the on-wire type-3 round-trip TestISISAuthSignVerifyHMACSHA256 |
| `RFC5310-3.4-2` | "The authentication data for the IS-IS IIH PDUs MUST be computed after the IS-IS Hello (IIH) has been padded to the MTU size, if padding is not explicitly disabled" (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L74). **negative:** no negative test. **{single-polarity}:** ze pads the IIH before signing it (circuit/runtime.go:273-276 pads then signs), with no sign-before-pad code path, so no negative (auth-computed-before-padding) behavior exists to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the signer receives the PDU already padded to MTU-LLC |
| `RFC5310-4-1` | "the remaining lifetime of the LSP MUST be set to zero before computing the authentication", so that field is not authenticated (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391). **positive:** `unit/verify` [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L666). **negative:** no negative test. **{single-polarity}:** ze always zeroes the Checksum and Remaining Lifetime before the LSP digest (auth_sign.go:268-272), so those fields are never part of the HMAC. The exclusion is observable only as non-rejection -- a post-sign Remaining-Lifetime change still verifies -- and there is no field-included code path that rejects, so no negative exists. The positive is proven in TestISISAuthLSPChecksumAfterSign and the HMAC-SHA-256 type-3 LSP round-trip TestISISAuthRotationOverlapAccepts |
| `RFC5310-4-2` | "implementations MUST be able to store and use more than one key at the same time" (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_keystore_test.go#L78). **positive:** `unit/verify` [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L663). **negative:** `unit/verify` [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L688). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637) |
| `RFC5310-3.5-1` | When the calculated data and the received authentication data do not match, the PDU is discarded and "an error event SHOULD be logged" (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5310-3.2-4` | The IS-IS HELLO PDU Link Level Authentication string "MAY be different from that of Link State PDUs" (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5310-3.5-2` | "An implementation MAY have a transition mode where it includes CRYPTO_AUTH information in the PDUs but does not verify this information" as a migration aid (§3.5) | MAY | 3.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5310-3.2-6`](#rfc5310-3.2-6) Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs "MUST NOT include the Checksum TLV" (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent is false -- ze implements no optional IS-IS per-PDU Checksum TLV (internal/plugins/isis/packet/tlv.go has no TLV 12 codec), so this conditional MUST NOT has no applicable code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5310-3.2-1`](#rfc5310-3.2-1)

Level 1 Sequence Number PDUs "SHALL use the Area Authentication string, as in Level 1 Link State PDUs" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L204) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L114) | unit/verify | unproven |

### [`RFC5310-3.2-2`](#rfc5310-3.2-2)

Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L286) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L118) | unit/verify | unproven |

### [`RFC5310-3.2-3`](#rfc5310-3.2-3)

"IS-IS HELLO PDUs SHALL use the Link Level Authentication string" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L170) | unit/verify | unproven |
| positive | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L158) | unit/verify | unproven |

### [`RFC5310-3.2-5`](#rfc5310-3.2-5)

"The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L71) | unit/verify | unproven |

### [`RFC5310-3.2-6`](#rfc5310-3.2-6)

Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs "MUST NOT include the Checksum TLV" (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5310-3.2-6, so no unit is bound to it.

### [`RFC5310-3.4-1`](#rfc5310-3.4-1)

"An implementation MUST fill the authentication type and the length before the authentication data is computed" (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISAuthHMACSHAApadPreimage`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L234) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACSHA256`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L161) | unit/verify | unproven |

### [`RFC5310-3.4-2`](#rfc5310-3.4-2)

"The authentication data for the IS-IS IIH PDUs MUST be computed after the IS-IS Hello (IIH) has been padded to the MTU size, if padding is not explicitly disabled" (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L74) | unit/verify | unproven |

### [`RFC5310-4-1`](#rfc5310-4-1)

"the remaining lifetime of the LSP MUST be set to zero before computing the authentication", so that field is not authenticated (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391) | unit/verify | unproven |
| positive | [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L666) | unit/verify | unproven |

### [`RFC5310-4-2`](#rfc5310-4-2)

"implementations MUST be able to store and use more than one key at the same time" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L688) | unit/verify | unproven |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637) | unit/verify | unproven |
| positive | [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_keystore_test.go#L78) | unit/verify | unproven |
| positive | [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L663) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5310, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5310, so its obligations are stated where they were written.
