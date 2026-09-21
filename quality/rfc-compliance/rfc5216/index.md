# RFC 5216 - The EAP-TLS Authentication Protocol

Partial. Every requirement this repository extracted from RFC 5216, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 27.7% | 13 of 47 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.8% | 6 of 47 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 47 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 20.0% | 8 of 40 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 47 | of 52 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 47 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 47 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 47 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 47 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 59.6% | 28 of 47 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 47 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 52 |
| Gated MUST-level | 47 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 27 |
| Nightly-only evidence | 0 |
| Test tags | 40 |
| Tagged units | 40 |
| Recorded audit verdicts | 0 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc5216.md` |
| Requirement shard | `rfc/requirements/rfc5216.md` |
| RFC text | `rfc/full/rfc5216.txt` |

## Enrolment

Enrolled: EAP-TLS (RFC 5216, EAP inside IKEv2): 52 rows, 47 of them MUST-level. 11 are MET in both polarities (fragmentation, L-bit, mutual-auth handshake, and all seven Section 2.1.3 termination MUSTs: EAP-Failure answers a peer TLS alert, the authenticator waits for the peer reply after its own alert, that reply is answered with EAP-Failure, the authenticator sends its change_cipher_spec and Finished closing flight before it concludes, the peer replies to the authenticator before it terminates, the peer answers the closing flight with a no-data EAP-Response, and the authenticator answers that with EAP-Success), and 6 carry a single-polarity positive proof (reserved flags, TLS>=1.0 both sides, no compression, no cipher-leak, MSK label). RFC5216-2.4-5, the mandatory TLS_RSA_WITH_3DES_EDE_CBC_SHA ciphersuite, is a gap. The 2026-09-21 extraction walk read the document against this checklist and added 27 MUST-level rows it did not carry: the Section 2.1.1 base-case flight and the Section 2.1.2 resumption flight, the six Section 2.1.5 fragment-ACK and Identifier obligations, TLS v1.0 support (2.4-7), the four Section 3 packet-field obligations, the five Section 5.2 certificate naming obligations, and the two Section 5.3 authorization obligations. None of the 27 carries a tagged test, so each is an open gap. The same walk corrected RFC5216-5.3-1 from MUST to SHOULD, which is the level Section 5.3 states for RFC 3280 path validation, and moved the section reference of RFC5216-2.4-1 and RFC5216-2.4-2 to Section 2.1.1, where the two TLS version sentences are written. Handshake harness enabled by the eap deadlock+cert-validation fix (commit 0816c2b74)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Full EAP-TLS handshake as both authenticator (RequireAndVerifyClientCert + ClientCAs) and peer (RootCAs), L/M/S fragmentation with a 64KB reassembly bound and empty-message fragment ACK, and MSK export via the label "client EAP encryption" feeding the IKEv2 AUTH payload
- certs from the PKI store. Section 2.1.3 termination in both directions: a rejected peer receives the fatal TLS alert in an EAP-Request, the authenticator waits for its EAP-Response and only then sends EAP-Failure, and a peer TLS alert that rejects the authenticator is answered with EAP-Failure in the round that carries it. A peer that rejects the authenticator replies first and reports the cause on the round after. Section 2.1.3 termination on the success path: the authenticator sends its change_cipher_spec and Finished closing flight, the peer answers with a no-data EAP-Response, and the authenticator answers that with EAP-Success. One reachability limit is not a conformance gap and is stated here so no reader assumes otherwise: the Section 2.3 derivation is a crypto/tls ExportKeyingMaterial call, and Go refuses that export on a TLS 1.2 session that did not negotiate the RFC 7627 extended master secret, so a peer such as strongSwan 5.9.14 cannot be authenticated over TLS 1.2 at all. That peer lands on TLS 1.2 by default rather than by limitation, and `charon.tls.version_max = 1.3` moves the same build onto the RFC 9190 path that scenario eap-tls13 proves. Ze implements the derivation correctly and reports the refusal with the peer, the negotiated version, RFC 7627 and the operator's answers (`eapTLS12ExportRefused`, [`internal/core/eap/eap_tls.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls.go))
- Go 1.27 removed the GODEBUG setting that once lifted the refusal and offers no replacement, and RFC 9190 covers the TLS 1.3 path, which needs none of it.


**What the ledger says remains**

28 MUST-level gaps in [`rfc/short/rfc5216.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5216.md). [`RFC5216-2.4-5`](#rfc5216-2.4-5), the mandatory TLS_RSA_WITH_3DES_EDE_CBC_SHA ciphersuite, is not offered (TLS 1.2+ Go defaults exclude insecure 3DES). The other 27 are the rows the 2026-09-21 extraction walk added from the document's own text and that no tagged test yet proves: [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3) through -10, [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1), [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2) through -7, [`RFC5216-2.4-7`](#rfc5216-2.4-7), [`RFC5216-3-3`](#rfc5216-3-3) through -6, [`RFC5216-5.2-1`](#rfc5216-5.2-1) through -5, and [`RFC5216-5.3-2`](#rfc5216-5.3-2) and -3. Ze runs the handshake through crypto/tls, so several of them are expected to hold structurally, but none has been demonstrated. [`RFC5216-5.4-2`](#rfc5216-5.4-2) is no longer among them: the peer re-checks the revocation status of the authenticator certificate once the CHILD_SA gives it a network, over https, and closes the SA when the responder reports it revoked (startServerCertRecheck, [`internal/component/ike/engine/postauth_revocation.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/postauth_revocation.go), reached from runEstablished). It runs whichever TLS version the exchange negotiated, which is what this RFC asks for, and the chain it re-reads is the one the peer accepted (eap.PeerSession.ServerChains). [`RFC5216-5.4-1`](#rfc5216-5.4-1) is no longer among them either: checkChainRevocation ([`internal/core/eap/revocation.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/revocation.go)) checks every certificate on each verified chain except the trust anchor against the crl leaf-list its CA publishes, on the authenticator through tls.Config.VerifyConnection and on the peer through serverChainCheck.verifyConnection, and it is proven in both polarities on TLS 1.2, which is the version this RFC governs.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 27 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **47** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC5216-3-1`](#rfc5216-3-1), [`RFC5216-2.1.1-1`](#rfc5216-2.1.1-1), [`RFC5216-2.1.1-2`](#rfc5216-2.1.1-2), [`RFC5216-2.1.5-1`](#rfc5216-2.1.5-1), [`RFC5216-5.4-1`](#rfc5216-5.4-1), [`RFC5216-5.4-2`](#rfc5216-5.4-2), [`RFC5216-2.1.3-1`](#rfc5216-2.1.3-1), [`RFC5216-2.1.3-3`](#rfc5216-2.1.3-3), [`RFC5216-2.1.3-4`](#rfc5216-2.1.3-4), [`RFC5216-2.1.3-5`](#rfc5216-2.1.3-5), [`RFC5216-2.1.3-6`](#rfc5216-2.1.3-6), [`RFC5216-2.1.3-7`](#rfc5216-2.1.3-7), [`RFC5216-2.1.3-8`](#rfc5216-2.1.3-8)

**Annotated instead of tested (7):** [`RFC5216-3-2`](#rfc5216-3-2), [`RFC5216-2.4-1`](#rfc5216-2.4-1), [`RFC5216-2.4-2`](#rfc5216-2.4-2), [`RFC5216-2.4-3`](#rfc5216-2.4-3), [`RFC5216-2.4-4`](#rfc5216-2.4-4), [`RFC5216-2.4-5`](#rfc5216-2.4-5), [`RFC5216-2.3-1`](#rfc5216-2.3-1)

**No test and no annotation (27):** [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3), [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4), [`RFC5216-2.1.1-5`](#rfc5216-2.1.1-5), [`RFC5216-2.1.1-6`](#rfc5216-2.1.1-6), [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7), [`RFC5216-2.1.1-8`](#rfc5216-2.1.1-8), [`RFC5216-2.1.1-9`](#rfc5216-2.1.1-9), [`RFC5216-2.1.1-10`](#rfc5216-2.1.1-10), [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1), [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2), [`RFC5216-2.1.5-3`](#rfc5216-2.1.5-3), [`RFC5216-2.1.5-4`](#rfc5216-2.1.5-4), [`RFC5216-2.1.5-5`](#rfc5216-2.1.5-5), [`RFC5216-2.1.5-6`](#rfc5216-2.1.5-6), [`RFC5216-2.1.5-7`](#rfc5216-2.1.5-7), [`RFC5216-2.4-7`](#rfc5216-2.4-7), [`RFC5216-3-3`](#rfc5216-3-3), [`RFC5216-3-4`](#rfc5216-3-4), [`RFC5216-3-5`](#rfc5216-3-5), [`RFC5216-3-6`](#rfc5216-3-6), [`RFC5216-5.2-1`](#rfc5216-5.2-1), [`RFC5216-5.2-2`](#rfc5216-5.2-2), [`RFC5216-5.2-3`](#rfc5216-5.2-3), [`RFC5216-5.2-4`](#rfc5216-5.2-4), [`RFC5216-5.2-5`](#rfc5216-5.2-5), [`RFC5216-5.3-2`](#rfc5216-5.3-2), [`RFC5216-5.3-3`](#rfc5216-5.3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5216-3-1` | L bit MUST be set on the first fragment of a TLS message (§3, Flags) | MUST | 3 | **positive:** `unit/verify` [`TestTLSFragmenterFirstFragmentHasLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L395). **negative:** `unit/verify` [`TestTLSFragmenterMiddleAndLastFragmentsHaveNoLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L475) |
| `RFC5216-3-2` | Reserved flag bits (3-7) must be zero on send (§3, Flags) | MUST | 3 | **positive:** `unit/verify` [`TestTLSFragmentReservedFlagBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L494). **negative:** no negative test. **{single-polarity}:** the flags byte is assembled only from L/M in nextFragment (and S alone in Start), so no code path can set reserved bits 3-7 and only the positive assertion is reachable (internal/core/eap/eap_tls.go:106, :190) |
| `RFC5216-2.4-1` | Peer MUST offer TLS 1.0 or later in ClientHello: "The version offered by the peer MUST correspond to TLS v1.0 or later" (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L364). **negative:** no negative test. **{single-polarity}:** the peer's tls.Config forces MinVersion TLS 1.2, so its ClientHello always offers at least TLS 1.0 and no path can offer lower (internal/core/eap/peer.go:334) |
| `RFC5216-2.4-2` | Server MUST respond with TLS 1.0 or later in ServerHello: "The version offered by the server MUST correspond to TLS v1.0 or later" (§2.4) | MUST | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L367). **negative:** no negative test. **{single-polarity}:** the server's tls.Config forces MinVersion TLS 1.2, so any ServerHello it emits is at least TLS 1.0 and it never negotiates lower (internal/core/eap/eap_tls.go:167) |
| `RFC5216-2.4-3` | TLS compression MUST NOT be requested or negotiated (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L370). **negative:** no negative test. **{single-polarity}:** both tls.Config values delegate to Go crypto/tls, which implements no TLS compression and always sends null compression, so the prohibition holds structurally with no knob to falsify (internal/core/eap/eap_tls.go:163, peer.go:330) |
| `RFC5216-2.4-4` | TLS ciphersuite negotiation MUST NOT be used to negotiate lower-layer data ciphersuites (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L375). **negative:** no negative test. **{single-polarity}:** the only value ze extracts from the TLS session is the 64-octet MSK fed to the IKEv2 AUTH payload; the ESP/data-plane ciphers come from independent IKE SA proposal negotiation, never from the TLS ciphersuite (internal/core/eap/eap_tls.go:268) |
| `RFC5216-2.4-5` | Mandatory ciphersuite TLS_RSA_WITH_3DES_EDE_CBC_SHA MUST be supported (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the server sets no CipherSuites and requires TLS 1.2+, so it inherits Go's secure defaults which classify 3DES as insecure and do not enable TLS_RSA_WITH_3DES_EDE_CBC_SHA, leaving the RFC's mandatory-to-implement ciphersuite unavailable (internal/core/eap/eap_tls.go:163, peer.go:330) |
| `RFC5216-2.1.1-1` | Server sends CertificateRequest; peer MUST provide a certificate for mutual authentication (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L349). **negative:** `unit/verify` [`TestEAPTLSAuthenticatorRequiresClientCert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L444) |
| `RFC5216-2.1.1-2` | "The EAP-TLS conversation will then begin, with the peer sending an EAP-Response packet with EAP-Type=EAP-TLS. The data field of that packet will encapsulate one or more TLS records in TLS record layer format, containing a TLS client_hello handshake message" -- stated in the indicative register, so the obligation on the peer's answer to the Start carries no RFC 2119 keyword (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSPeerFirstResponseCarriesClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L89). **negative:** `unit/verify` [`TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L231) |
| `RFC5216-2.1.5-1` | Fragmentation MUST be implemented; fragment ACK is an empty EAP-TLS message (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestTLSFragmenterRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L348). **negative:** `unit/verify` [`TestTLSReassemblyRejectsOversized`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L444) |
| `RFC5216-5.3-1` | Both sides SHOULD perform RFC 3280 compliant path validation: the server "SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation" and "The EAP-TLS peer SHOULD support validating the server certificate using RFC 3280 [RFC3280] compliant path validation" (§5.3) | SHOULD | 5.3 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L355). **negative:** `unit/verify` [`TestEAPTLSPeerRejectsUntrustedServerChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L497). **negative:** `unit/verify` [`TestEAPTLSPeerWithoutCARefusesToStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L532). **negative:** `unit/verify` [`TestEAPTLSServerRejectsUntrustedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L468) |
| `RFC5216-5.4-1` | CRL checking MUST be supported (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS12RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L353). **negative:** `unit/verify` [`TestEAPTLS12CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L384) |
| `RFC5216-5.4-2` | Post-authentication revocation checking MUST be supported (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L396). **positive:** `unit/verify` [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L253). **negative:** `unit/verify` [`TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L416). **negative:** `unit/verify` [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L283) |
| `RFC5216-2.1.3-1` | "The EAP Server MUST reply with an EAP-Failure packet since server authentication failure is a terminal condition" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216ServerRepliesEAPFailureToPeerAlert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L119). **negative:** `unit/verify` [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L202) |
| `RFC5216-2.1.3-3` | "To ensure that the peer receives the TLS alert message, the EAP server MUST wait for the peer to reply with an EAP-Response packet" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L95). **negative:** `unit/verify` [`TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L361) |
| `RFC5216-2.1.3-4` | On a reply with EAP-Type=EAP-TLS and no data, "the EAP-Server MUST send an EAP-Failure packet and terminate the conversation" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L101). **positive:** `unit/verify` [`TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L208). **negative:** `unit/verify` [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L207) |
| `RFC5216-2.1.3-5` | "If the peer authenticates successfully, the EAP server MUST respond with an EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new TLS session, one or more TLS records containing TLS change_cipher_spec and finished handshake messages" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L229). **negative:** `unit/verify` [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L333) |
| `RFC5216-2.1.3-6` | "To ensure that the EAP Server receives the TLS alert message, the peer MUST wait for the EAP Server to reply before terminating the conversation" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216PeerRepliesBeforeItTerminates`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L41). **negative:** `unit/verify` [`TestRFC5216PeerDoesNotWaitWhenItSentNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L168) |
| `RFC5216-2.1.3-7` | "If the EAP server authenticates successfully, the peer MUST send an EAP-Response packet of EAP-Type=EAP-TLS, and no data" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L237). **negative:** `unit/verify` [`TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L386) |
| `RFC5216-2.1.3-8` | After that no-data EAP-Response, "The EAP Server then MUST respond with an EAP-Success message" (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L242). **negative:** `unit/verify` [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L339) |
| `RFC5216-2.3-1` | MSK and EMSK are derived from TLS master_secret using the label "client EAP encryption" (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L360). **positive:** `unit/verify` [`TestRFC5216MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L112). **negative:** no negative test. **{single-polarity}:** both server and peer export the 64-octet MSK via ExportKeyingMaterial with the exact RFC label 'client EAP encryption', the only key the IKEv2 lower layer consumes (internal/core/eap/eap_tls.go:268, peer.go:381) |
| `RFC5216-2.1.1-3` | "Once having received the peer's Identity, the EAP server MUST respond with an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS, the Start (S) bit set, and no data" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-4` | "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-5` | "If the session matches the peer's, then the ciphersuite MUST match the one negotiated during the handshake protocol execution that established the session" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-6` | "If the EAP server is not resuming a previously established session, then it MUST include a TLS server_certificate handshake message, and a server_hello_done handshake message MUST be the last handshake message encapsulated in this EAP-Request packet" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-7` | Where the certificate carries a signature public key, "a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-8` | "If the peer supports EAP-TLS and is configured to use it, it MUST respond to the EAP-Request with an EAP-Response packet of EAP-Type=EAP-TLS" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-9` | Where the preceding server_hello did not indicate resumption, "the data field of this packet MUST encapsulate one or more TLS records containing a TLS client_key_exchange, change_cipher_spec, and finished messages" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.1-10` | Where the preceding server_hello indicated resumption, "the peer MUST send only the change_cipher_spec and finished handshake messages" (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.2-1` | "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message" (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-2` | "When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-3` | "The EAP server MUST wait until it receives the EAP-Response before sending another fragment" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-4` | "the EAP server MUST increment the Identifier field for each fragment contained within an EAP-Request, and the peer MUST include this Identifier value in the fragment ACK contained within the EAP-Response" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-5` | "when the EAP server receives an EAP-Response with the M bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS and no data" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-6` | "The EAP peer MUST wait until it receives the EAP-Request before sending another fragment" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.5-7` | "the EAP server MUST increment the Identifier value for each fragment ACK contained within an EAP-Request, and the peer MUST include this Identifier value in the subsequent fragment contained within an EAP-Response" (§2.1.5) | MUST | 2.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.4-7` | "EAP-TLS implementations MUST support TLS v1.0" (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-3-3` | "The Identifier field MUST be changed on each Request packet" (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-3-4` | "Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception" (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-3-5` | "Implementations of this specification MUST set the reserved bits to zero, and MUST ignore them on reception" -- the reception half of that sentence (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-3-6` | In an EAP-TLS Response, "The Identifier field is one octet and MUST match the Identifier field from the corresponding request" (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.2-1` | "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.2-2` | "If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.2-3` | "Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.2-4` | "The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used" (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.2-5` | "Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN)" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.3-2` | "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.3-3` | When comparing the Extended Key Usage of the presented certificate, "implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.4-3` | OCSP revocation checking SHOULD be supported (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.4-4` | TLS Certificate Status Request (stapling) SHOULD be supported (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.4-6` | TLS_RSA_WITH_AES_128_CBC_SHA SHOULD be supported (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.3-2` | Server MAY allow EAP-TLS restart after peer authentication failure, subject to per-peer limit (§2.1.3) | MAY | 2.1.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5216-2.4-5`](#rfc5216-2.4-5) Mandatory ciphersuite TLS_RSA_WITH_3DES_EDE_CBC_SHA MUST be supported (§2.4) | {gap}, no test | the server sets no CipherSuites and requires TLS 1.2+, so it inherits Go's secure defaults which classify 3DES as insecure and do not enable TLS_RSA_WITH_3DES_EDE_CBC_SHA, leaving the RFC's mandatory-to-implement ciphersuite unavailable (internal/core/eap/eap_tls.go:163, peer.go:330) |
| [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3) "Once having received the peer's Identity, the EAP server MUST respond with an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS, the Start (S) bit set, and no data" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4) "If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-5`](#rfc5216-2.1.1-5) "If the session matches the peer's, then the ciphersuite MUST match the one negotiated during the handshake protocol execution that established the session" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-6`](#rfc5216-2.1.1-6) "If the EAP server is not resuming a previously established session, then it MUST include a TLS server_certificate handshake message, and a server_hello_done handshake message MUST be the last handshake message encapsulated in this EAP-Request packet" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7) Where the certificate carries a signature public key, "a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-8`](#rfc5216-2.1.1-8) "If the peer supports EAP-TLS and is configured to use it, it MUST respond to the EAP-Request with an EAP-Response packet of EAP-Type=EAP-TLS" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-9`](#rfc5216-2.1.1-9) Where the preceding server_hello did not indicate resumption, "the data field of this packet MUST encapsulate one or more TLS records containing a TLS client_key_exchange, change_cipher_spec, and finished messages" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.1-10`](#rfc5216-2.1.1-10) Where the preceding server_hello indicated resumption, "the peer MUST send only the change_cipher_spec and finished handshake messages" (§2.1.1) | no test | no test carries this requirement id |
| [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1) "If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message" (§2.1.2) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2) "When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-3`](#rfc5216-2.1.5-3) "The EAP server MUST wait until it receives the EAP-Response before sending another fragment" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-4`](#rfc5216-2.1.5-4) "the EAP server MUST increment the Identifier field for each fragment contained within an EAP-Request, and the peer MUST include this Identifier value in the fragment ACK contained within the EAP-Response" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-5`](#rfc5216-2.1.5-5) "when the EAP server receives an EAP-Response with the M bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS and no data" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-6`](#rfc5216-2.1.5-6) "The EAP peer MUST wait until it receives the EAP-Request before sending another fragment" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.1.5-7`](#rfc5216-2.1.5-7) "the EAP server MUST increment the Identifier value for each fragment ACK contained within an EAP-Request, and the peer MUST include this Identifier value in the subsequent fragment contained within an EAP-Response" (§2.1.5) | no test | no test carries this requirement id |
| [`RFC5216-2.4-7`](#rfc5216-2.4-7) "EAP-TLS implementations MUST support TLS v1.0" (§2.4) | no test | no test carries this requirement id |
| [`RFC5216-3-3`](#rfc5216-3-3) "The Identifier field MUST be changed on each Request packet" (§3) | no test | no test carries this requirement id |
| [`RFC5216-3-4`](#rfc5216-3-4) "Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception" (§3) | no test | no test carries this requirement id |
| [`RFC5216-3-5`](#rfc5216-3-5) "Implementations of this specification MUST set the reserved bits to zero, and MUST ignore them on reception" -- the reception half of that sentence (§3) | no test | no test carries this requirement id |
| [`RFC5216-3-6`](#rfc5216-3-6) In an EAP-TLS Response, "The Identifier field is one octet and MUST match the Identifier field from the corresponding request" (§3) | no test | no test carries this requirement id |
| [`RFC5216-5.2-1`](#rfc5216-5.2-1) "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName" (§5.2) | no test | no test carries this requirement id |
| [`RFC5216-5.2-2`](#rfc5216-5.2-2) "If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical" (§5.2) | no test | no test carries this requirement id |
| [`RFC5216-5.2-3`](#rfc5216-5.2-3) "Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities" (§5.2) | no test | no test carries this requirement id |
| [`RFC5216-5.2-4`](#rfc5216-5.2-4) "The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used" (§5.2) | no test | no test carries this requirement id |
| [`RFC5216-5.2-5`](#rfc5216-5.2-5) "Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN)" (§5.2) | no test | no test carries this requirement id |
| [`RFC5216-5.3-2`](#rfc5216-5.3-2) "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS" (§5.3) | no test | no test carries this requirement id |
| [`RFC5216-5.3-3`](#rfc5216-5.3-3) When comparing the Extended Key Usage of the presented certificate, "implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]" (§5.3) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5216-3-1`](#rfc5216-3-1)

L bit MUST be set on the first fragment of a TLS message (§3, Flags)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTLSFragmenterMiddleAndLastFragmentsHaveNoLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L475) | unit/verify | unproven |
| positive | [`TestTLSFragmenterFirstFragmentHasLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L395) | unit/verify | unproven |

### [`RFC5216-3-2`](#rfc5216-3-2)

Reserved flag bits (3-7) must be zero on send (§3, Flags)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTLSFragmentReservedFlagBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L494) | unit/verify | unproven |

### [`RFC5216-2.4-1`](#rfc5216-2.4-1)

Peer MUST offer TLS 1.0 or later in ClientHello: "The version offered by the peer MUST correspond to TLS v1.0 or later" (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L364) | unit/verify | unproven |

### [`RFC5216-2.4-2`](#rfc5216-2.4-2)

Server MUST respond with TLS 1.0 or later in ServerHello: "The version offered by the server MUST correspond to TLS v1.0 or later" (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L367) | unit/verify | unproven |

### [`RFC5216-2.4-3`](#rfc5216-2.4-3)

TLS compression MUST NOT be requested or negotiated (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L370) | unit/verify | unproven |

### [`RFC5216-2.4-4`](#rfc5216-2.4-4)

TLS ciphersuite negotiation MUST NOT be used to negotiate lower-layer data ciphersuites (§2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L375) | unit/verify | unproven |

### [`RFC5216-2.4-5`](#rfc5216-2.4-5)

Mandatory ciphersuite TLS_RSA_WITH_3DES_EDE_CBC_SHA MUST be supported (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.4-5, so no unit is bound to it.

### [`RFC5216-2.1.1-1`](#rfc5216-2.1.1-1)

Server sends CertificateRequest; peer MUST provide a certificate for mutual authentication (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSAuthenticatorRequiresClientCert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L444) | unit/verify | unproven |
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L349) | unit/verify | unproven |

### [`RFC5216-2.1.1-2`](#rfc5216-2.1.1-2)

"The EAP-TLS conversation will then begin, with the peer sending an EAP-Response packet with EAP-Type=EAP-TLS. The data field of that packet will encapsulate one or more TLS records in TLS record layer format, containing a TLS client_hello handshake message" -- stated in the indicative register, so the obligation on the peer's answer to the Start carries no RFC 2119 keyword (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L231) | unit/verify | unproven |
| positive | [`TestEAPTLSPeerFirstResponseCarriesClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L89) | unit/verify | unproven |

### [`RFC5216-2.1.5-1`](#rfc5216-2.1.5-1)

Fragmentation MUST be implemented; fragment ACK is an empty EAP-TLS message (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTLSReassemblyRejectsOversized`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L444) | unit/verify | unproven |
| positive | [`TestTLSFragmenterRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L348) | unit/verify | unproven |

### [`RFC5216-5.3-1`](#rfc5216-5.3-1)

Both sides SHOULD perform RFC 3280 compliant path validation: the server "SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation" and "The EAP-TLS peer SHOULD support validating the server certificate using RFC 3280 [RFC3280] compliant path validation" (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSPeerRejectsUntrustedServerChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L497) | unit/verify | unproven |
| negative | [`TestEAPTLSPeerWithoutCARefusesToStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L532) | unit/verify | unproven |
| negative | [`TestEAPTLSServerRejectsUntrustedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L468) | unit/verify | unproven |
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L355) | unit/verify | unproven |

### [`RFC5216-5.4-1`](#rfc5216-5.4-1)

CRL checking MUST be supported (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS12CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L384) | unit/verify | revert, verified |
| positive | [`TestEAPTLS12RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L353) | unit/verify | revert, verified |

### [`RFC5216-5.4-2`](#rfc5216-5.4-2)

Post-authentication revocation checking MUST be supported (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L283) | unit/verify | revert, verified |
| negative | [`TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L416) | unit/verify | revert, verified |
| positive | [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L253) | unit/verify | revert, verified |
| positive | [`TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L396) | unit/verify | revert, verified |

### [`RFC5216-2.1.3-1`](#rfc5216-2.1.3-1)

"The EAP Server MUST reply with an EAP-Failure packet since server authentication failure is a terminal condition" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC5216ServerRepliesEAPFailureToPeerAlert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L119) | unit/verify | unproven |

### [`RFC5216-2.1.3-3`](#rfc5216-2.1.3-3)

"To ensure that the peer receives the TLS alert message, the EAP server MUST wait for the peer to reply with an EAP-Response packet" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L361) | unit/verify | unproven |
| positive | [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L95) | unit/verify | revert, verified |

### [`RFC5216-2.1.3-4`](#rfc5216-2.1.3-4)

On a reply with EAP-Type=EAP-TLS and no data, "the EAP-Server MUST send an EAP-Failure packet and terminate the conversation" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L207) | unit/verify | unproven |
| positive | [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L208) | unit/verify | unproven |

### [`RFC5216-2.1.3-5`](#rfc5216-2.1.3-5)

"If the peer authenticates successfully, the EAP server MUST respond with an EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new TLS session, one or more TLS records containing TLS change_cipher_spec and finished handshake messages" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L333) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L229) | unit/verify | unproven |

### [`RFC5216-2.1.3-6`](#rfc5216-2.1.3-6)

"To ensure that the EAP Server receives the TLS alert message, the peer MUST wait for the EAP Server to reply before terminating the conversation" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerDoesNotWaitWhenItSentNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L168) | unit/verify | unproven |
| positive | [`TestRFC5216PeerRepliesBeforeItTerminates`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L41) | unit/verify | unproven |

### [`RFC5216-2.1.3-7`](#rfc5216-2.1.3-7)

"If the EAP server authenticates successfully, the peer MUST send an EAP-Response packet of EAP-Type=EAP-TLS, and no data" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L386) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L237) | unit/verify | unproven |

### [`RFC5216-2.1.3-8`](#rfc5216-2.1.3-8)

After that no-data EAP-Response, "The EAP Server then MUST respond with an EAP-Success message" (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L339) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L242) | unit/verify | unproven |

### [`RFC5216-2.3-1`](#rfc5216-2.3-1)

MSK and EMSK are derived from TLS master_secret using the label "client EAP encryption" (§2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L360) | unit/verify | unproven |
| positive | [`TestRFC5216MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L112) | unit/verify | unproven |

### [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3)

"Once having received the peer's Identity, the EAP server MUST respond with an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS, the Start (S) bit set, and no data" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-3, so no unit is bound to it.

### [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4)

"If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-4, so no unit is bound to it.

### [`RFC5216-2.1.1-5`](#rfc5216-2.1.1-5)

"If the session matches the peer's, then the ciphersuite MUST match the one negotiated during the handshake protocol execution that established the session" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-5, so no unit is bound to it.

### [`RFC5216-2.1.1-6`](#rfc5216-2.1.1-6)

"If the EAP server is not resuming a previously established session, then it MUST include a TLS server_certificate handshake message, and a server_hello_done handshake message MUST be the last handshake message encapsulated in this EAP-Request packet" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-6, so no unit is bound to it.

### [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7)

Where the certificate carries a signature public key, "a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-7, so no unit is bound to it.

### [`RFC5216-2.1.1-8`](#rfc5216-2.1.1-8)

"If the peer supports EAP-TLS and is configured to use it, it MUST respond to the EAP-Request with an EAP-Response packet of EAP-Type=EAP-TLS" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-8, so no unit is bound to it.

### [`RFC5216-2.1.1-9`](#rfc5216-2.1.1-9)

Where the preceding server_hello did not indicate resumption, "the data field of this packet MUST encapsulate one or more TLS records containing a TLS client_key_exchange, change_cipher_spec, and finished messages" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-9, so no unit is bound to it.

### [`RFC5216-2.1.1-10`](#rfc5216-2.1.1-10)

Where the preceding server_hello indicated resumption, "the peer MUST send only the change_cipher_spec and finished handshake messages" (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-10, so no unit is bound to it.

### [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1)

"If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message" (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.2-1, so no unit is bound to it.

### [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2)

"When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-2, so no unit is bound to it.

### [`RFC5216-2.1.5-3`](#rfc5216-2.1.5-3)

"The EAP server MUST wait until it receives the EAP-Response before sending another fragment" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-3, so no unit is bound to it.

### [`RFC5216-2.1.5-4`](#rfc5216-2.1.5-4)

"the EAP server MUST increment the Identifier field for each fragment contained within an EAP-Request, and the peer MUST include this Identifier value in the fragment ACK contained within the EAP-Response" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-4, so no unit is bound to it.

### [`RFC5216-2.1.5-5`](#rfc5216-2.1.5-5)

"when the EAP server receives an EAP-Response with the M bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS and no data" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-5, so no unit is bound to it.

### [`RFC5216-2.1.5-6`](#rfc5216-2.1.5-6)

"The EAP peer MUST wait until it receives the EAP-Request before sending another fragment" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-6, so no unit is bound to it.

### [`RFC5216-2.1.5-7`](#rfc5216-2.1.5-7)

"the EAP server MUST increment the Identifier value for each fragment ACK contained within an EAP-Request, and the peer MUST include this Identifier value in the subsequent fragment contained within an EAP-Response" (§2.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.5-7, so no unit is bound to it.

### [`RFC5216-2.4-7`](#rfc5216-2.4-7)

"EAP-TLS implementations MUST support TLS v1.0" (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.4-7, so no unit is bound to it.

### [`RFC5216-3-3`](#rfc5216-3-3)

"The Identifier field MUST be changed on each Request packet" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-3-3, so no unit is bound to it.

### [`RFC5216-3-4`](#rfc5216-3-4)

"Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-3-4, so no unit is bound to it.

### [`RFC5216-3-5`](#rfc5216-3-5)

"Implementations of this specification MUST set the reserved bits to zero, and MUST ignore them on reception" -- the reception half of that sentence (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-3-5, so no unit is bound to it.

### [`RFC5216-3-6`](#rfc5216-3-6)

In an EAP-TLS Response, "The Identifier field is one octet and MUST match the Identifier field from the corresponding request" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-3-6, so no unit is bound to it.

### [`RFC5216-5.2-1`](#rfc5216-5.2-1)

"Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-1, so no unit is bound to it.

### [`RFC5216-5.2-2`](#rfc5216-5.2-2)

"If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-2, so no unit is bound to it.

### [`RFC5216-5.2-3`](#rfc5216-5.2-3)

"Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-3, so no unit is bound to it.

### [`RFC5216-5.2-4`](#rfc5216-5.2-4)

"The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-4, so no unit is bound to it.

### [`RFC5216-5.2-5`](#rfc5216-5.2-5)

"Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN)" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-5, so no unit is bound to it.

### [`RFC5216-5.3-2`](#rfc5216-5.3-2)

"Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.3-2, so no unit is bound to it.

### [`RFC5216-5.3-3`](#rfc5216-5.3-3)

When comparing the Extended Key Usage of the presented certificate, "implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.3-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5216.txt |
| Source fingerprint | fbadcfb857b6d879 |
| Record | rfc/extraction/rfc5216.json |
| Mapped sentences | 45 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.1.1` | not stated | 11 | walked | not stated |
| `2.1.2` | not stated | 1 | walked | not stated |
| `2.1.3` | not stated | 7 | walked | not stated |
| `2.1.4` | not stated | 5 | walked | not stated |
| `2.1.5` | not stated | 8 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 4 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 5 | walked | not stated |
| `5.3` | not stated | 2 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1.4:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Section 2.1.4 makes privacy optional: "EAP-TLS peer and server implementations MAY support privacy." Ze declined it: internal/core/eap carries no privacy mode, no anonymous-NAI identity path and no empty-certificate-list handling, so every obligation this sentence conditions on supporting or being configured for privacy is out of scope. | In order to avoid disclosing the peer username, an EAP-TLS peer configured for privacy MUST negotiate a TLS ciphersuite supporting confidentiality and MUST provide a client certificate list containing no entries in response to the initial certificate_request from the EAP-TLS server. |
| `2.1.4:2` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Section 2.1.4 makes privacy optional: "EAP-TLS peer and server implementations MAY support privacy." Ze declined it: internal/core/eap carries no privacy mode, no anonymous-NAI identity path and no empty-certificate-list handling, so every obligation this sentence conditions on supporting or being configured for privacy is out of scope. | An EAP-TLS server supporting privacy MUST NOT treat a certificate list containing no entries as a terminal condition; instead, it MUST bring up the TLS session and then send a hello_request. |
| `2.1.4:3` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Section 2.1.4 makes privacy optional: "EAP-TLS peer and server implementations MAY support privacy." Ze declined it: internal/core/eap carries no privacy mode, no anonymous-NAI identity path and no empty-certificate-list handling, so every obligation this sentence conditions on supporting or being configured for privacy is out of scope. | An EAP-TLS peer supporting privacy MUST provide a certificate list containing at least one entry in response to the subsequent certificate_request sent by the server. |
| `2.1.4:4` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Section 2.1.4 makes privacy optional: "EAP-TLS peer and server implementations MAY support privacy." Ze declined it: internal/core/eap carries no privacy mode, no anonymous-NAI identity path and no empty-certificate-list handling, so every obligation this sentence conditions on supporting or being configured for privacy is out of scope. | If the EAP-TLS server supporting privacy does not receive a client certificate in response to the subsequent certificate_request, then it MUST abort the session. |
| `2.1.4:5` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | Section 2.1.4 makes privacy optional: "EAP-TLS peer and server implementations MAY support privacy." Ze declined it: internal/core/eap carries no privacy mode, no anonymous-NAI identity path and no empty-certificate-list handling, so every obligation this sentence conditions on supporting or being configured for privacy is out of scope. | EAP-TLS servers supporting privacy MUST request a client certificate, and MUST be able to accept a client certificate offered by the EAP-TLS peer, in order to preserve interoperability with EAP- TLS peers that do not support privacy. |
| `2.1.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 2.1.5 restates the L-flag obligation that Section 3.1 states for the Request packet and RFC5216-3-1 already maps. | The L flag is set to indicate the presence of the four-octet TLS Message Length field, and MUST be set for the first fragment of a fragmented TLS message or set of messages. |
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2 repeats the Section 3.1 sentence about octets outside the Length field word for word; RFC5216-3-4 maps it. | Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception. |
| `3.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.2 repeats the Section 3.1 L-bit sentence word for word; RFC5216-3-1 maps it. | The L bit (length included) is set to indicate the presence of the four-octet TLS Message Length field, and MUST be set for the first fragment of a fragmented TLS message or set of messages. |

## Superseded

No document obsoletes RFC 5216, so its obligations are stated where they were written.
