# RFC 9190 - EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3

No row in the public ledger. Every requirement this repository extracted from RFC 9190, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 36.5% | 19 of 52 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 52 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| Proven by a recorded break | 90.5% | 86 of 95 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 52 | of 95 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 52 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 52 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 52 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 52 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| One polarity, unexcused | 13.5% | 7 of 52 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 50.0% | 26 of 52 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 52 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | bad | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (backlog) |
| Requirements | 95 |
| Gated MUST-level | 52 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 26 |
| Nightly-only evidence | 0 |
| Test tags | 95 |
| Tagged units | 95 |
| Recorded audit verdicts | 0 |
| Discrimination records | 86 |
| Summary | `rfc/short/rfc9190.md` |
| Requirement shard | `rfc/requirements/rfc9190.md` |
| RFC text | `rfc/full/rfc9190.txt` |

## Enrolment

Not enrolled (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): EAP-TLS 1.3: Using EAP with TLS 1.3. Summary written 2026-08-01, extraction sign-off walked 2026-09-08 (rfc/extraction/rfc9190.json, 52 sites in 36 sections, register prose, 48 mapped and 4 excluded). It declares 52 MUST-level obligations over 20 sections. It is NOT enrolled because 33 of them are not proven in both polarities: 26 carry no tagged test at all and 7 carry a positive only. Measured 2026-09-08 by counting `RFC requirement: RFC9190-<id> <polarity>` tags under internal/, test/, cmd/ and pkg/ against the gated rows of this checklist. Enrolment demands every gated MUST proven in both polarities or annotated, and annotating is the conformance judgement ai/rules/rfc-compliance.md reserves to the owner; the owner ruling of 2026-08-01 chose to implement the features and enrol with everything proven, so the annotation route is closed. WHAT IS BUILT AND PROVEN, all in both polarities. Section 2.3 key derivation: exportEAPTLSMSK (internal/core/eap/eap_tls.go) selects the exporter label EXPORTER_EAP_TLS_Key_Material, the EAP Type octet as context and the 128-octet length whenever the negotiated version is TLS 1.3, and test/interop-ipsec/scenarios/eap-tls13 exercises that path against strongSwan. Section 2.5 protected success indication, both roles: tlsMethod.indicateSuccess (same file) writes the encrypted TLS record carrying application data 0x00 in the round that completes the handshake, and PeerSession.requireSuccessIndication (internal/core/eap/peer_indication.go) refuses the EAP-Success without it; scenario responder-eap-tls13 proves the server half against strongSwan, which logs `missing protected success indication for EAP-TLS with TLS 1.3` when the write is reverted. The peer half is STRICTER than the published RFC, which addresses Section 2.5 only to the server, so no requirement id covers it and its tests carry no RFC requirement tag. Section 2.1.2 and 2.1.3 resumption, both roles: Resumption (internal/core/eap/resumption.go) owns the ticket keys and the client cache per peering, and serverChainCheck.rebuildResumedChains (internal/core/eap/peer_chain.go) rebuilds the chain crypto/tls skips on a resumed handshake so the Section 5.4 check still runs. ALL FIVE Section 5.4 requirements are built as of 2026-09-08: checkChainRevocation (internal/core/eap/revocation.go) walks every certificate on each chain except the trust anchor on both roles (5.4-1), newTLSMethod staples the operator's ocsp-response (5.4-2), checkStapledChainStatus (internal/core/eap/ocsp.go) refuses a CertificateEntry with no valid status while certificate-status-request is set (5.4-3), and startServerCertRecheck (internal/component/ike/engine/postauth_revocation.go) re-checks the chain over https once the Child SA is up, refusing an http responder URL before any connection opens (5.4-4 and 5.4-5). Section 5.10-1 is proven by sixteen tagged units in internal/core/eap/rfc9190_attack_mitigation_test.go over the RFC 7457 Section 2 attacks. THE TWO OBLIGATIONS THAT WERE UNMET IN CODE ARE MET AND PROVEN AS OF 2026-09-08. RFC9190-2.1.9-1, the MUST NOT to set the L bit in an unfragmented message: tlsFragmenter.nextFragment (internal/core/eap/eap_tls.go) is the one producer of outbound EAP-TLS TypeData on both roles, and it now declares a length only where the first fragment is not also the last, so a message that fits in one fragment leaves as a bare flags octet with its TLS data at offset 1. RFC9190-2.1.9-2, the receive half of the same sentence, is proven beside it over tlsFragmenter.reassemble, which takes an unfragmented message with the L bit and without it. RFC9190-1-1, the Section 1 MUST to limit the maximum TLS version to 1.3 unless the administrator enables a later one: newTLSMethod (eap_tls.go) and PeerSession.tlsClientConfig (internal/core/eap/peer.go) each set MaxVersion to tls.VersionTLS13, and ze exposes no leaf that raises that ceiling, so the exception has nothing to switch on. RFC9190-1-1 was added to this checklist by the 2026-09-08 extraction walk, which found the summary had missed it.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9190.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 19 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 7 | one part of the gated population |
| No test and no annotation | 26 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **52** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (19):** [`RFC9190-1-1`](#rfc9190-1-1), [`RFC9190-2.1.2-1`](#rfc9190-2.1.2-1), [`RFC9190-2.1.3-1`](#rfc9190-2.1.3-1), [`RFC9190-2.1.8-2`](#rfc9190-2.1.8-2), [`RFC9190-2.1.8-3`](#rfc9190-2.1.8-3), [`RFC9190-2.1.9-1`](#rfc9190-2.1.9-1), [`RFC9190-2.1.9-2`](#rfc9190-2.1.9-2), [`RFC9190-2.5-1`](#rfc9190-2.5-1), [`RFC9190-2.5-2`](#rfc9190-2.5-2), [`RFC9190-5.4-1`](#rfc9190-5.4-1), [`RFC9190-5.4-2`](#rfc9190-5.4-2), [`RFC9190-5.4-3`](#rfc9190-5.4-3), [`RFC9190-5.4-4`](#rfc9190-5.4-4), [`RFC9190-5.4-5`](#rfc9190-5.4-5), [`RFC9190-5.7-1`](#rfc9190-5.7-1), [`RFC9190-5.7-2`](#rfc9190-5.7-2), [`RFC9190-5.7-5`](#rfc9190-5.7-5), [`RFC9190-5.7-6`](#rfc9190-5.7-6), [`RFC9190-5.10-1`](#rfc9190-5.10-1)

**One polarity only (7):** [`RFC9190-2.1.2-2`](#rfc9190-2.1.2-2), [`RFC9190-2.1.2-3`](#rfc9190-2.1.2-3), [`RFC9190-2.1.3-2`](#rfc9190-2.1.3-2), [`RFC9190-2.1.8-1`](#rfc9190-2.1.8-1), [`RFC9190-2.1.8-4`](#rfc9190-2.1.8-4), [`RFC9190-2.3-1`](#rfc9190-2.3-1), [`RFC9190-5.7-3`](#rfc9190-5.7-3)

**No test and no annotation (26):** [`RFC9190-2.1-2`](#rfc9190-2.1-2), [`RFC9190-2.1-3`](#rfc9190-2.1-3), [`RFC9190-2.1-4`](#rfc9190-2.1-4), [`RFC9190-2.1-5`](#rfc9190-2.1-5), [`RFC9190-2.1-6`](#rfc9190-2.1-6), [`RFC9190-2.1-7`](#rfc9190-2.1-7), [`RFC9190-2.1-8`](#rfc9190-2.1-8), [`RFC9190-2.1.1-1`](#rfc9190-2.1.1-1), [`RFC9190-2.1.1-2`](#rfc9190-2.1.1-2), [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4), [`RFC9190-2.1.4-1`](#rfc9190-2.1.4-1), [`RFC9190-2.1.4-2`](#rfc9190-2.1.4-2), [`RFC9190-2.1.4-3`](#rfc9190-2.1.4-3), [`RFC9190-2.2-1`](#rfc9190-2.2-1), [`RFC9190-2.3-2`](#rfc9190-2.3-2), [`RFC9190-2.3-3`](#rfc9190-2.3-3), [`RFC9190-2.4-1`](#rfc9190-2.4-1), [`RFC9190-2.4-2`](#rfc9190-2.4-2), [`RFC9190-5.6-1`](#rfc9190-5.6-1), [`RFC9190-5.6-2`](#rfc9190-5.6-2), [`RFC9190-5.6-3`](#rfc9190-5.6-3), [`RFC9190-5.6-4`](#rfc9190-5.6-4), [`RFC9190-5.7-4`](#rfc9190-5.7-4), [`RFC9190-5.8-1`](#rfc9190-5.8-1), [`RFC9190-5.8-2`](#rfc9190-5.8-2), [`RFC9190-5.8-3`](#rfc9190-5.8-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9190-1-1` | Implementations MUST limit the maximum TLS version they use to 1.3, unless later versions are explicitly enabled by the administrator (§1) | MUST | 1 | **positive:** `unit/verify` [`TestEAPTLSCapsBothRolesAtTLS13`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_version_cap_test.go#L51). **negative:** `unit/verify` [`TestEAPTLSVersionCapLeavesTLS12Reachable`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_version_cap_test.go#L110) |
| `RFC9190-2.1-2` | Early data MUST NOT be used in EAP-TLS (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-3` | EAP-TLS servers MUST NOT send an "early_data" extension (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-4` | Clients MUST NOT send an EndOfEarlyData message (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-5` | Post-handshake authentication MUST NOT be used in EAP-TLS (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-6` | Clients MUST NOT send a "post_handshake_auth" extension (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-7` | Servers MUST NOT request post-handshake client authentication (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1-8` | When EAP-TLS is used with TLS 1.3, the formatting and processing of the TLS handshake SHALL be done as specified in version 1.3 of TLS (§2.1) | SHALL | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-1` | The EAP-TLS server MUST authenticate with a certificate (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-2` | Pre-Shared Key authentication SHALL NOT be used except for resumption (§2.1.1) | SHALL NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.2-1` | To enable resumption, the EAP-TLS server MUST send one or more post-handshake NewSessionTicket messages in the initial authentication (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L105). **negative:** `unit/verify` [`TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L60). **negative:** `unit/verify` [`TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L327) |
| `RFC9190-2.1.2-2` | EAP-TLS servers MUST respect the 604800 second maximum ticket lifetime when issuing tickets (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L110). **negative:** no negative test |
| `RFC9190-2.1.2-3` | The NewSessionTicket message MUST NOT include an "early_data" extension (§2.1.2) | MUST NOT | 2.1.2 | **positive:** `unit/verify` [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L117). **negative:** no negative test |
| `RFC9190-2.1.2-4` | If the "early_data" extension is received, then it MUST be ignored (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-1` | When EAP-TLS is used with TLS 1.3, EAP-TLS SHALL use a resumption mechanism compatible with version 1.3 of TLS (§2.1.3) | SHALL | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L121). **negative:** `unit/verify` [`TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L332) |
| `RFC9190-2.1.3-2` | The "psk_dh_ke" key exchange mode MUST be used for resumption unless the deployment has a local requirement to allow configuration of other mechanisms (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L125). **negative:** no negative test |
| `RFC9190-2.1.4-1` | If the EAP-TLS peer authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.4-2` | If the EAP-TLS server authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.4-3` | Whenever an implementation encounters a fatal error condition, it MUST send an appropriate TLS Error alert (§2.1.4) | MUST | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-1` | EAP-TLS peer and server implementations supporting TLS 1.3 MUST support anonymous Network Access Identifiers (§2.1.8) | MUST | 2.1.8 | **positive:** `unit/verify` [`TestEAPTLS13AuthenticatorAcceptsAnAnonymousNAI`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L259). **negative:** no negative test |
| `RFC9190-2.1.8-2` | A client supporting TLS 1.3 MUST NOT send its username or any other permanent identifier in cleartext in the Identity Response, or in any message used instead of the Identity Response (§2.1.8) | MUST NOT | 2.1.8 | **positive:** `unit/verify` [`TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L70). **positive:** `unit/verify` [`TestEAPTLSPeerDropsTheUsernameTheCertificateCarries`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L373). **negative:** `unit/verify` [`TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L237) |
| `RFC9190-2.1.8-3` | The NAI MUST be a UTF-8 string as defined by the grammar in Section 2.2 of RFC 7542 (§2.1.8) | MUST | 2.1.8 | **positive:** `unit/verify` [`TestEAPTLSPeerAnonymizesEveryConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L131). **negative:** `unit/verify` [`TestNAIGrammarMatchesRFC7542Section22`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L182) |
| `RFC9190-2.1.8-4` | When EAP-TLS is used with TLS 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the certificate_list processing specified by version 1.3 of TLS (§2.1.8) | SHALL | 2.1.8 | **positive:** `unit/verify` [`TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L362). **negative:** no negative test |
| `RFC9190-2.1.9-1` | Implementations MUST NOT set the L bit in unfragmented messages (§2.1.9) | MUST NOT | 2.1.9 | **positive:** `unit/verify` [`TestEAPTLSKeepsTheLengthBitOnAFragmentedMessage`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L145). **negative:** `unit/verify` [`TestEAPTLSSetsNoLengthBitOnAnUnfragmentedMessage`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L73) |
| `RFC9190-2.1.9-2` | Implementations MUST accept unfragmented messages with and without the L bit set (§2.1.9) | MUST | 2.1.9 | **positive:** `unit/verify` [`TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L202). **negative:** `unit/verify` [`TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L244) |
| `RFC9190-2.2-1` | Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.3-1` | The Key_Material and Method-Id SHALL be derived from the exporter_secret using the TLS exporter interface (§2.3) | SHALL | 2.3 | **positive:** `unit/verify` [`TestRFC9190MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L158). **negative:** no negative test |
| `RFC9190-2.3-2` | The key derivation MUST use the length values given in the section, 128 octets for Key_Material and 64 octets for Method-Id (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.3-3` | An implementation that intends to use only a part of the TLS-Exporter output MUST ask for the full output and then only use the desired part (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.4-1` | EAP-TLS peers and EAP-TLS servers MUST comply with the compliance requirements defined in Section 9 of RFC 8446 (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.4-2` | In EAP-TLS with TLS 1.3, only cipher suites with confidentiality SHALL be supported (§2.4) | SHALL | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.5-1` | The protected success result indication procedure MUST be followed: after processing the client Finished and sending its last handshake message, the server sends an encrypted TLS record with application data 0x00, then sends no further EAP-Request and may only send EAP-Success (§2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L369). **positive:** `unit/verify` [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L156). **negative:** `unit/verify` [`TestEAPTLS12SendsNoProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L289). **negative:** `unit/verify` [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L254). **positive:** `interop/nightly` [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L260). **negative:** `interop/nightly` [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L266) |
| `RFC9190-2.5-2` | The EAP-TLS server MUST NOT send an encrypted TLS record with application data 0x00 before it has successfully processed the client Finished and sent its last handshake message (§2.5) | MUST NOT | 2.5 | **positive:** `unit/verify` [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L164). **negative:** `unit/verify` [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L248) |
| `RFC9190-5.4-1` | When EAP-TLS is used with TLS 1.3, the revocation status of all the certificates in the certificate chains MUST be checked, except the trust anchor (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS13ExceptsTheTrustAnchorFromRevocation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L264). **positive:** `unit/verify` [`TestEAPTLS13RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L52). **positive:** `unit/verify` [`TestEAPTLS13RefusesARevokedIntermediate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L162). **positive:** `unit/verify` [`TestEAPTLS13RefusesARevokedServerCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L128). **positive:** `unit/verify` [`TestEAPTLS13RefusesAStaleRevocationList`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L235). **positive:** `unit/verify` [`TestEAPTLS13RefusesAnUncheckableChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L206). **negative:** `unit/verify` [`TestEAPTLS12CompletesWithNoRevocationList`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L323). **negative:** `unit/verify` [`TestEAPTLS13CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L297). **negative:** `unit/verify` [`TestEAPTLS13ResumptionStillNeedsARevocationSource`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L209). **positive:** `interop/nightly` [`checkResponderEAPTLS13RevokedClient`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L303) |
| `RFC9190-5.4-2` | EAP-TLS servers supporting TLS 1.3 MUST implement Certificate Status Requests, that is OCSP stapling (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS13StaplesTheConfiguredOCSPResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L163). **negative:** `unit/verify` [`TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L198) |
| `RFC9190-5.4-3` | An EAP-TLS peer using Certificate Status Requests MUST treat a CertificateEntry without a valid CertificateStatus extension as invalid, except the trust anchor, and abort the handshake with an appropriate alert (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestCertificateStatusRefusesAResponseAboutAnotherCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L502). **positive:** `unit/verify` [`TestCertificateStatusRefusesAResponseDatedAhead`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L481). **positive:** `unit/verify` [`TestCertificateStatusRefusesAnExpiredResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L460). **positive:** `unit/verify` [`TestCertificateStatusRefusesAnUndelegatedResponder`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L540). **positive:** `unit/verify` [`TestCertificateStatusRefusesAnUnknownStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L520). **positive:** `unit/verify` [`TestEAPTLS13PeerRefusesARevokedStapledStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L282). **positive:** `unit/verify` [`TestEAPTLS13PeerRefusesAnAuthenticatorThatStaplesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L221). **positive:** `unit/verify` [`TestEAPTLS13PeerRefusesAnIntermediateItCannotReadTheStatusOf`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L334). **positive:** `unit/verify` [`TestStapledChainStatusRefusesAnEmptyChainSet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L596). **negative:** `unit/verify` [`TestCertificateStatusAcceptsADelegatedResponder`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L562). **negative:** `unit/verify` [`TestEAPTLS13PeerCompletesWithAValidStapledStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L252). **negative:** `unit/verify` [`TestEAPTLS13PeerWithoutTheStatusLeafAcceptsAnAuthenticatorThatStaplesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L312). **negative:** `unit/verify` [`TestStapledChainStatusExceptsTheTrustAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L581) |
| `RFC9190-5.4-4` | EAP-TLS peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS13PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L377). **positive:** `unit/verify` [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L248). **negative:** `unit/verify` [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L278) |
| `RFC9190-5.4-5` | An EAP peer MUST use a secure transport to verify the revocation status of the server certificate (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestPostAuthenticationCheckRefusesAnInsecureResponderURL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L323). **negative:** `unit/verify` [`TestPostAuthenticationCheckReadsAnHTTPSResponder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L348) |
| `RFC9190-5.6-1` | When peer authentication is not used, EAP-TLS server implementations MUST take care to limit network access appropriately for unauthenticated peers (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-2` | Implementations MUST use resumption with caution to ensure that a resumed session is not granted more privilege than was intended for the original session (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-3` | Authorization and accounting MUST be based on authenticated information such as information in the certificate, or the PSK identity and cached data provisioned for resumption (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.6-4` | The requirements for Network Access Identifiers specified in Section 4 of RFC 7542 still apply and MUST be followed (§5.6) | MUST | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-1` | Authorization during resumption MUST be based on cached data from the initial full handshake (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestEAPTLS13ResumptionRefusesARevokedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L227). **negative:** `unit/verify` [`TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L90) |
| `RFC9190-5.7-2` | Any security policies for authorization MUST be followed also for resumption (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestEAPTLS13PeerRefusesAResumptionItCannotRebuildAChainFor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L246). **positive:** `unit/verify` [`TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L183). **negative:** `unit/verify` [`TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L261). **negative:** `unit/verify` [`TestEAPTLS13RefusesAResumptionAgainstAReplacedTrustAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L176) |
| `RFC9190-5.7-3` | The EAP-TLS server or EAP client MUST cache data during the initial full handshake sufficient to allow authorization decisions to be made during resumption (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestEAPTLS13ResumptionRefusesARevokedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L232). **negative:** no negative test |
| `RFC9190-5.7-4` | If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7) | MUST NOT | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.7-5` | EAP-TLS peers MUST NOT store resumption PSKs or tickets, and associated cached data, for longer than 604800 seconds regardless of the PSK or ticket lifetime (§5.7) | MUST NOT | 5.7 | **positive:** `unit/verify` [`TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L285). **negative:** `unit/verify` [`TestEAPTLS13RefusesATicketPastTheSection57Lifetime`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L119). **negative:** `unit/verify` [`TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L290) |
| `RFC9190-5.7-6` | If any authorization, accounting, or policy decision was made with information that has changed between the initial full handshake and resumption, and the change may lead to a different decision, that decision MUST be reevaluated (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L143). **positive:** `unit/verify` [`TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L188). **negative:** `unit/verify` [`TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L265) |
| `RFC9190-5.8-1` | When anonymous NAIs are not used, privacy-friendly identities MUST be generated in a cryptographically secure way, so that an attacker cannot differentiate two identities belonging to the same user from two identities belonging to different users in the same realm (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-2` | Privacy-friendly usernames MUST NOT include substrings that can be used to relate the identity to a specific user (§5.8) | MUST NOT | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.8-3` | Privacy-friendly usernames MUST NOT be formed by a fixed mapping that stays the same across multiple different authentications (§5.8) | MUST NOT | 5.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-5.10-1` | EAP-TLS implementations MUST mitigate known attacks (§5.10) | MUST | 5.10 | **positive:** `unit/verify` [`TestRFC9190MitigationCarriesNoServerName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L861). **positive:** `unit/verify` [`TestRFC9190MitigationNegotiatesAnAEADSuite`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L680). **positive:** `unit/verify` [`TestRFC9190MitigationNegotiatesEphemeralKeyExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L616). **positive:** `unit/verify` [`TestRFC9190MitigationNegotiatesTLS12OrAbove`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L432). **positive:** `unit/verify` [`TestRFC9190MitigationOffersExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L552). **positive:** `unit/verify` [`TestRFC9190MitigationOffersOnlyNullCompression`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L744). **positive:** `unit/verify` [`TestRFC9190MitigationOffersSecureRenegotiationInfo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L574). **positive:** `unit/verify` [`TestRFC9190MitigationUsesANamedGroup`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L797). **negative:** `unit/verify` [`TestRFC9190MitigationDropsBytesPipelinedBehindTheStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L953). **negative:** `unit/verify` [`TestRFC9190MitigationOffersNoFiniteFieldDHGroup`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L831). **negative:** `unit/verify` [`TestRFC9190MitigationOffersNoPostHandshakeAuthentication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L595). **negative:** `unit/verify` [`TestRFC9190MitigationOffersNoRC4Suite`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L715). **negative:** `unit/verify` [`TestRFC9190MitigationOffersNoStaticRSAKeyExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L658). **negative:** `unit/verify` [`TestRFC9190MitigationRefusesACompressingClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L768). **negative:** `unit/verify` [`TestRFC9190MitigationRefusesAMethodDowngrade`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L899). **negative:** `unit/verify` [`TestRFC9190MitigationRefusesTLS11`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L475) |
| `RFC9190-2.1-1` | Implementations SHOULD NOT send the KeyUpdate message (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.1-3` | The EAP-TLS server SHOULD require the EAP-TLS peer to authenticate with a certificate (§2.1.1) | SHOULD | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-3` | The EAP-TLS peer SHOULD supply a "key_share" extension when attempting resumption (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-4` | EAP-TLS peers and EAP-TLS servers SHOULD follow the client tracking preventions in Appendix C.4 of RFC 8446 (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-5` | It is RECOMMENDED that the EAP-TLS peer use resumption if it has a valid ticket that has not been used before (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-6` | It is RECOMMENDED that the EAP-TLS server accept resumption if the ticket that was issued is still valid (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-7` | It is RECOMMENDED to use Network Access Identifiers with the same realm during resumption and the original full handshake (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.3-8` | When NAI reuse can be done without privacy implications, it is RECOMMENDED to use the same NAI in the resumption as was used in the original full handshake (§2.1.3) | RECOMMENDED | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.7-1` | When the client certificate contains an NAI as subject name or alternative subject name, an anonymous NAI SHOULD be derived from the NAI in the certificate (§2.1.7) | SHOULD | 2.1.7 | **positive:** `unit/verify` [`TestEAPTLS13PeerDerivesItsRealmFromTheCertificateNAI`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L169). **positive:** `unit/verify` [`TestEAPTLSPeerConfigCarriesTheCertificateTheNAIIsDerivedFrom`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_nai_wiring_test.go#L63). **positive:** `unit/verify` [`TestEAPTLSPeerDerivesTheRealmFromEveryNAIBearingCertificateField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L214). **negative:** `unit/verify` [`TestEAPTLSPeerKeepsTheAnonymousFallbackWhenNoCertificateNAIIsUsable`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L284). **negative:** `unit/verify` [`TestEAPTLSPeerPrefersTheConfiguredRealmOverTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L350) |
| `RFC9190-2.1.7-2` | It is RECOMMENDED to use anonymous NAIs in the Identity Response (§2.1.7) | RECOMMENDED | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.7-3` | Opaque blob identities are NOT RECOMMENDED because they are not routable (§2.1.7) | NOT RECOMMENDED | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9190-2.1.8-5` | It is RECOMMENDED to omit the username, so that the NAI is @realm (§2.1.8) | RECOMMENDED | 2.1.8 | **positive:** `unit/verify` [`TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L100). **negative:** no negative test |
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
| [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4) If the "early_data" extension is received, then it MUST be ignored (§2.1.2) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-1`](#rfc9190-2.1.4-1) If the EAP-TLS peer authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-2`](#rfc9190-2.1.4-2) If the EAP-TLS server authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS used (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.1.4-3`](#rfc9190-2.1.4-3) Whenever an implementation encounters a fatal error condition, it MUST send an appropriate TLS Error alert (§2.1.4) | no test | no test carries this requirement id |
| [`RFC9190-2.2-1`](#rfc9190-2.2-1) Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2) | no test | no test carries this requirement id |
| [`RFC9190-2.3-2`](#rfc9190-2.3-2) The key derivation MUST use the length values given in the section, 128 octets for Key_Material and 64 octets for Method-Id (§2.3) | no test | no test carries this requirement id |
| [`RFC9190-2.3-3`](#rfc9190-2.3-3) An implementation that intends to use only a part of the TLS-Exporter output MUST ask for the full output and then only use the desired part (§2.3) | no test | no test carries this requirement id |
| [`RFC9190-2.4-1`](#rfc9190-2.4-1) EAP-TLS peers and EAP-TLS servers MUST comply with the compliance requirements defined in Section 9 of RFC 8446 (§2.4) | no test | no test carries this requirement id |
| [`RFC9190-2.4-2`](#rfc9190-2.4-2) In EAP-TLS with TLS 1.3, only cipher suites with confidentiality SHALL be supported (§2.4) | no test | no test carries this requirement id |
| [`RFC9190-5.6-1`](#rfc9190-5.6-1) When peer authentication is not used, EAP-TLS server implementations MUST take care to limit network access appropriately for unauthenticated peers (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-2`](#rfc9190-5.6-2) Implementations MUST use resumption with caution to ensure that a resumed session is not granted more privilege than was intended for the original session (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-3`](#rfc9190-5.6-3) Authorization and accounting MUST be based on authenticated information such as information in the certificate, or the PSK identity and cached data provisioned for resumption (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.6-4`](#rfc9190-5.6-4) The requirements for Network Access Identifiers specified in Section 4 of RFC 7542 still apply and MUST be followed (§5.6) | no test | no test carries this requirement id |
| [`RFC9190-5.7-4`](#rfc9190-5.7-4) If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7) | no test | no test carries this requirement id |
| [`RFC9190-5.8-1`](#rfc9190-5.8-1) When anonymous NAIs are not used, privacy-friendly identities MUST be generated in a cryptographically secure way, so that an attacker cannot differentiate two identities belonging to the same user from two identities belonging to different users in the same realm (§5.8) | no test | no test carries this requirement id |
| [`RFC9190-5.8-2`](#rfc9190-5.8-2) Privacy-friendly usernames MUST NOT include substrings that can be used to relate the identity to a specific user (§5.8) | no test | no test carries this requirement id |
| [`RFC9190-5.8-3`](#rfc9190-5.8-3) Privacy-friendly usernames MUST NOT be formed by a fixed mapping that stays the same across multiple different authentications (§5.8) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9190-1-1`](#rfc9190-1-1)

Implementations MUST limit the maximum TLS version they use to 1.3, unless later versions are explicitly enabled by the administrator (§1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSVersionCapLeavesTLS12Reachable`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_version_cap_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestEAPTLSCapsBothRolesAtTLS13`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_version_cap_test.go#L51) | unit/verify | revert, verified |

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L60) | unit/verify | revert, verified |
| negative | [`TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L327) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L105) | unit/verify | revert, verified |

### [`RFC9190-2.1.2-2`](#rfc9190-2.1.2-2)

EAP-TLS servers MUST respect the 604800 second maximum ticket lifetime when issuing tickets (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L110) | unit/verify | revert, verified |

### [`RFC9190-2.1.2-3`](#rfc9190-2.1.2-3)

The NewSessionTicket message MUST NOT include an "early_data" extension (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L117) | unit/verify | revert, verified |

### [`RFC9190-2.1.2-4`](#rfc9190-2.1.2-4)

If the "early_data" extension is received, then it MUST be ignored (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.1.2-4, so no unit is bound to it.

### [`RFC9190-2.1.3-1`](#rfc9190-2.1.3-1)

When EAP-TLS is used with TLS 1.3, EAP-TLS SHALL use a resumption mechanism compatible with version 1.3 of TLS (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L332) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L121) | unit/verify | revert, verified |

### [`RFC9190-2.1.3-2`](#rfc9190-2.1.3-2)

The "psk_dh_ke" key exchange mode MUST be used for resumption unless the deployment has a local requirement to allow configuration of other mechanisms (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L125) | unit/verify | revert, verified |

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13AuthenticatorAcceptsAnAnonymousNAI`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L259) | unit/verify | revert, verified |

### [`RFC9190-2.1.8-2`](#rfc9190-2.1.8-2)

A client supporting TLS 1.3 MUST NOT send its username or any other permanent identifier in cleartext in the Identity Response, or in any message used instead of the Identity Response (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L237) | unit/verify | revert, verified |
| positive | [`TestEAPTLSPeerDropsTheUsernameTheCertificateCarries`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L373) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L70) | unit/verify | revert, verified |

### [`RFC9190-2.1.8-3`](#rfc9190-2.1.8-3)

The NAI MUST be a UTF-8 string as defined by the grammar in Section 2.2 of RFC 7542 (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNAIGrammarMatchesRFC7542Section22`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L182) | unit/verify | revert, verified |
| positive | [`TestEAPTLSPeerAnonymizesEveryConfiguredIdentity`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L131) | unit/verify | revert, verified |

### [`RFC9190-2.1.8-4`](#rfc9190-2.1.8-4)

When EAP-TLS is used with TLS 1.3, the EAP-TLS peer and EAP-TLS server SHALL follow the certificate_list processing specified by version 1.3 of TLS (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L362) | unit/verify | revert, verified |

### [`RFC9190-2.1.9-1`](#rfc9190-2.1.9-1)

Implementations MUST NOT set the L bit in unfragmented messages (§2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSSetsNoLengthBitOnAnUnfragmentedMessage`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestEAPTLSKeepsTheLengthBitOnAFragmentedMessage`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L145) | unit/verify | revert, verified |

### [`RFC9190-2.1.9-2`](#rfc9190-2.1.9-2)

Implementations MUST accept unfragmented messages with and without the L bit set (§2.1.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_fragmentation_test.go#L202) | unit/verify | revert, verified |

### [`RFC9190-2.2-1`](#rfc9190-2.2-1)

Unauthenticated information MUST NOT be used for accounting purposes or to give authorization (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-2.2-1, so no unit is bound to it.

### [`RFC9190-2.3-1`](#rfc9190-2.3-1)

The Key_Material and Method-Id SHALL be derived from the exporter_secret using the TLS exporter interface (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9190MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L158) | unit/verify | unproven |

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
| negative | [`TestEAPTLS12SendsNoProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L289) | unit/verify | unproven |
| negative | [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L254) | unit/verify | unproven |
| negative | [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L266) | interop/nightly | unproven |
| positive | [`TestEAPTLS13ResumedExchangeStillSendsTheSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L369) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L156) | unit/verify | unproven |
| positive | [`checkResponderEAPTLS13`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L260) | interop/nightly | unproven |

### [`RFC9190-2.5-2`](#rfc9190-2.5-2)

The EAP-TLS server MUST NOT send an encrypted TLS record with application data 0x00 before it has successfully processed the client Finished and sent its last handshake message (§2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13RefusedClientGetsNoSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L248) | unit/verify | unproven |
| positive | [`TestEAPTLS13SendsProtectedSuccessIndication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_test.go#L164) | unit/verify | unproven |

### [`RFC9190-5.4-1`](#rfc9190-5.4-1)

When EAP-TLS is used with TLS 1.3, the revocation status of all the certificates in the certificate chains MUST be checked, except the trust anchor (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13ResumptionStillNeedsARevocationSource`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L209) | unit/verify | revert, verified |
| negative | [`TestEAPTLS12CompletesWithNoRevocationList`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L323) | unit/verify | revert, verified |
| negative | [`TestEAPTLS13CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L297) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13ExceptsTheTrustAnchorFromRevocation`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L264) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesARevokedIntermediate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L162) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesARevokedServerCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesAStaleRevocationList`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesAnUncheckableChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L206) | unit/verify | revert, verified |
| positive | [`checkResponderEAPTLS13RevokedClient`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L303) | interop/nightly | unproven |

### [`RFC9190-5.4-2`](#rfc9190-5.4-2)

EAP-TLS servers supporting TLS 1.3 MUST implement Certificate Status Requests, that is OCSP stapling (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13StaplesTheConfiguredOCSPResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L163) | unit/verify | revert, verified |

### [`RFC9190-5.4-3`](#rfc9190-5.4-3)

An EAP-TLS peer using Certificate Status Requests MUST treat a CertificateEntry without a valid CertificateStatus extension as invalid, except the trust anchor, and abort the handshake with an appropriate alert (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCertificateStatusAcceptsADelegatedResponder`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L562) | unit/verify | revert, verified |
| negative | [`TestEAPTLS13PeerCompletesWithAValidStapledStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L252) | unit/verify | revert, verified |
| negative | [`TestEAPTLS13PeerWithoutTheStatusLeafAcceptsAnAuthenticatorThatStaplesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L312) | unit/verify | revert, verified |
| negative | [`TestStapledChainStatusExceptsTheTrustAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L581) | unit/verify | revert, verified |
| positive | [`TestCertificateStatusRefusesAResponseAboutAnotherCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L502) | unit/verify | revert, verified |
| positive | [`TestCertificateStatusRefusesAResponseDatedAhead`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L481) | unit/verify | revert, verified |
| positive | [`TestCertificateStatusRefusesAnExpiredResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L460) | unit/verify | revert, verified |
| positive | [`TestCertificateStatusRefusesAnUndelegatedResponder`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L540) | unit/verify | revert, verified |
| positive | [`TestCertificateStatusRefusesAnUnknownStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L520) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerRefusesARevokedStapledStatus`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L282) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerRefusesAnAuthenticatorThatStaplesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerRefusesAnIntermediateItCannotReadTheStatusOf`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L334) | unit/verify | revert, verified |
| positive | [`TestStapledChainStatusRefusesAnEmptyChainSet`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L596) | unit/verify | revert, verified |

### [`RFC9190-5.4-4`](#rfc9190-5.4-4)

EAP-TLS peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L278) | unit/verify | revert, verified |
| positive | [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L248) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L377) | unit/verify | revert, verified |

### [`RFC9190-5.4-5`](#rfc9190-5.4-5)

An EAP peer MUST use a secure transport to verify the revocation status of the server certificate (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPostAuthenticationCheckReadsAnHTTPSResponder`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L348) | unit/verify | revert, verified |
| positive | [`TestPostAuthenticationCheckRefusesAnInsecureResponderURL`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L323) | unit/verify | revert, verified |

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13ResumptionRefusesARevokedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L227) | unit/verify | revert, verified |

### [`RFC9190-5.7-2`](#rfc9190-5.7-2)

Any security policies for authorization MUST be followed also for resumption (§5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13RefusesAResumptionAgainstAReplacedTrustAnchor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L176) | unit/verify | revert, verified |
| negative | [`TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerRefusesAResumptionItCannotRebuildAChainFor`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L246) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L183) | unit/verify | revert, verified |

### [`RFC9190-5.7-3`](#rfc9190-5.7-3)

The EAP-TLS server or EAP client MUST cache data during the initial full handshake sufficient to allow authorization decisions to be made during resumption (§5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13ResumptionRefusesARevokedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L232) | unit/verify | revert, verified |

### [`RFC9190-5.7-4`](#rfc9190-5.7-4)

If cached data cannot be retrieved securely, resumption MUST NOT be done (§5.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9190-5.7-4, so no unit is bound to it.

### [`RFC9190-5.7-5`](#rfc9190-5.7-5)

EAP-TLS peers MUST NOT store resumption PSKs or tickets, and associated cached data, for longer than 604800 seconds regardless of the PSK or ticket lifetime (§5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13RefusesATicketPastTheSection57Lifetime`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L119) | unit/verify | revert, verified |
| negative | [`TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L285) | unit/verify | revert, verified |

### [`RFC9190-5.7-6`](#rfc9190-5.7-6)

If any authorization, accounting, or policy decision was made with information that has changed between the initial full handshake and resumption, and the change may lead to a different decision, that decision MUST be reevaluated (§5.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L265) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_refusal_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_resumption_test.go#L188) | unit/verify | revert, verified |

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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9190MitigationDropsBytesPipelinedBehindTheStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L953) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationOffersNoFiniteFieldDHGroup`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L831) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationOffersNoPostHandshakeAuthentication`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L595) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationOffersNoRC4Suite`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L715) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationOffersNoStaticRSAKeyExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L658) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationRefusesACompressingClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L768) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationRefusesAMethodDowngrade`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L899) | unit/verify | revert, verified |
| negative | [`TestRFC9190MitigationRefusesTLS11`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L475) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationCarriesNoServerName`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L861) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationNegotiatesAnAEADSuite`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L680) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationNegotiatesEphemeralKeyExchange`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L616) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationNegotiatesTLS12OrAbove`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L432) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationOffersExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L552) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationOffersOnlyNullCompression`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L744) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationOffersSecureRenegotiationInfo`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L574) | unit/verify | revert, verified |
| positive | [`TestRFC9190MitigationUsesANamedGroup`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_attack_mitigation_test.go#L797) | unit/verify | revert, verified |

### [`RFC9190-2.1.7-1`](#rfc9190-2.1.7-1)

When the client certificate contains an NAI as subject name or alternative subject name, an anonymous NAI SHOULD be derived from the NAI in the certificate (§2.1.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSPeerKeepsTheAnonymousFallbackWhenNoCertificateNAIIsUsable`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L284) | unit/verify | revert, verified |
| negative | [`TestEAPTLSPeerPrefersTheConfiguredRealmOverTheCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L350) | unit/verify | revert, verified |
| positive | [`TestEAPTLSPeerConfigCarriesTheCertificateTheNAIIsDerivedFrom`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_nai_wiring_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestEAPTLS13PeerDerivesItsRealmFromTheCertificateNAI`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestEAPTLSPeerDerivesTheRealmFromEveryNAIBearingCertificateField`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_cert_nai_test.go#L214) | unit/verify | revert, verified |

### [`RFC9190-2.1.8-5`](#rfc9190-2.1.8-5)

It is RECOMMENDED to omit the username, so that the NAI is @realm (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_nai_test.go#L100) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement phase agent, plan/spec-ipsec-rfc9190.md step 6, RFC 9190 walk |
| Signed off | 2026-09-08 |
| Register | prose |
| Source | rfc/full/rfc9190.txt |
| Source fingerprint | be7b2c2e7aeee4a9 |
| Record | rfc/extraction/rfc9190.json |
| Mapped sentences | 48 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Stream and RFC number, Abstract, Status of This Memo and Copyright Notice. Its one site is the IETF Trust Revised BSD License sentence, excluded below. |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 5 | walked | not stated |
| `2.1.1` | not stated | 2 | walked | not stated |
| `2.1.2` | not stated | 4 | walked | not stated |
| `2.1.3` | not stated | 3 | walked | not stated |
| `2.1.4` | not stated | 3 | walked | not stated |
| `2.1.5` | not stated | 0 | walked | not stated |
| `2.1.6` | not stated | 0 | walked | not stated |
| `2.1.7` | not stated | 0 | walked | not stated |
| `2.1.8` | not stated | 4 | walked | not stated |
| `2.1.9` | not stated | 1 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 3 | walked | not stated |
| `2.4` | not stated | 2 | walked | not stated |
| `2.5` | not stated | 3 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has added EXPORTER_EAP_TLS_Key_Material and EXPORTER_EAP_TLS_Method-Id to the TLS Exporter Labels registry of RFC 5705. The registry action binds IANA; the obligation to USE those labels is Section 2.3's and is mapped at site 2.3:1. |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 5 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `5.6` | not stated | 3 | walked | not stated |
| `5.7` | not stated | 7 | walked | not stated |
| `5.8` | not stated | 3 | walked | not stated |
| `5.9` | not stated | 0 | walked | not stated |
| `5.10` | not stated | 1 | walked | not stated |
| `5.11` | not stated | 0 | walked | not stated |
| `6` | References | 0 | skipped (references) | References. A heading over the two lists below it. |
| `6.1` | Normative References | 0 | skipped (references) | Normative References. Citation entries only. |
| `6.2` | Informative references | 0 | skipped (references) | Informative references. Citation entries only. |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate the splitter did not strip. The sentence binds a person who extracts Code Components from the document into other software and tells them to carry the Revised BSD License text. It directs no EAP-TLS peer and no EAP-TLS server and describes no protocol behavior. Its 'must' is lowercase, so it is raised only by the case-insensitive modal scan the 'prose' register uses. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `2.1.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative. It describes what FOLLOWS when a server exercises the MAY in the preceding sentence ('However, the EAP-TLS server MAY choose to require a full handshake'), which is declared as RFC9190-2.1.3-9: the negotiation proceeds as a new authentication and the resumption attempt is ignored. It directs no behavior of its own and states no obligation level. Its 'is required' is what the case-insensitive modal scan of the 'prose' register raised, not a keyword. | In the case a full handshake is required, the negotiation proceeds as if the session was a new authentication, and the resumption attempt is ignored. |
| `2.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Step 3 of the very procedure site 2.5:1 makes mandatory: after the EAP-Request carrying the indication, no more EAP-Requests and only an EAP-Success. Its 'must not' and 'may' are lowercase, so they set no level of their own and the level is the MUST at the head of the procedure. It restates an obligation RFC9190-2.5-1 already carries, which site 2.5:1 maps. | After sending an EAP-Request that contains the protected success result indication, the EAP-TLS server must not send any more EAP-Requests and may only send an EAP-Success. |
| `5.7:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative rationale, not an obligation. It explains WHY the preceding requirements also cover a server that expects accounting: accounting must be tied to an authenticated identity, resumption supplies none, so accounting is impossible without the cached data. The obligation the paragraph reaches is the SHOULD in the sentence AFTER it, declared as RFC9190-5.7-7 and unsourced on this section. Its 'must' is lowercase and was raised only by the case-insensitive modal scan. | Since accounting must be tied to an authenticated identity, and resumption does not supply such an identity, accounting is impossible without access to cached data. |

## Superseded

No document obsoletes RFC 9190, so its obligations are stated where they were written.
