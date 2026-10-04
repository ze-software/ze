# RFC 5310 - IS-IS Generic Cryptographic Authentication

Experimental. Every requirement this repository extracted from RFC 5310, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 44.4% | 4 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 44.4% | 4 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 84.0% | 21 of 25 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.1% | 1 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 12 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 25 |
| Tagged units | 25 |
| Recorded audit verdicts | 8 |
| Discrimination records | 21 |
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
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5310-3.2-1`](#rfc5310-3.2-1), [`RFC5310-3.2-2`](#rfc5310-3.2-2), [`RFC5310-3.2-3`](#rfc5310-3.2-3), [`RFC5310-4-2`](#rfc5310-4-2)

**Annotated (including scoped evidence) (5):** [`RFC5310-3.2-5`](#rfc5310-3.2-5), [`RFC5310-3.2-6`](#rfc5310-3.2-6), [`RFC5310-3.4-1`](#rfc5310-3.4-1), [`RFC5310-3.4-2`](#rfc5310-3.4-2), [`RFC5310-4-1`](#rfc5310-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5310-3.2-1` | When calculating the CRYPTO_AUTH result for Sequence Number PDUs, Level 1 Sequence Number PDUs SHALL use the Area Authentication string, as in Level 1 Link State PDUs. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC5310Level1SNPUsesAreaString`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L49). **negative:** `unit/verify` [`TestRFC5310Level1SNPRefusesOtherStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L66) |
| `RFC5310-3.2-2` | Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L112). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L270) |
| `RFC5310-3.2-3` | IS-IS HELLO PDUs SHALL use the Link Level Authentication string (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC5310HelloUsesLinkString`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L86). **negative:** `unit/verify` [`TestRFC5310HelloRefusesAreaAndDomainStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L103) |
| `RFC5310-3.2-5` | The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L72). **positive:** `unit/verify` [`TestRFC5310P2PHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L159). **negative:** no negative test. **{single-polarity}:** ze always pads the IIH before signing it (circuit/runtime.go:273-276 for LAN, :293-304 for P2P), so there is no sign-before-pad code path and no negative (unpadded-sign) behavior to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the bytes handed to the signer are already padded to MTU-LLC and carry Padding TLV 8 |
| `RFC5310-3.2-6` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs MUST NOT include the Checksum TLV. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent is false -- ze implements no optional IS-IS per-PDU Checksum TLV (internal/plugins/isis/packet/tlv.go has no TLV 12 codec), so this conditional MUST NOT has no applicable code path |
| `RFC5310-3.4-1` | An implementation MUST fill the authentication type and the length before the authentication data is computed. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISAuthHMACSHAApadPreimage`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L234). **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACSHA256`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L161). **negative:** no negative test. **{single-polarity}:** ze always builds the Authentication TLV with the auth-type byte and sets the TLV length before the digest is computed (auth_sign.go:47-52), then runs the HMAC over the full signed PDU (auth_sign.go:274-276), so no code path omits the type or length from the pre-image and there is no negative to assert. The positive is proven by the known-answer TestISISAuthHMACSHAApadPreimage (the re-hashed pre-image still carries the type byte and length) and the on-wire type-3 round-trip TestISISAuthSignVerifyHMACSHA256 |
| `RFC5310-3.4-2` | The authentication data for the IS-IS IIH PDUs MUST be computed after the IS-IS Hello (IIH) has been padded to the MTU size, if padding is not explicitly disabled. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L75). **positive:** `unit/verify` [`TestRFC5310P2PHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L161). **negative:** no negative test. **{single-polarity}:** ze pads the IIH before signing it (circuit/runtime.go:273-276 pads then signs), with no sign-before-pad code path, so no negative (auth-computed-before-padding) behavior exists to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the signer receives the PDU already padded to MTU-LLC |
| `RFC5310-4-1` | This document states that the remaining lifetime of the LSP MUST be set to zero before computing the authentication, thus this field is not authenticated. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391). **positive:** `unit/verify` [`TestRFC5310LifetimeNotAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5310_auth_lifetime_test.go#L17). **negative:** no negative test. **{single-polarity}:** ze always zeroes the Checksum and Remaining Lifetime before the LSP digest (auth_sign.go:268-272), so those fields are never part of the HMAC. The exclusion is observable only as non-rejection -- a post-sign Remaining-Lifetime change still verifies -- and there is no field-included code path that rejects, so no negative exists. The positive is proven in TestISISAuthLSPChecksumAfterSign and the HMAC-SHA-256 type-3 LSP round-trip TestISISAuthRotationOverlapAccepts |
| `RFC5310-4-2` | To ensure greater security, the keys used should be changed periodically, and implementations MUST be able to store and use more than one key at the same time. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_keystore_test.go#L78). **positive:** `unit/verify` [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L662). **negative:** `unit/verify` [`TestRFC5310NonSendingKeyStillUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L122) |
| `RFC5310-3.5-1` | The calculated data is compared with the received authentication data in the PDU, and the PDU is discarded if the two do not match. In such a case, an error event SHOULD be logged. (§3.5) | SHOULD | 3.5 | **positive:** `unit/verify` [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L29). **positive:** `unit/verify` [`TestRFC5310MismatchLogsAnErrorEvent`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_error_event_test.go#L44). **positive:** `unit/verify` [`TestRFC5310MismatchWarnsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_warn_test.go#L23). **negative:** `unit/verify` [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L683). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637). **negative:** `unit/verify` [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L30). **negative:** `unit/verify` [`TestRFC5310MismatchLogsAnErrorEvent`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_error_event_test.go#L47). **negative:** `unit/verify` [`TestRFC5310MismatchWarnsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_warn_test.go#L28) |
| `RFC5310-3.2-4` | IS-IS HELLO PDUs SHALL use the Link Level Authentication string, which MAY be different from that of Link State PDUs. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5310-3.5-2` | An implementation MAY have a transition mode where it includes CRYPTO_AUTH information in the PDUs but does not verify this information. This is provided as a transition aid for networks in the process of migrating to the new CRYPTO_AUTH-based authentication schemes. (§3.5) | MAY | 3.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5310-3.2-6`](#rfc5310-3.2-6) Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs MUST NOT include the Checksum TLV. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent is false -- ze implements no optional IS-IS per-PDU Checksum TLV (internal/plugins/isis/packet/tlv.go has no TLV 12 codec), so this conditional MUST NOT has no applicable code path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5310-3.2-1`](#rfc5310-3.2-1)

When calculating the CRYPTO_AUTH result for Sequence Number PDUs, Level 1 Sequence Number PDUs SHALL use the Area Authentication string, as in Level 1 Link State PDUs. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Now drives Level 1 SNPs, not LSPs. TestRFC5310Level1SNPUsesAreaString: signLevelPDU signs an L1 CSNP and PSNP that verify with areasecret under Key ID 1 and not with domainsecret under the same Key ID, and verifyFrame accepts them. TestRFC5310Level1SNPRefusesOtherStrings: the same CSNP/PSNP signed with the domain or link string, under Key ID 1 and under that string's own Key ID (2, 3), is rejected by verifyFrame while the area-signed copy is accepted, so a receiver that tried every configured chain goes red. Recorded red on signLevelPDU and verifyFrame.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5310Level1SNPRefusesOtherStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC5310Level1SNPUsesAreaString`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L49) | unit/verify | revert, verified |

### [`RFC5310-3.2-2`](#rfc5310-3.2-2)

Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L270) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L112) | unit/verify | unproven |

### [`RFC5310-3.2-3`](#rfc5310-3.2-3)

IS-IS HELLO PDUs SHALL use the Link Level Authentication string (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5310HelloUsesLinkString: signHelloPDU signs a LAN and a P2P IIH that verify with iihsecret under Key ID 3 and not with areasecret under Key ID 3, and verifyFrame accepts them, so a sender using the area chain goes red. TestRFC5310HelloRefusesAreaAndDomainStrings: the IIHs signed with the area or domain string, under Key ID 3 and under their own Key IDs (1, 2), are rejected while the link-signed IIH is accepted. Recorded red on signHelloPDU and verifyFrame. The old self-consistent round trip and no-TLV-10 tags are gone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5310HelloRefusesAreaAndDomainStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L103) | unit/verify | revert, verified |
| positive | [`TestRFC5310HelloUsesLinkString`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L86) | unit/verify | revert, verified |

### [`RFC5310-3.2-5`](#rfc5310-3.2-5)

The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (no sign-before-pad path, no padding-disable option). Both IIH producers are now asserted: circuit TestISISHelloSignedOverPaddedPDU on the LAN IIH (sendLANHello) and TestRFC5310P2PHelloSignedOverPaddedPDU on the point-to-point IIH (sendP2PHello), each capturing the bytes handed to the CRYPTO_AUTH signer and requiring length MTU - LLC with Padding TLV 8 present, so signing before padding on either path goes red. Recorded + on hello.go padHello for the P2P unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5310P2PHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L72) | unit/verify | revert, verified |

### [`RFC5310-3.2-6`](#rfc5310-3.2-6)

Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs MUST NOT include the Checksum TLV. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5310-3.2-6, so no unit is bound to it.

### [`RFC5310-3.4-1`](#rfc5310-3.4-1)

An implementation MUST fill the authentication type and the length before the authentication data is computed. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive. (a) forbidden: computing the digest before the auth-type byte and TLV length are in place; (b) TestISISAuthHMACSHAApadPreimage recomputes HMAC-SHA-256 over the signed PDU (which carries the final type 3 byte and length) with the data region Apad-filled and asserts bytes.Equal(onWire, wantApad): a digest computed over a pre-image lacking the type or length differs and goes red. TestISISAuthSignVerifyHMACSHA256 pins AuthType 3 and value length 34 on every PDU class.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISAuthHMACSHAApadPreimage`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L234) | unit/verify | unproven |
| positive | [`TestISISAuthSignVerifyHMACSHA256`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L161) | unit/verify | unproven |

### [`RFC5310-3.4-2`](#rfc5310-3.4-2)

The authentication data for the IS-IS IIH PDUs MUST be computed after the IS-IS Hello (IIH) has been padded to the MTU size, if padding is not explicitly disabled. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same units as RFC5310-3.2-5: the LAN IIH (TestISISHelloSignedOverPaddedPDU) and the point-to-point IIH (TestRFC5310P2PHelloSignedOverPaddedPDU) are both handed to the signer already padded to MTU - LLC with Padding TLV 8, so authentication data computed before padding on either producer goes red. Single-polarity positive per the row. Recorded + on padHello.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5310P2PHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L161) | unit/verify | revert, verified |
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L75) | unit/verify | revert, verified |

### [`RFC5310-4-1`](#rfc5310-4-1)

This document states that the remaining lifetime of the LSP MUST be set to zero before computing the authentication, thus this field is not authenticated. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive, now on the CRYPTO_AUTH path. New TestRFC5310LifetimeNotAuthenticated signs an LSP with HMAC-SHA-256 (type 3, Key ID 7), sets the Remaining Lifetime to 1, 600 and 0xFFFF with the Fletcher checksum recomputed, and requires VerifyPDU to accept each; a digest that covered the lifetime (lspAuthLayout not zeroing it) goes red. A changed sequence number is refused, so an accept-all verifier also goes red. Recorded red on auth_sign.go lspAuthLayout. The {single-polarity} marker holds (no lifetime-included path exists to refuse). Its prose still names TestISISAuthRotationOverlapAccepts, whose tag was removed: stale annotation text, not a coverage gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC5310LifetimeNotAuthenticated`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/rfc5310_auth_lifetime_test.go#L17) | unit/verify | revert, verified |

### [`RFC5310-4-2`](#rfc5310-4-2)

To ensure greater security, the keys used should be changed periodically, and implementations MUST be able to store and use more than one key at the same time. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) on its positives, per the main-thread decision following the owner's BFD RFC5880-6.1-3 ruling (a capability MUST with no refusal path is proven by its positives). Every clause is proven: STORE more than one key, root TestISISAuthRotation requires verifyKeys to return both keys (len 2) before and after the send rollover; USE more than one key at the same time, packet TestISISAuthRotationOverlapAccepts verifies a PDU signed by either of two candidate keys, and root TestRFC5310NonSendingKeyStillUsed verifies with an accepted key that is not the sending key, while signKey moves from key 1 to key 2 in TestISISAuthRotation. Each fails with a one-key store (keys[:1] fails the new-key verify; a single-entry store fails len 2). Records: + revert acceptKeys (TestISISAuthRotation), + revert VerifyPDU (TestISISAuthRotationOverlapAccepts, recorded by this judge), - revert acceptKeys (TestRFC5310NonSendingKeyStillUsed). The negative tag on TestRFC5310NonSendingKeyStillUsed is kept as supplementary because the coverage ratchet holds it: it is a further positive, not a rejected violation, and no input drives the store toward one key.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5310NonSendingKeyStillUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_scope_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L662) | unit/verify | revert, verified |
| positive | [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_keystore_test.go#L78) | unit/verify | revert, verified |

### [`RFC5310-3.5-1`](#rfc5310-3.5-1)

The calculated data is compared with the received authentication data in the PDU, and the PDU is discarded if the two do not match. In such a case, an error event SHOULD be logged. (§3.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the D-8 (R41) fix: verifyFrame now logs a Warn (RFC 5310 sec 3.5 quoted above it), rate-limited to one line per interface and level per 10 s by auth_warn.go authWarnLimiter, with the suppressed count on the next line; every failure still increments ze_isis_auth_failures_total. Both clauses now held. Discard: root TestRFC1195InvalidAuthLSPDiscardedAtReceive, TestRFC5310MismatchLogsAnErrorEvent and TestRFC5310MismatchWarnsRateLimited deliver the matching HMAC-SHA-256 L1 LSP and drop the one-octet-altered copy before the handler; packet TestISISAuthWrongKeyRejected and TestISISAuthKeyIDMismatchRejected refuse at VerifyPDU. Error event: TestRFC5310MismatchWarnsRateLimited runs the logger at Info (Debug hidden) and requires a WARN line naming level=l1 and the neighbor SNPA on the first mismatch, none 1 s later, and a second line with suppressed=1 after the interval (positive); the matching LSP logs no WARN (negative). A regression to Debug goes red at the assertion. Recorded + on auth_warn.go allow, - on verifyFrame.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L30) | unit/verify | revert, verified |
| negative | [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L683) | unit/verify | revert, verified |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637) | unit/verify | revert, verified |
| negative | [`TestRFC5310MismatchLogsAnErrorEvent`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_error_event_test.go#L47) | unit/verify | revert, verified |
| negative | [`TestRFC5310MismatchWarnsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_warn_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC1195InvalidAuthLSPDiscardedAtReceive`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_discard_rfc1195_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC5310MismatchLogsAnErrorEvent`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_error_event_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC5310MismatchWarnsRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/rfc5310_auth_warn_test.go#L23) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5310.txt |
| Source fingerprint | f5b2eddf2b4cd973 |
| Record | rfc/extraction/rfc5310.json |
| Mapped sentences | 9 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 5 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 2 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `4` | not stated | 4 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose explaining why confidentiality is out of scope for a routing protocol; the lowercase 'required' describes the problem space and imposes nothing. | However, the objective of a routing protocol is to advertise the routing topology, and confidentiality is not normally required for routing protocols. |
| `4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations guidance to the operator, in lowercase 'must'; Section 1.1 binds only the capitalised spellings to RFC 2119, and the sentence constrains how long a deployment keeps the optional transition mode rather than what an implementation does. | The operator must ensure that this mode is only used when migrating to the new CRYPTO_AUTH-based authentication scheme, as this leaves the router vulnerable to an attack. |
| `4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations commentary pointing at RFC 2154 digital signatures as an approach that 'should be seriously considered' if stronger authentication were wanted; lowercase, conditional, and about a mechanism this document does not define. | If a stronger authentication were believed to be required, then the use of a full digital signature [RFC2154] would be an approach that should be seriously considered. |

## Superseded

No document obsoletes RFC 5310, so its obligations are stated where they were written.
