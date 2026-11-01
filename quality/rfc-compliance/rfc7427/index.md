# RFC 7427 - Signature Authentication in the Internet Key Exchange Version 2 (IKEv2)

No row in the public ledger. Every requirement this repository extracted from RFC 7427, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 11 | 11 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 4 gated MUSTs: they add up to 4. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 11 |
| Recorded audit verdicts | 3 |
| Discrimination records | 11 |
| Summary | `rfc/short/rfc7427.md` |
| Requirement shard | `rfc/requirements/rfc7427.md` |
| RFC text | `rfc/full/rfc7427.txt` |

## Enrolment

Enrolled: Signature Authentication in IKEv2: four MUST-level requirements. RFC7427-3-4 (Section 3 permits the Digital Signature method only once a SIGNATURE_HASH_ALGORITHMS notify has been sent and received by each peer) is tested both polarities: computeX509Auth (internal/component/ike/engine/auth.go) emits no method 14 while sa.RemoteHashAlgos is nil (no notify received), signing with the key's legacy method 1, 9, 10 or 11 instead (auth_legacy.go, RFC 8247 Section 3.1), and emits method 14 once one arrived (TestRFC7427DigitalSignatureNeedsTheNotify, TestAuthX509UsesMethod14), while PSK auth uses method 2 (TestAuthPSKCompute), and ze always sends the notify (buildSignatureHashAlgosNotify). RFC7427-4-1 (Section 4: the signer picks one algorithm the other peer sent) is tested both polarities over selectSignatureAlgorithm and computeX509Auth (TestRFC7427SignatureAlgorithmIsOneThePeerSent, TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend). RFC7427-3-1 (ANSI X9.62 hash truncation) binds Ze as signer and verifier, and Go crypto/ecdsa (ecdsa.SignASN1, ecdsa.VerifyASN1) performs the truncation on the full digest Ze hands it: TestRFC7427ECDSAHashTruncationIsX962 has verifyRemoteAuth accept a P-256 peer's ecdsa-with-SHA384 signature over the leftmost 256 bits of the digest and refuse one over the rightmost 256. No MUST gap remains gated. docs/features/rfc-status.md carries no RFC 7427 row, which its grandfathering permits and which no change here alters.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7427.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7427-4-1`](#rfc7427-4-1), [`RFC7427-3-4`](#rfc7427-3-4), [`RFC7427-3-5`](#rfc7427-3-5), [`RFC7427-3-1`](#rfc7427-3-1)

## Figures

- **MUST bar**: Ze proven 4
- **Overview bar**: Ze MUST proven 4, SHOULD not decided 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 4 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
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
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 3 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc7427.json](../../../data/rfc/rfc7427.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7427-4-1` | When calculating the digital signature, a peer MUST pick one algorithm sent by the other peer. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L140). **negative:** `unit/verify` [`TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L203). **negative:** `unit/verify` [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L145) |
| `RFC7427-3-4` | the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestAuthX509UsesMethod14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_auth_test.go#L73). **positive:** `unit/verify` [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L90). **positive:** `unit/verify` [`TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_notify_sent_test.go#L54). **negative:** `unit/verify` [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L96) |
| `RFC7427-3-5` | If both ends send and receive SIGNATURE_HASH_ALGORITHMS Notify payloads, and signature authentication is to be used, then the authentication method specified in this Authentication payload MUST be used. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L31). **negative:** `unit/verify` [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L32) |
| `RFC7427-3-1` | For hash truncation, the method specified in ANSI X9.62:2005 [X9.62] MUST be used. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7427ECDSAHashTruncationIsX962`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_truncation_test.go#L52). **negative:** `unit/verify` [`TestRFC7427ECDSAHashTruncationIsX962`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_truncation_test.go#L57) |
| `RFC7427-4-2` | The supported hash algorithms that can be used for the signature algorithms are indicated with a Notify payload of type SIGNATURE_HASH_ALGORITHMS sent inside the IKE_SA_INIT exchange. This notification also implicitly indicates support of the new "Digital Signature" algorithm method, as well as the list of hash functions supported by the sending peer. Both ends send their list of supported hash algorithms. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7427-3-2` | When encoding the ASN.1, implementations SHOULD use the preferred format called for by the algorithm specification. If the algorithm specification says "preferredPresent", then the parameters object needs to be present, although it will be NULL if no parameters are specified. If the algorithm specification says "preferredAbsent", then the entire optional parameters object is missing. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7427-3-3` | This length field allows simple implementations to know the length of the ASN.1 object without the need to parse it, so they can use it as a binary blob to be compared against known signature algorithm ASN.1 objects. (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7427 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7427-4-1`](#rfc7427-4-1)

When calculating the digital signature, a peer MUST pick one algorithm sent by the other peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: signing with a hash algorithm the peer did not list in its SIGNATURE_HASH_ALGORITHMS notify. TestRFC7427SignatureAlgorithmIsOneThePeerSent asserts selectSignatureAlgorithm returns errNoMutualHashAlgo for every list lacking a usable algorithm (SHA-1 alone, SHA2-384 alone for P-256, SHA2-256 alone for P-384, nil) and that the chosen identifier equals the one the peer sent; TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend asserts computeX509Auth returns errNoMutualHashAlgo for a P-384 key against {2} and an RSA key against {1}. Both polarities. Residual: no computeX509Auth-level positive reads the AlgorithmIdentifier in AuthData, so the wiring from select to the encoded payload is proven only through the refusal path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427AuthRefusesAnAlgorithmThePeerDidNotSend`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L203) | unit/verify | revert, verified |
| negative | [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC7427SignatureAlgorithmIsOneThePeerSent`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L140) | unit/verify | revert, verified |

### [`RFC7427-3-4`](#rfc7427-3-4)

the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after the no-notify branch changed from a refusal to a legacy signature. RFC 7427 Section 3 grants method 14 only once the notify was sent and received by each peer. Received half: TestRFC7427DigitalSignatureNeedsTheNotify drives computeX509Auth with RemoteHashAlgos nil and fails on method 14 (and pins the P-256 key's method 9), then sets a received list and requires method 14, so a computeX509Auth that ignored the notify goes red on one branch or the other. Sent half: TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify parses Ze's real IKE_SA_INIT request and responder reply, each carrying exactly one notify listing 2,3,4. TestAuthX509UsesMethod14 positive, PSK uses method 2. The recorded discrimination breaks are whole-body panics, which prove only that each unit reaches its producer; the branch discrimination above is read from the assertions. Untagged support: TestParseHashAlgoNotifyShort keeps nil meaning no notify received.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestAuthX509UsesMethod14`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_auth_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_notify_sent_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC7427DigitalSignatureNeedsTheNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_sighash_test.go#L90) | unit/verify | revert, verified |

### [`RFC7427-3-5`](#rfc7427-3-5)

If both ends send and receive SIGNATURE_HASH_ALGORITHMS Notify payloads, and signature authentication is to be used, then the authentication method specified in this Authentication payload MUST be used. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after the test dropped its no-notify case and gained key types. With RemoteHashAlgos set, TestRFC7427DigitalSignatureMethodIsUsed drives computeX509Auth for RSA-2048 and ECDSA on P-256, P-384 and P-521, every key type that now has a legacy method in auth_legacy.go and auth_ecdsa.go, and fails on any of methods 1, 3, 9, 10, 11 and on anything other than 14 with non-empty data, so a legacy branch leaking past the notify goes red for each key. Dropping the no-notify case is right: RFC 7427 Section 3 conditions this sentence on both ends having sent the notify, and that case belongs to RFC7427-3-4. Finding: the negative tag is entailed by the positive's == 14 assertion over the same inputs, so the pair is one assertion in two hats; it still discriminates because the requirement is a signer obligation whose only violation is Ze's own output. Untested edge: a received notify with an empty list must not fall back to a legacy method (parseHashAlgoNotify answers []uint16{}, pinned untagged by TestParseHashAlgoNotifyShort, never through computeX509Auth). Recorded discrimination breaks are whole-body panics only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC7427DigitalSignatureMethodIsUsed`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_method_test.go#L31) | unit/verify | revert, verified |

### [`RFC7427-3-1`](#rfc7427-3-1)

For hash truncation, the method specified in ANSI X9.62:2005 [X9.62] MUST be used. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7427ECDSAHashTruncationIsX962`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_truncation_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC7427ECDSAHashTruncationIsX962`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc7427_truncation_test.go#L52) | unit/verify | revert, verified |

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
