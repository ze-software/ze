# RFC 7427 - Signature Authentication in the Internet Key Exchange Version 2 (IKEv2)

No row in the public ledger. Every requirement this repository extracted from RFC 7427, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 25.0% | 2 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 25.0% | 1 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 3 | of 4 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 3 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc7427.md` |
| Requirement shard | `rfc/requirements/rfc7427.md` |
| RFC text | `rfc/full/rfc7427.txt` |

## Enrolment

Enrolled: Signature Authentication in IKEv2: three MUST-level requirements. RFC7427-3-4 (Section 3 permits the Digital Signature method only once a SIGNATURE_HASH_ALGORITHMS notify has been sent and received by each peer) is tested both polarities: computeX509Auth (internal/component/ike/engine/auth.go) refuses the method on an empty sa.RemoteHashAlgos and emits method 14 once one arrived (TestRFC7427DigitalSignatureNeedsTheNotify, TestAuthX509UsesMethod14), while PSK auth uses method 2 (TestAuthPSKCompute), and ze always sends the notify (buildSignatureHashAlgosNotify). RFC7427-4-1 (Section 4: the signer picks one algorithm the other peer sent) is tested both polarities over selectSignatureAlgorithm and computeX509Auth (TestRFC7427SignatureAlgorithmIsOneThePeerSent, TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend). RFC7427-3-1 (ANSI X9.62 hash truncation) is {not-applicable}: ze delegates ECDSA to Go crypto/ecdsa (ecdsa.SignASN1/VerifyASN1), which does the truncation; ze implements none itself. No MUST gap remains gated. docs/features/rfc-status.md carries no RFC 7427 row, which its grandfathering permits and which no change here alters.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7427.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC7427-4-1`](#rfc7427-4-1), [`RFC7427-3-4`](#rfc7427-3-4), [`RFC7427-3-5`](#rfc7427-3-5)

**Annotated instead of tested (1):** [`RFC7427-3-1`](#rfc7427-3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7427-4-1` | When calculating the digital signature, a peer MUST pick one algorithm sent by the other peer. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L129). **negative:** `unit/verify` [`TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L192). **negative:** `unit/verify` [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L134) |
| `RFC7427-3-4` | the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestAuthX509UsesMethod14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/auth_test.go#L72). **positive:** `unit/verify` [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L89). **negative:** `unit/verify` [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L95) |
| `RFC7427-3-5` | If both ends send and receive SIGNATURE_HASH_ALGORITHMS Notify payloads, and signature authentication is to be used, then the authentication method specified in this Authentication payload MUST be used. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L24). **negative:** `unit/verify` [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L25) |
| `RFC7427-3-1` | For hash truncation, the method specified in ANSI X9.62:2005 [X9.62] MUST be used. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not implement ECDSA hash truncation itself -- it passes the full digest to Go's crypto/ecdsa (signDigest -> ecdsa.SignASN1, and verifySignature -> ecdsa.VerifyASN1, both in internal/component/ike/engine/auth.go), which performs the FIPS 186-4 / ANSI X9.62 leftmost-bits truncation internally. There is no ze-authored truncation code to gate |
| `RFC7427-4-2` | The supported hash algorithms that can be used for the signature algorithms are indicated with a Notify payload of type SIGNATURE_HASH_ALGORITHMS sent inside the IKE_SA_INIT exchange. This notification also implicitly indicates support of the new "Digital Signature" algorithm method, as well as the list of hash functions supported by the sending peer. Both ends send their list of supported hash algorithms. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7427-3-2` | When encoding the ASN.1, implementations SHOULD use the preferred format called for by the algorithm specification. If the algorithm specification says "preferredPresent", then the parameters object needs to be present, although it will be NULL if no parameters are specified. If the algorithm specification says "preferredAbsent", then the entire optional parameters object is missing. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7427-3-3` | This length field allows simple implementations to know the length of the ASN.1 object without the need to parse it, so they can use it as a binary blob to be compared against known signature algorithm ASN.1 objects. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7427-3-1`](#rfc7427-3-1) For hash truncation, the method specified in ANSI X9.62:2005 [X9.62] MUST be used. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not implement ECDSA hash truncation itself -- it passes the full digest to Go's crypto/ecdsa (signDigest -> ecdsa.SignASN1, and verifySignature -> ecdsa.VerifyASN1, both in internal/component/ike/engine/auth.go), which performs the FIPS 186-4 / ANSI X9.62 leftmost-bits truncation internally. There is no ze-authored truncation code to gate |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7427-4-1`](#rfc7427-4-1)

When calculating the digital signature, a peer MUST pick one algorithm sent by the other peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: signing with a hash algorithm the peer did not list in its SIGNATURE_HASH_ALGORITHMS notify. TestRFC7427SignatureAlgorithmIsOneThePeerSent asserts selectSignatureAlgorithm returns errNoMutualHashAlgo for every list lacking a usable algorithm (SHA-1 alone, SHA2-384 alone for P-256, SHA2-256 alone for P-384, nil) and that the chosen identifier equals the one the peer sent; TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend asserts computeX509Auth returns errNoMutualHashAlgo for a P-384 key against {2} and an RSA key against {1}. Both polarities. Residual: no computeX509Auth-level positive reads the AlgorithmIdentifier in AuthData, so the wiring from select to the encoded payload is proven only through the refusal path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L192) | unit/verify | unproven |
| negative | [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L134) | unit/verify | unproven |
| positive | [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L129) | unit/verify | unproven |

### [`RFC7427-3-4`](#rfc7427-3-4)

the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence conditions method 14 on the notify having been sent AND received by each peer. Received half: TestRFC7427DigitalSignatureNeedsTheNotify asserts computeX509Auth returns errNoSignatureHashAlgos with sa.RemoteHashAlgos nil and method 14 once a list arrived; red on a producer that emits 14 without a received notify. Sent half: no tagged assertion goes red if ze stops sending its own SIGNATURE_HASH_ALGORITHMS notify in IKE_SA_INIT (buildSignatureHashAlgosNotify) while still emitting method 14; the tests rest on the comment that ze always sends it. TestAuthX509UsesMethod14 is positive only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L95) | unit/verify | unproven |
| positive | [`TestAuthX509UsesMethod14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/auth_test.go#L72) | unit/verify | unproven |
| positive | [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L89) | unit/verify | unproven |

### [`RFC7427-3-5`](#rfc7427-3-5)

If both ends send and receive SIGNATURE_HASH_ALGORITHMS Notify payloads, and signature authentication is to be used, then the authentication method specified in this Authentication payload MUST be used. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: with the notify exchanged and signature authentication in use, emitting a legacy AUTH method (1 RSA, 3 DSS) instead of Digital Signature (14). TestRFC7427DigitalSignatureMethodIsUsed asserts auth.AuthMethod == wire.AuthMethodDigitalSig with RemoteHashAlgos set (red on any other method) and, as the negative, that with no notify received computeX509Auth returns an error and a nil payload rather than falling back to a legacy method.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L24) | unit/verify | revert, verified |

### [`RFC7427-3-1`](#rfc7427-3-1)

For hash truncation, the method specified in ANSI X9.62:2005 [X9.62] MUST be used. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7427-3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc7427.txt |
| Source fingerprint | afec1d0d40ee2a25 |
| Record | rfc/extraction/rfc7427.json |
| Mapped sentences | 3 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.1.1` | not stated | 1 | walked | not stated |
| `A.1.2` | not stated | 1 | walked | not stated |
| `A.1.3` | not stated | 1 | walked | not stated |
| `A.1.4` | not stated | 1 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.2.1` | not stated | 0 | walked | not stated |
| `A.2.2` | not stated | 0 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |
| `A.3.1` | not stated | 0 | walked | not stated |
| `A.3.2` | not stated | 0 | walked | not stated |
| `A.3.3` | not stated | 0 | walked | not stated |
| `A.3.4` | not stated | 0 | walked | not stated |
| `A.4` | not stated | 1 | walked | not stated |
| `A.4.1` | not stated | 1 | walked | not stated |
| `A.4.2` | not stated | 0 | walked | not stated |
| `A.4.3` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `B.1` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC boilerplate: the Trust Legal Provisions notice about Simplified BSD License text in extracted code components. It is a licence term, not a protocol requirement. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction motivation: it states why fixed per-algorithm AUTH method numbers do not scale ('will cause the number of required authentication methods to become unmanageable'), and imposes nothing. | The combination of new ECDSA groups and hash functions will cause the number of required authentication methods to become unmanageable. |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so this sentence sets precedence rather than an obligation of its own: where the example differs from the algorithm's own specification, that specification governs (the same point Section 3 makes with SHOULD, RFC7427-3-2). | If the ASN.1 object listed in Appendix A and the ASN.1 object specified by the algorithm differ, then the algorithm specification must be used. |
| `A.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the lowercase 'must be NULL' describes the sha1WithRSAEncryption AlgorithmIdentifier encoding that [RFC3279]/[RFC4055] define, reproduced here as an example. | Parameters are required, and they must be NULL. |
| `A.1.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the lowercase 'must be NULL' describes the sha256WithRSAEncryption AlgorithmIdentifier encoding that [RFC3279]/[RFC4055] define, reproduced here as an example. | Parameters are required, and they must be NULL. |
| `A.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the lowercase 'must be NULL' describes the sha384WithRSAEncryption AlgorithmIdentifier encoding that [RFC3279]/[RFC4055] define, reproduced here as an example. | Parameters are required, and they must be NULL. |
| `A.1.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the lowercase 'must be NULL' describes the sha512WithRSAEncryption AlgorithmIdentifier encoding that [RFC3279]/[RFC4055] define, reproduced here as an example. | Parameters are required, and they must be NULL. |
| `A.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the sentence describes how RSASSA-PSS encodes its identifier and parameters, as [RFC3447] Section 8.1 defines it. | With RSASSA-PSS, the algorithm object identifier must always be id-RSASSA-PSS, and the hash function and padding parameters are conveyed in the parameters (which are not optional in this case). |
| `A.4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A opens 'This section is not normative, and these values should only be used as examples.', so the sentence describes the ASN.1 encoding of the RSASSA-PSS default parameter set in this example. | Parameters are empty, but the ASN.1 part of the sequence must be present. |

## Superseded

No document obsoletes RFC 7427, so its obligations are stated where they were written.
