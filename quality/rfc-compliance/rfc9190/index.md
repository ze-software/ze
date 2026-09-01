# RFC 9190 - EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3

No row in the public ledger. Every requirement this repository extracted from RFC 9190, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3.9% | 2 of 51 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 51 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 51 | of 94 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 51 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| One polarity, unexcused | 2.0% | 1 of 51 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 94.1% | 48 of 51 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 51 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | bad | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (backlog) |
| Requirements | 94 |
| Gated MUST-level | 51 |
| Obligations that bind Ze | 51 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 48 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9190.md` |
| Requirement shard | `rfc/requirements/rfc9190.md` |
| RFC text | `rfc/full/rfc9190.txt` |

## Enrolment

Not enrolled (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): EAP-TLS 1.3: Using EAP with TLS 1.3. Summary written 2026-08-01 and it declares 51 MUST-level obligations over 19 sections. It is NOT enrolled because 49 of them are not yet proven by a tagged test, and the two routes to enrolment are both closed to an implementer: proving all 51 in both polarities is spec-sized work, and annotating the remainder is a conformance judgement ai/rules/rfc-compliance.md reserves to the owner. What Ze does implement today is the Section 2.3 key derivation and the Section 2.5 protected success result indication. exportEAPTLSMSK (internal/component/ike/eap/eap_tls.go) selects the exporter label EXPORTER_EAP_TLS_Key_Material, the Type-Code context and the 128-octet length whenever the negotiated version is TLS 1.3, and test/interop-ipsec/scenarios/eap-tls13 exercises that path against strongSwan. tlsMethod.indicateSuccess (same file, added 2026-08-12) writes the encrypted TLS record carrying application data 0x00 in the round that completes the handshake, so RFC9190-2.5-1 and RFC9190-2.5-2 now carry both polarities; test/interop-ipsec/scenarios/responder-eap-tls13 proves it with Ze in the EAP-TLS SERVER role, and with the write reverted strongSwan logs missing protected success indication for EAP-TLS with TLS 1.3 and the SA never establishes. PeerSession.handleTLSRequest still does not CONSUME the indication: it answers the record with the no-data EAP-Response Section 2.5 step 4 asks for, but never decrypts it, so a Ze peer cannot tell a server that sent one from a server that did not. The published RFC states no peer-side obligation and errata 7577, which proposes one, is Reported rather than Verified. Escalated for a scoping ruling rather than decided, per the same route rfc1035 and rfc5301 took.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9190.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 1 | one part of the gated population |
| No test and no annotation | 48 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **51** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9190-2.5-1`](#rfc9190-2.5-1), [`RFC9190-2.5-2`](#rfc9190-2.5-2)

**One polarity only (1):** [`RFC9190-2.3-1`](#rfc9190-2.3-1)

**No test and no annotation (48):** [`RFC9190-2.1-2`](#rfc9190-2.1-2), [`RFC9190-2.1-3`](#rfc9190-2.1-3), [`RFC9190-2.1-4`](#rfc9190-2.1-4), [`RFC9190-2.1-5`](#rfc9190-2.1-5), [`RFC9190-2.1-6`](#rfc9190-2.1-6), [`RFC9190-2.1-7`](#rfc9190-2.1-7), [`RFC9190-2.1-8`](#rfc9190-2.1-8), [`RFC9190-2.1.1-1`](#rfc9190-2.1.1-1), [`RFC9190-2.1.1-2`](#rfc9190-2.1.1-2), [`RFC9190-2.1.2-1`](#rfc9190-2.1.2-1), [`RFC9190-2.1.2-2`](#rfc9190-2.1.2-2), [`RFC9190-2.1.2-3`](#rfc9190-2.1.2-3), [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4), [`RFC9190-2.1.3-1`](#rfc9190-2.1.3-1), [`RFC9190-2.1.3-2`](#rfc9190-2.1.3-2), [`RFC9190-2.1.4-1`](#rfc9190-2.1.4-1), [`RFC9190-2.1.4-2`](#rfc9190-2.1.4-2), [`RFC9190-2.1.4-3`](#rfc9190-2.1.4-3), [`RFC9190-2.1.8-1`](#rfc9190-2.1.8-1), [`RFC9190-2.1.8-2`](#rfc9190-2.1.8-2), [`RFC9190-2.1.8-3`](#rfc9190-2.1.8-3), [`RFC9190-2.1.8-4`](#rfc9190-2.1.8-4), [`RFC9190-2.1.9-1`](#rfc9190-2.1.9-1), [`RFC9190-2.1.9-2`](#rfc9190-2.1.9-2), [`RFC9190-2.2-1`](#rfc9190-2.2-1), [`RFC9190-2.3-2`](#rfc9190-2.3-2), [`RFC9190-2.3-3`](#rfc9190-2.3-3), [`RFC9190-2.4-1`](#rfc9190-2.4-1), [`RFC9190-2.4-2`](#rfc9190-2.4-2), [`RFC9190-5.4-1`](#rfc9190-5.4-1), [`RFC9190-5.4-2`](#rfc9190-5.4-2), [`RFC9190-5.4-3`](#rfc9190-5.4-3), [`RFC9190-5.4-4`](#rfc9190-5.4-4), [`RFC9190-5.4-5`](#rfc9190-5.4-5), [`RFC9190-5.6-1`](#rfc9190-5.6-1), [`RFC9190-5.6-2`](#rfc9190-5.6-2), [`RFC9190-5.6-3`](#rfc9190-5.6-3), [`RFC9190-5.6-4`](#rfc9190-5.6-4), [`RFC9190-5.7-1`](#rfc9190-5.7-1), [`RFC9190-5.7-2`](#rfc9190-5.7-2), [`RFC9190-5.7-3`](#rfc9190-5.7-3), [`RFC9190-5.7-4`](#rfc9190-5.7-4), [`RFC9190-5.7-5`](#rfc9190-5.7-5), [`RFC9190-5.7-6`](#rfc9190-5.7-6), [`RFC9190-5.8-1`](#rfc9190-5.8-1), [`RFC9190-5.8-2`](#rfc9190-5.8-2), [`RFC9190-5.8-3`](#rfc9190-5.8-3), [`RFC9190-5.10-1`](#rfc9190-5.10-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9190-2.1-2` | Early data MUST NOT be used in EAP-TLS (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-3` | EAP-TLS servers MUST NOT send an "early_data" extension (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-4` | Clients MUST NOT send an EndOfEarlyData message (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-5` | Post-handshake authentication MUST NOT be used in EAP-TLS (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-6` | Clients MUST NOT send a "post_handshake_auth" extension (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-7` | Servers MUST NOT request post-handshake client authentication (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-8` | When EAP-TLS is used with TLS 1.3, the formatting and processing of the TLS handshake SHALL be done as specified in version 1.3 of TLS (§2.1) | SHALL | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-1` | The EAP-TLS server MUST authenticate with a certificate (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-2` | Pre-Shared Key authentication SHALL NOT be used except for resumption (§2.1.1) | SHALL NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.2-1` | To enable resumption, the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages in the initial authentication (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.2-2` | EAP-TLS servers MUST respect the 604800 second maximum ticket lifetime when issuing tickets (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.2-3` | The NewSessionTicket message MUST NOT include an "early_data" extension (§2.1.2) | MUST NOT | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.2-4` | If the "early_data" extension is received, then it MUST be ignored (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-1` | When EAP-TLS is used with TLS 1.3, EAP-TLS SHALL use a resumption mechanism compatible with version 1.3 of TLS (§2.1.3) | SHALL | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-2` | The "psk_dh_ke" key exchange mode MUST be used for resumption unless the deployment has a local requirement to allow configuration of other mechanisms (§2.1.3) | MUST | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.4-1` | If the EAP-TLS peer authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.4-2` | If the EAP-TLS server authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.4-3` | Whenever an implementation encounters a fatal error condition, it MUST send an appropriate TLS Error alert (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-1` | EAP-TLS peer and server implementations supporting TLS 1.3 MUST support anonymous Network Access Identifiers (§2.1.8) | MUST | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-2` | A client supporting TLS 1.3 MUST NOT send its username or any other permanent identifier in cleartext in the Identity Response, or in any message used instead of the Identity Response (§2.1.8) | MUST NOT | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-3` | The NAI MUST be a UTF-8 string as defined by the grammar in Section 2.2 of RFC 7542 (§2.1.8) | MUST | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-4` | When EAP-TLS is used with TLS 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the certificate_list processing specified by version 1.3 of TLS (§2.1.8) | SHALL | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.9-1` | Implementations MUST NOT set the L bit in unfragmented messages (§2.1.9) | MUST NOT | 2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.9-2` | Implementations MUST accept unfragmented messages with and without the L bit set (§2.1.9) | MUST | 2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-1` | Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.3-1` | The Key_Material and Method-Id SHALL be derived from the exporter_secret using the TLS exporter interface (§2.3) | SHALL | 2.3 | **positive:** `unit/verify` [`TestRFC9190MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc5216_msk_label_test.go#L159). **negative:** no negative test |
| `RFC9190-2.3-2` | The key derivation MUST use the length values given in the section, 128 octets for Key_Material and 64 octets for Method-Id (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.3-3` | An implementation that intends to use only a part of the TLS-Exporter output MUST ask for the full output and then only use the desired part (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.4-1` | EAP-TLS peers and EAP-TLS servers MUST comply with the compliance requirements defined in Section 9 of RFC 8446 (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.4-2` | In EAP-TLS with TLS 1.3, only cipher suites with confidentiality SHALL be supported (§2.4) | SHALL | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.5-1` | The protected success result indication procedure MUST be followed: after processing the client Finished and sending its last handshake message, the server sends an encrypted TLS record with application data 0x00, then sends no further EAP-Request and may only send EAP-Success (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L167). **negative:** `unit/verify` [`TestEAPTLS12SendsNoProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L298). **negative:** `unit/verify` [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L264). **positive:** `interop/nightly` [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L253). **negative:** `interop/nightly` [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L259) |
| `RFC9190-2.5-2` | The EAP-TLS server MUST NOT send an encrypted TLS record with application data 0x00 before it has successfully processed the client Finished and sent its last handshake message (§2.5) | MUST NOT | 2.5 | **positive:** `unit/verify` [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L175). **negative:** `unit/verify` [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L258) |
| `RFC9190-5.4-1` | When EAP-TLS is used with TLS 1.3, the revocation status of all the certificates in the certificate chains MUST be checked, except the trust anchor (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-2` | EAP-TLS servers supporting TLS 1.3 MUST implement Certificate Status Requests, that is OCSP stapling (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-3` | An EAP-TLS peer using Certificate Status Requests MUST treat a CertificateEntry without a valid CertificateStatus extension as invalid, except the trust anchor, and abort the handshake with an appropriate alert (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-4` | EAP-TLS peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-5` | An EAP peer MUST use a secure transport to verify the revocation status of the server certificate (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-1` | When peer authentication is not used, EAP-TLS server implementations MUST take care to limit network access appropriately for unauthenticated peers (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-2` | Implementations MUST use resumption with caution to ensure that a resumed session is not granted more privilege than was intended for the original session (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-3` | Authorization and accounting MUST be based on authenticated information such as information in the certificate, or the PSK identity and cached data provisioned for resumption (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-4` | The requirements for Network Access Identifiers specified in Section 4 of RFC 7542 still apply and MUST be followed (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-1` | Authorization during resumption MUST be based on cached data from the initial full handshake (§5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-2` | Any security policies for authorization MUST be followed also for resumption (§5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-3` | The EAP-TLS server or EAP client MUST cache data during the initial full handshake sufficient to allow authorization decisions to be made during resumption (§5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-4` | If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7) | MUST NOT | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-5` | EAP-TLS peers MUST NOT store resumption PSKs or tickets, and associated cached data, for longer than 604800 seconds regardless of the PSK or ticket lifetime (§5.7) | MUST NOT | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-6` | If any authorization, accounting, or policy decision was made with information that has changed between the initial full handshake and resumption, and the change may lead to a different decision, that decision MUST be reevaluated (§5.7) | MUST | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-1` | When anonymous NAIs are not used, privacy-friendly identities MUST be generated in a cryptographically secure way, so that an attacker cannot differentiate two identities belonging to the same user from two identities belonging to different users in the same realm (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-2` | Privacy-friendly usernames MUST NOT include substrings that can be used to relate the identity to a specific user (§5.8) | MUST NOT | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-3` | Privacy-friendly usernames MUST NOT be formed by a fixed mapping that stays the same across multiple different authentications (§5.8) | MUST NOT | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.10-1` | EAP-TLS implementations MUST mitigate known attacks (§5.10) | MUST | 5.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-1` | Implementations SHOULD NOT send the KeyUpdate message (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-3` | The EAP-TLS server SHOULD require the EAP-TLS peer to authenticate with a certificate (§2.1.1) | SHOULD | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-3` | The EAP-TLS peer SHOULD supply a "key_share" extension when attempting resumption (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-4` | EAP-TLS peers and EAP-TLS servers SHOULD follow the client tracking preventions in Appendix C.4 of RFC 8446 (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-5` | It is RECOMMENDED that the EAP-TLS peer use resumption if it has a valid ticket that has not been used before (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-6` | It is RECOMMENDED that the EAP-TLS server accept resumption if the ticket that was issued is still valid (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-7` | It is RECOMMENDED to use Network Access Identifiers with the same realm during resumption and the original full handshake (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-8` | When NAI reuse can be done without privacy implications, it is RECOMMENDED to use the same NAI in the resumption as was used in the original full handshake (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.7-1` | When the client certificate contains an NAI as subject name or alternative subject name, an anonymous NAI SHOULD be derived from the NAI in the certificate (§2.1.7) | SHOULD | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.7-2` | It is RECOMMENDED to use anonymous NAIs in the Identity Response (§2.1.7) | RECOMMENDED | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.7-3` | Opaque blob identities are NOT RECOMMENDED because they are not routable (§2.1.7) | NOT RECOMMENDED | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-5` | It is RECOMMENDED to omit the username, so that the NAI is @realm (§2.1.8) | RECOMMENDED | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.9-3` | It is RECOMMENDED to keep the sizes of peer, server, and trust anchor certificates small and the certificate chains short (§2.1.9) | RECOMMENDED | 2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.9-4` | It is RECOMMENDED to use mechanisms that reduce the sizes of Certificate messages (§2.1.9) | RECOMMENDED | 2.1.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-2` | EAP peer implementations SHOULD allow configuration of one or more trusted root certificates and one or more server names to match against the SubjectAltName extension (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-3` | Automated methods of provisioning the root CA certificate and the server name are RECOMMENDED (§2.2) | RECOMMENDED | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.5-3` | TLS Error alerts SHOULD be considered a failure result indication (§2.5) | SHOULD | 2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-6` | An EAP peer implementation SHOULD NOT trust the network, and any services, until it has verified the revocation status of the server certificate after receiving network connectivity (§5.4) | SHOULD NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-7` | An EAP peer SHOULD NOT send any other traffic before revocation checking for the server certificate is complete (§5.4) | SHOULD NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.4-8` | It is RECOMMENDED that EAP-TLS peers and EAP-TLS servers use OCSP stapling for verifying the status of the EAP-TLS server's certificate chain (§5.4) | RECOMMENDED | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-7` | Systems that expect to perform accounting for the session SHOULD cache an identifier that can be used in subsequent accounting (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-8` | If a safe decision is not possible, EAP-TLS servers SHOULD reject the resumption and continue with a full handshake (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-9` | It is RECOMMENDED that authorization, accounting, and policy decisions are reevaluated based on the information given in the resumption (§5.7) | RECOMMENDED | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-4` | EAP-TLS peers SHOULD use record padding to reduce information leakage of certificate sizes (§5.8) | SHOULD | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-5` | An EAP-TLS peer SHOULD NOT continue the EAP authentication attempt if a TLS 1.2 EAP-TLS server sends an EAP-TLS/Request with a TLS alert message in response to an empty certificate message from the peer (§5.8) | SHOULD NOT | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-6` | It is RECOMMENDED for EAP-TLS peers to not use EAP-TLS with TLS 1.2 and static RSA-based cipher suites without privacy (§5.8) | RECOMMENDED | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.11-1` | Using different certificates and resumption caches for different protocols is RECOMMENDED (§5.11) | RECOMMENDED | 5.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-4` | A TLS implementation MAY not allow the EAP-TLS layer to control the order in which things are sent (§2.1.1) | MAY | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-5` | The application data 0x00 MAY therefore be sent before a NewSessionTicket (§2.1.1) | MAY | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-9` | The EAP-TLS server MAY choose to require a full handshake instead of accepting resumption (§2.1.3) | MAY | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-6` | The EAP-TLS server MAY treat an empty certificate_list as a terminal condition (§2.1.8) | MAY | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-4` | The authenticator and the EAP-TLS server MAY examine the identity presented in EAP-Response/Identity for purposes such as routing and EAP method selection (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-5` | EAP-TLS servers MAY reject conversations if the identity does not match their policy (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.2-6` | In the absence of a trusted root CA certificate, EAP peers MAY implement a trust on first use mechanism (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.4-3` | The negotiated cipher suites and algorithms MAY be used to secure data as done in other TLS-based EAP methods (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.5-1` | Protected failure result indications provide integrity and replay protection but MAY be unauthenticated (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-5` | EAP-TLS servers MAY reject conversations based on non-EAP information provided by the encapsulating protocol (§5.6) | MAY | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-6` | EAP peer implementations MAY allow binding the configured acceptable SubjectAltName to a specific CA that should have issued the server certificate (§5.6) | MAY | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-10` | The EAP-TLS peer and EAP-TLS server MAY perform fresh revocation checks on the cached certificate data (§5.7) | MAY | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-11` | If the cached revocation data is not sufficiently current, the EAP-TLS peer or EAP-TLS server MAY force a full TLS handshake (§5.7) | MAY | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-12` | The EAP-TLS peer MAY delete resumption PSKs or tickets earlier based on local policy (§5.7) | MAY | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-13` | The cached data MAY also be removed on the EAP-TLS server or EAP-TLS peer if any certificate in the certificate chain has been revoked or has expired (§5.7) | MAY | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-14` | EAP-TLS servers MAY reject resumption where the information supplied during resumption does not match the information supplied during the original authentication (§5.7) | MAY | 5.7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9190-2.1-2`](#rfc9190-2.1-2) Early data MUST NOT be used in EAP-TLS (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-3`](#rfc9190-2.1-3) EAP-TLS servers MUST NOT send an "early_data" extension (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-4`](#rfc9190-2.1-4) Clients MUST NOT send an EndOfEarlyData message (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-5`](#rfc9190-2.1-5) Post-handshake authentication MUST NOT be used in EAP-TLS (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-6`](#rfc9190-2.1-6) Clients MUST NOT send a "post_handshake_auth" extension (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-7`](#rfc9190-2.1-7) Servers MUST NOT request post-handshake client authentication (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1-8`](#rfc9190-2.1-8) When EAP-TLS is used with TLS 1.3, the formatting and processing of the TLS handshake SHALL be done as specified in version 1.3 of TLS (§2.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1.1-1`](#rfc9190-2.1.1-1) The EAP-TLS server MUST authenticate with a certificate (§2.1.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1.1-2`](#rfc9190-2.1.1-2) Pre-Shared Key authentication SHALL NOT be used except for resumption (§2.1.1) | no test | no test carries this requirement id |
| [`RFC9190-2.1.2-1`](#rfc9190-2.1.2-1) To enable resumption, the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages in the initial authentication (§2.1.2) | no test | no test carries this requirement id |
| [`RFC9190-2.1.2-2`](#rfc9190-2.1.2-2) EAP-TLS servers MUST respect the 604800 second maximum ticket lifetime when issuing tickets (§2.1.2) | no test | no test carries this requirement id |
| [`RFC9190-2.1.2-3`](#rfc9190-2.1.2-3) The NewSessionTicket message MUST NOT include an "early_data" extension (§2.1.2) | no test | no test carries this requirement id |
| [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4) If the "early_data" extension is received, then it MUST be ignored (§2.1.2) | no test | no test carries this requirement id |
| [`RFC9190-2.1.3-1`](#rfc9190-2.1.3-1) When EAP-TLS is used with TLS 1.3, EAP-TLS SHALL use a resumption mechanism compatible with version 1.3 of TLS (§2.1.3) | no test | no test carries this requirement id |
| [`RFC9190-2.1.3-2`](#rfc9190-2.1.3-2) The "psk_dh_ke" key exchange mode MUST be used for resumption unless the deployment has a local requirement to allow configuration of other mechanisms (§2.1.3) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-1`](#rfc9190-2.1.4-1) If the EAP-TLS peer authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-2`](#rfc9190-2.1.4-2) If the EAP-TLS server authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-3`](#rfc9190-2.1.4-3) Whenever an implementation encounters a fatal error condition, it MUST send an appropriate TLS Error alert (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.1.8-1`](#rfc9190-2.1.8-1) EAP-TLS peer and server implementations supporting TLS 1.3 MUST support anonymous Network Access Identifiers (§2.1.8) | no test | no test carries this requirement id |
| [`RFC9190-2.1.8-2`](#rfc9190-2.1.8-2) A client supporting TLS 1.3 MUST NOT send its username or any other permanent identifier in cleartext in the Identity Response, or in any message used instead of the Identity Response (§2.1.8) | no test | no test carries this requirement id |
| [`RFC9190-2.1.8-3`](#rfc9190-2.1.8-3) The NAI MUST be a UTF-8 string as defined by the grammar in Section 2.2 of RFC 7542 (§2.1.8) | no test | no test carries this requirement id |
| [`RFC9190-2.1.8-4`](#rfc9190-2.1.8-4) When EAP-TLS is used with TLS 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the certificate_list processing specified by version 1.3 of TLS (§2.1.8) | no test | no test carries this requirement id |
| [`RFC9190-2.1.9-1`](#rfc9190-2.1.9-1) Implementations MUST NOT set the L bit in unfragmented messages (§2.1.9) | no test | no test carries this requirement id |
| [`RFC9190-2.1.9-2`](#rfc9190-2.1.9-2) Implementations MUST accept unfragmented messages with and without the L bit set (§2.1.9) | no test | no test carries this requirement id |
| [`RFC9190-2.2-1`](#rfc9190-2.2-1) Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2) | no test | no test carries this requirement id |
| [`RFC9190-2.3-2`](#rfc9190-2.3-2) The key derivation MUST use the length values given in the section, 128 octets for Key_Material and 64 octets for Method-Id (§2.3) | no test | no test carries this requirement id |
| [`RFC9190-2.3-3`](#rfc9190-2.3-3) An implementation that intends to use only a part of the TLS-Exporter output MUST ask for the full output and then only use the desired part (§2.3) | no test | no test carries this requirement id |
| [`RFC9190-2.4-1`](#rfc9190-2.4-1) EAP-TLS peers and EAP-TLS servers MUST comply with the compliance requirements defined in Section 9 of RFC 8446 (§2.4) | no test | no test carries this requirement id |
| [`RFC9190-2.4-2`](#rfc9190-2.4-2) In EAP-TLS with TLS 1.3, only cipher suites with confidentiality SHALL be supported (§2.4) | no test | no test carries this requirement id |
| [`RFC9190-5.4-1`](#rfc9190-5.4-1) When EAP-TLS is used with TLS 1.3, the revocation status of all the certificates in the certificate chains MUST be checked, except the trust anchor (§5.4) | no test | no test carries this requirement id |
| [`RFC9190-5.4-2`](#rfc9190-5.4-2) EAP-TLS servers supporting TLS 1.3 MUST implement Certificate Status Requests, that is OCSP stapling (§5.4) | no test | no test carries this requirement id |
| [`RFC9190-5.4-3`](#rfc9190-5.4-3) An EAP-TLS peer using Certificate Status Requests MUST treat a CertificateEntry without a valid CertificateStatus extension as invalid, except the trust anchor, and abort the handshake with an appropriate alert (§5.4) | no test | no test carries this requirement id |
| [`RFC9190-5.4-4`](#rfc9190-5.4-4) EAP-TLS peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4) | no test | no test carries this requirement id |
| [`RFC9190-5.4-5`](#rfc9190-5.4-5) An EAP peer MUST use a secure transport to verify the revocation status of the server certificate (§5.4) | no test | no test carries this requirement id |
| [`RFC9190-5.6-1`](#rfc9190-5.6-1) When peer authentication is not used, EAP-TLS server implementations MUST take care to limit network access appropriately for unauthenticated peers (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-2`](#rfc9190-5.6-2) Implementations MUST use resumption with caution to ensure that a resumed session is not granted more privilege than was intended for the original session (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-3`](#rfc9190-5.6-3) Authorization and accounting MUST be based on authenticated information such as information in the certificate, or the PSK identity and cached data provisioned for resumption (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-4`](#rfc9190-5.6-4) The requirements for Network Access Identifiers specified in Section 4 of RFC 7542 still apply and MUST be followed (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.7-1`](#rfc9190-5.7-1) Authorization during resumption MUST be based on cached data from the initial full handshake (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.7-2`](#rfc9190-5.7-2) Any security policies for authorization MUST be followed also for resumption (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.7-3`](#rfc9190-5.7-3) The EAP-TLS server or EAP client MUST cache data during the initial full handshake sufficient to allow authorization decisions to be made during resumption (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.7-4`](#rfc9190-5.7-4) If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.7-5`](#rfc9190-5.7-5) EAP-TLS peers MUST NOT store resumption PSKs or tickets, and associated cached data, for longer than 604800 seconds regardless of the PSK or ticket lifetime (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.7-6`](#rfc9190-5.7-6) If any authorization, accounting, or policy decision was made with information that has changed between the initial full handshake and resumption, and the change may lead to a different decision, that decision MUST be reevaluated (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.8-1`](#rfc9190-5.8-1) When anonymous NAIs are not used, privacy-friendly identities MUST be generated in a cryptographically secure way, so that an attacker cannot differentiate two identities belonging to the same user from two identities belonging to different users in the same realm (§5.8) | no test | no test carries this requirement id |
| [`RFC9190-5.8-2`](#rfc9190-5.8-2) Privacy-friendly usernames MUST NOT include substrings that can be used to relate the identity to a specific user (§5.8) | no test | no test carries this requirement id |
| [`RFC9190-5.8-3`](#rfc9190-5.8-3) Privacy-friendly usernames MUST NOT be formed by a fixed mapping that stays the same across multiple different authentications (§5.8) | no test | no test carries this requirement id |
| [`RFC9190-5.10-1`](#rfc9190-5.10-1) EAP-TLS implementations MUST mitigate known attacks (§5.10) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9190-2.1-2`](#rfc9190-2.1-2)

Early data MUST NOT be used in EAP-TLS (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-2, so no unit is bound to it.

### [`RFC9190-2.1-3`](#rfc9190-2.1-3)

EAP-TLS servers MUST NOT send an "early_data" extension (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-3, so no unit is bound to it.

### [`RFC9190-2.1-4`](#rfc9190-2.1-4)

Clients MUST NOT send an EndOfEarlyData message (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-4, so no unit is bound to it.

### [`RFC9190-2.1-5`](#rfc9190-2.1-5)

Post-handshake authentication MUST NOT be used in EAP-TLS (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-5, so no unit is bound to it.

### [`RFC9190-2.1-6`](#rfc9190-2.1-6)

Clients MUST NOT send a "post_handshake_auth" extension (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-6, so no unit is bound to it.

### [`RFC9190-2.1-7`](#rfc9190-2.1-7)

Servers MUST NOT request post-handshake client authentication (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-7, so no unit is bound to it.

### [`RFC9190-2.1-8`](#rfc9190-2.1-8)

When EAP-TLS is used with TLS 1.3, the formatting and processing of the TLS handshake SHALL be done as specified in version 1.3 of TLS (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1-8, so no unit is bound to it.

### [`RFC9190-2.1.1-1`](#rfc9190-2.1.1-1)

The EAP-TLS server MUST authenticate with a certificate (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.1-1, so no unit is bound to it.

### [`RFC9190-2.1.1-2`](#rfc9190-2.1.1-2)

Pre-Shared Key authentication SHALL NOT be used except for resumption (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.1-2, so no unit is bound to it.

### [`RFC9190-2.1.2-1`](#rfc9190-2.1.2-1)

To enable resumption, the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages in the initial authentication (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.2-1, so no unit is bound to it.

### [`RFC9190-2.1.2-2`](#rfc9190-2.1.2-2)

EAP-TLS servers MUST respect the 604800 second maximum ticket lifetime when issuing tickets (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.2-2, so no unit is bound to it.

### [`RFC9190-2.1.2-3`](#rfc9190-2.1.2-3)

The NewSessionTicket message MUST NOT include an "early_data" extension (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.2-3, so no unit is bound to it.

### [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4)

If the "early_data" extension is received, then it MUST be ignored (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.2-4, so no unit is bound to it.

### [`RFC9190-2.1.3-1`](#rfc9190-2.1.3-1)

When EAP-TLS is used with TLS 1.3, EAP-TLS SHALL use a resumption mechanism compatible with version 1.3 of TLS (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.3-1, so no unit is bound to it.

### [`RFC9190-2.1.3-2`](#rfc9190-2.1.3-2)

The "psk_dh_ke" key exchange mode MUST be used for resumption unless the deployment has a local requirement to allow configuration of other mechanisms (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.3-2, so no unit is bound to it.

### [`RFC9190-2.1.4-1`](#rfc9190-2.1.4-1)

If the EAP-TLS peer authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.4-1, so no unit is bound to it.

### [`RFC9190-2.1.4-2`](#rfc9190-2.1.4-2)

If the EAP-TLS server authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.4-2, so no unit is bound to it.

### [`RFC9190-2.1.4-3`](#rfc9190-2.1.4-3)

Whenever an implementation encounters a fatal error condition, it MUST send an appropriate TLS Error alert (§2.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.4-3, so no unit is bound to it.

### [`RFC9190-2.1.8-1`](#rfc9190-2.1.8-1)

EAP-TLS peer and server implementations supporting TLS 1.3 MUST support anonymous Network Access Identifiers (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.8-1, so no unit is bound to it.

### [`RFC9190-2.1.8-2`](#rfc9190-2.1.8-2)

A client supporting TLS 1.3 MUST NOT send its username or any other permanent identifier in cleartext in the Identity Response, or in any message used instead of the Identity Response (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.8-2, so no unit is bound to it.

### [`RFC9190-2.1.8-3`](#rfc9190-2.1.8-3)

The NAI MUST be a UTF-8 string as defined by the grammar in Section 2.2 of RFC 7542 (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.8-3, so no unit is bound to it.

### [`RFC9190-2.1.8-4`](#rfc9190-2.1.8-4)

When EAP-TLS is used with TLS 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the certificate_list processing specified by version 1.3 of TLS (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.8-4, so no unit is bound to it.

### [`RFC9190-2.1.9-1`](#rfc9190-2.1.9-1)

Implementations MUST NOT set the L bit in unfragmented messages (§2.1.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.9-1, so no unit is bound to it.

### [`RFC9190-2.1.9-2`](#rfc9190-2.1.9-2)

Implementations MUST accept unfragmented messages with and without the L bit set (§2.1.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.9-2, so no unit is bound to it.

### [`RFC9190-2.2-1`](#rfc9190-2.2-1)

Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.2-1, so no unit is bound to it.

### [`RFC9190-2.3-1`](#rfc9190-2.3-1)

The Key_Material and Method-Id SHALL be derived from the exporter_secret using the TLS exporter interface (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9190MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc5216_msk_label_test.go#L159) | unit/verify | unproven |

### [`RFC9190-2.3-2`](#rfc9190-2.3-2)

The key derivation MUST use the length values given in the section, 128 octets for Key_Material and 64 octets for Method-Id (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.3-2, so no unit is bound to it.

### [`RFC9190-2.3-3`](#rfc9190-2.3-3)

An implementation that intends to use only a part of the TLS-Exporter output MUST ask for the full output and then only use the desired part (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.3-3, so no unit is bound to it.

### [`RFC9190-2.4-1`](#rfc9190-2.4-1)

EAP-TLS peers and EAP-TLS servers MUST comply with the compliance requirements defined in Section 9 of RFC 8446 (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.4-1, so no unit is bound to it.

### [`RFC9190-2.4-2`](#rfc9190-2.4-2)

In EAP-TLS with TLS 1.3, only cipher suites with confidentiality SHALL be supported (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.4-2, so no unit is bound to it.

### [`RFC9190-2.5-1`](#rfc9190-2.5-1)

The protected success result indication procedure MUST be followed: after processing the client Finished and sending its last handshake message, the server sends an encrypted TLS record with application data 0x00, then sends no further EAP-Request and may only send EAP-Success (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS12SendsNoProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L298) | unit/verify | unproven |
| negative | [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L264) | unit/verify | unproven |
| negative | [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L259) | interop/nightly | unproven |
| positive | [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L167) | unit/verify | unproven |
| positive | [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L253) | interop/nightly | unproven |

### [`RFC9190-2.5-2`](#rfc9190-2.5-2)

The EAP-TLS server MUST NOT send an encrypted TLS record with application data 0x00 before it has successfully processed the client Finished and sent its last handshake message (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L258) | unit/verify | unproven |
| positive | [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/component/ike/eap/rfc9190_test.go#L175) | unit/verify | unproven |

### [`RFC9190-5.4-1`](#rfc9190-5.4-1)

When EAP-TLS is used with TLS 1.3, the revocation status of all the certificates in the certificate chains MUST be checked, except the trust anchor (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.4-1, so no unit is bound to it.

### [`RFC9190-5.4-2`](#rfc9190-5.4-2)

EAP-TLS servers supporting TLS 1.3 MUST implement Certificate Status Requests, that is OCSP stapling (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.4-2, so no unit is bound to it.

### [`RFC9190-5.4-3`](#rfc9190-5.4-3)

An EAP-TLS peer using Certificate Status Requests MUST treat a CertificateEntry without a valid CertificateStatus extension as invalid, except the trust anchor, and abort the handshake with an appropriate alert (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.4-3, so no unit is bound to it.

### [`RFC9190-5.4-4`](#rfc9190-5.4-4)

EAP-TLS peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.4-4, so no unit is bound to it.

### [`RFC9190-5.4-5`](#rfc9190-5.4-5)

An EAP peer MUST use a secure transport to verify the revocation status of the server certificate (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.4-5, so no unit is bound to it.

### [`RFC9190-5.6-1`](#rfc9190-5.6-1)

When peer authentication is not used, EAP-TLS server implementations MUST take care to limit network access appropriately for unauthenticated peers (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.6-1, so no unit is bound to it.

### [`RFC9190-5.6-2`](#rfc9190-5.6-2)

Implementations MUST use resumption with caution to ensure that a resumed session is not granted more privilege than was intended for the original session (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.6-2, so no unit is bound to it.

### [`RFC9190-5.6-3`](#rfc9190-5.6-3)

Authorization and accounting MUST be based on authenticated information such as information in the certificate, or the PSK identity and cached data provisioned for resumption (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.6-3, so no unit is bound to it.

### [`RFC9190-5.6-4`](#rfc9190-5.6-4)

The requirements for Network Access Identifiers specified in Section 4 of RFC 7542 still apply and MUST be followed (§5.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.6-4, so no unit is bound to it.

### [`RFC9190-5.7-1`](#rfc9190-5.7-1)

Authorization during resumption MUST be based on cached data from the initial full handshake (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-1, so no unit is bound to it.

### [`RFC9190-5.7-2`](#rfc9190-5.7-2)

Any security policies for authorization MUST be followed also for resumption (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-2, so no unit is bound to it.

### [`RFC9190-5.7-3`](#rfc9190-5.7-3)

The EAP-TLS server or EAP client MUST cache data during the initial full handshake sufficient to allow authorization decisions to be made during resumption (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-3, so no unit is bound to it.

### [`RFC9190-5.7-4`](#rfc9190-5.7-4)

If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-4, so no unit is bound to it.

### [`RFC9190-5.7-5`](#rfc9190-5.7-5)

EAP-TLS peers MUST NOT store resumption PSKs or tickets, and associated cached data, for longer than 604800 seconds regardless of the PSK or ticket lifetime (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-5, so no unit is bound to it.

### [`RFC9190-5.7-6`](#rfc9190-5.7-6)

If any authorization, accounting, or policy decision was made with information that has changed between the initial full handshake and resumption, and the change may lead to a different decision, that decision MUST be reevaluated (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-6, so no unit is bound to it.

### [`RFC9190-5.8-1`](#rfc9190-5.8-1)

When anonymous NAIs are not used, privacy-friendly identities MUST be generated in a cryptographically secure way, so that an attacker cannot differentiate two identities belonging to the same user from two identities belonging to different users in the same realm (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.8-1, so no unit is bound to it.

### [`RFC9190-5.8-2`](#rfc9190-5.8-2)

Privacy-friendly usernames MUST NOT include substrings that can be used to relate the identity to a specific user (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.8-2, so no unit is bound to it.

### [`RFC9190-5.8-3`](#rfc9190-5.8-3)

Privacy-friendly usernames MUST NOT be formed by a fixed mapping that stays the same across multiple different authentications (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.8-3, so no unit is bound to it.

### [`RFC9190-5.10-1`](#rfc9190-5.10-1)

EAP-TLS implementations MUST mitigate known attacks (§5.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.10-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9190, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9190, so its obligations are stated where they were written.
