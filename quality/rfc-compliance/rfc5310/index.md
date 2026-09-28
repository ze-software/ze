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
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.1% | 1 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 7 | of 9 gated MUSTs judged | 6 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

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
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 7 |
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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5310-3.2-1`](#rfc5310-3.2-1), [`RFC5310-3.2-2`](#rfc5310-3.2-2), [`RFC5310-3.2-3`](#rfc5310-3.2-3), [`RFC5310-4-2`](#rfc5310-4-2)

**Annotated instead of tested (5):** [`RFC5310-3.2-5`](#rfc5310-3.2-5), [`RFC5310-3.2-6`](#rfc5310-3.2-6), [`RFC5310-3.4-1`](#rfc5310-3.4-1), [`RFC5310-3.4-2`](#rfc5310-3.4-2), [`RFC5310-4-1`](#rfc5310-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5310-3.2-1` | When calculating the CRYPTO_AUTH result for Sequence Number PDUs, Level 1 Sequence Number PDUs SHALL use the Area Authentication string, as in Level 1 Link State PDUs. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L112). **negative:** `unit/verify` [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L202) |
| `RFC5310-3.2-2` | Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L116). **negative:** `unit/verify` [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L284) |
| `RFC5310-3.2-3` | IS-IS HELLO PDUs SHALL use the Link Level Authentication string (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L156). **negative:** `unit/verify` [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L168) |
| `RFC5310-3.2-5` | The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L72). **negative:** no negative test. **{single-polarity}:** ze always pads the IIH before signing it (circuit/runtime.go:273-276 for LAN, :293-304 for P2P), so there is no sign-before-pad code path and no negative (unpadded-sign) behavior to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the bytes handed to the signer are already padded to MTU-LLC and carry Padding TLV 8 |
| `RFC5310-3.2-6` | Implementations that support the optional checksum for the Sequence Number PDUs and IS-IS HELLO PDUs MUST NOT include the Checksum TLV. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent is false -- ze implements no optional IS-IS per-PDU Checksum TLV (internal/plugins/isis/packet/tlv.go has no TLV 12 codec), so this conditional MUST NOT has no applicable code path |
| `RFC5310-3.4-1` | An implementation MUST fill the authentication type and the length before the authentication data is computed. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISAuthHMACSHAApadPreimage`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L234). **positive:** `unit/verify` [`TestISISAuthSignVerifyHMACSHA256`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L161). **negative:** no negative test. **{single-polarity}:** ze always builds the Authentication TLV with the auth-type byte and sets the TLV length before the digest is computed (auth_sign.go:47-52), then runs the HMAC over the full signed PDU (auth_sign.go:274-276), so no code path omits the type or length from the pre-image and there is no negative to assert. The positive is proven by the known-answer TestISISAuthHMACSHAApadPreimage (the re-hashed pre-image still carries the type byte and length) and the on-wire type-3 round-trip TestISISAuthSignVerifyHMACSHA256 |
| `RFC5310-3.4-2` | The authentication data for the IS-IS IIH PDUs MUST be computed after the IS-IS Hello (IIH) has been padded to the MTU size, if padding is not explicitly disabled. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L75). **negative:** no negative test. **{single-polarity}:** ze pads the IIH before signing it (circuit/runtime.go:273-276 pads then signs), with no sign-before-pad code path, so no negative (auth-computed-before-padding) behavior exists to assert. The positive is proven in TestISISHelloSignedOverPaddedPDU: the signer receives the PDU already padded to MTU-LLC |
| `RFC5310-4-1` | This document states that the remaining lifetime of the LSP MUST be set to zero before computing the authentication, thus this field is not authenticated. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391). **positive:** `unit/verify` [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L666). **negative:** no negative test. **{single-polarity}:** ze always zeroes the Checksum and Remaining Lifetime before the LSP digest (auth_sign.go:268-272), so those fields are never part of the HMAC. The exclusion is observable only as non-rejection -- a post-sign Remaining-Lifetime change still verifies -- and there is no field-included code path that rejects, so no negative exists. The positive is proven in TestISISAuthLSPChecksumAfterSign and the HMAC-SHA-256 type-3 LSP round-trip TestISISAuthRotationOverlapAccepts |
| `RFC5310-4-2` | To ensure greater security, the keys used should be changed periodically, and implementations MUST be able to store and use more than one key at the same time. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_keystore_test.go#L78). **positive:** `unit/verify` [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L663). **negative:** `unit/verify` [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L688). **negative:** `unit/verify` [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637) |
| `RFC5310-3.5-1` | The calculated data is compared with the received authentication data in the PDU, and the PDU is discarded if the two do not match. In such a case, an error event SHOULD be logged. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
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

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence binds Level 1 Sequence Number PDUs (CSNP/PSNP). Both tagged units drive an LSP (authTestLSP): TestISISAuthEngineSignLevel round-trips an L1 LSP through signLevelPDU/verifyFrame, and TestISISAuthChainSelection rejects an L1 LSP signed with the IIH key. They prove the neighbouring rule that L1 LSPs use the area string. No tagged unit signs or verifies an L1 SNP, so an SNP signed with the domain or IIH string stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthChainSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L202) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L112) | unit/verify | unproven |

### [`RFC5310-3.2-2`](#rfc5310-3.2-2)

Level 2 Sequence Number PDUs shall use the domain authentication string, as in Level 2 Link State PDUs (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthLevelChainCrossUseL2`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L284) | unit/verify | unproven |
| positive | [`TestISISAuthEngineSignLevel`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L116) | unit/verify | unproven |

### [`RFC5310-3.2-3`](#rfc5310-3.2-3)

IS-IS HELLO PDUs SHALL use the Link Level Authentication string (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an IIH authenticated with a string other than the Link Level one (the area or domain string); (b) no tagged assertion goes red on it. The positive round-trips signHelloPDU through verifyFrame, which stays green if both sides switched to the area chain (self-consistent). The negative rejects an IIH with no TLV 10, which proves 'authentication is required', not which string. The untagged wrong-key case uses an unconfigured key 'nope'. Missing: an IIH signed with the area/domain key rejected, and a check that the sender picks the per-interface chain.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L168) | unit/verify | unproven |
| positive | [`TestISISAuthReject`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_wiring_test.go#L156) | unit/verify | unproven |

### [`RFC5310-3.2-5`](#rfc5310-3.2-5)

The CRYPTO_AUTH result for the IS-IS HELLO PDUs SHALL be calculated after the PDU is padded to the MTU size, if padding is not disabled. (§3.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Single-polarity positive. (a) forbidden: computing the IIH CRYPTO_AUTH result before padding to MTU; (b) TestISISHelloSignedOverPaddedPDU len(cs.pdu)!=wantPDU and the Padding TLV 8 check go red on sign-before-pad, but only on a LAN circuit (lanCircuit, SendHello->sendLANHello). The P2P IIH producer (sendP2PHello, runtime.go) has no tagged assertion, so a sign-before-pad there stays green. Ze has no padding-disable option, so the conditional branch does not arise.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L72) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Same unit and gap as RFC5310-3.2-5: the pad-before-sign assertion (len == MTU-LLC, Padding TLV 8 present) runs on the LAN IIH only; the P2P IIH producer sendP2PHello has no tagged assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISHelloSignedOverPaddedPDU`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/runtime_test.go#L75) | unit/verify | unproven |

### [`RFC5310-4-1`](#rfc5310-4-1)

This document states that the remaining lifetime of the LSP MUST be set to zero before computing the authentication, thus this field is not authenticated. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Single-polarity positive. (a) forbidden: including the Remaining Lifetime in the CRYPTO_AUTH digest; (b) the only assertion that goes red on it, TestISISAuthLSPChecksumAfterSign 'VerifyPDU after lifetime change', signs with HMAC-MD5 (RFC 5304 type 54), not a CRYPTO_AUTH key. The type-3 unit TestISISAuthRotationOverlapAccepts only round-trips an LSP without touching the lifetime, so it stays green if the lifetime were hashed on both sides. The zeroing in auth_sign.go is shared across algorithms today, but no tagged unit proves it on the RFC 5310 path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestISISAuthLSPChecksumAfterSign`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L391) | unit/verify | unproven |
| positive | [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L666) | unit/verify | unproven |

### [`RFC5310-4-2`](#rfc5310-4-2)

To ensure greater security, the keys used should be changed periodically, and implementations MUST be able to store and use more than one key at the same time. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive is sound: TestISISAuthRotationOverlapAccepts verifies PDUs signed by either of two candidate keys, and TestISISAuthRotation asserts verifyKeys returns 2 keys before and after rollover; a one-key store goes red. The negative tags (TestISISAuthWrongKeyRejected, TestISISAuthKeyIDMismatchRejected) prove a neighbouring rule, rejection of an unknown key or Key ID (RFC 5304 sec 2 / RFC 5310 sec 3.5), not a violation of 'store and use more than one key'. No true negative and no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISAuthKeyIDMismatchRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L688) | unit/verify | unproven |
| negative | [`TestISISAuthWrongKeyRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L637) | unit/verify | unproven |
| positive | [`TestISISAuthRotation`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/auth_keystore_test.go#L78) | unit/verify | unproven |
| positive | [`TestISISAuthRotationOverlapAccepts`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/auth_verify_test.go#L663) | unit/verify | unproven |

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
