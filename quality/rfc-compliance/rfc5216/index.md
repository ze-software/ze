# RFC 5216 - The EAP-TLS Authentication Protocol

Partial. Every requirement this repository extracted from RFC 5216, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.0% | 31 of 47 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 12.8% | 6 of 47 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 47 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 57.9% | 44 of 76 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 47 | of 52 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 47 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 47 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 4.3% | 2 of 47 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 47 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 17.0% | 8 of 47 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 38 | of 47 gated MUSTs judged | 18 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 52 |
| Gated MUST-level | 47 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 76 |
| Tagged units | 76 |
| Recorded audit verdicts | 38 |
| Discrimination records | 44 |
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

[`RFC5216-2.4-5`](#rfc5216-2.4-5), the mandatory TLS_RSA_WITH_3DES_EDE_CBC_SHA ciphersuite, is not offered (TLS 1.2+ Go defaults exclude insecure 3DES). The 15 rows the 2026-09-21 extraction walk added were classified the same day under the owner ruling that every MUST is a requirement. Five TLS handshake obligations crypto/tls performs on the tls.Config Ze installs (the resumption ciphersuite, the server_certificate flight, the peer's full and resumed flights, and the server's resumed flight) are proven at that boundary in rfc5216_tls_config_test.go; the server_key_exchange ([`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7)) is decided by the crypto/tls default suites and carries a lower-layer annotation. [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4) is audited wrong: the server does not choose a sessionId for a new session, because crypto/tls, as newTLSMethod configures it, sends an empty ServerHello session_id on a TLS 1.2 full handshake, and its tags prove the session-ticket rule instead.

- **Seven are scheduled gaps:** TLS 1.0 support ([`RFC5216-2.4-7`](#rfc5216-2.4-7), which RFC 8996 forbids, [`plan/spec-eap-tls-tls10.md`](https://github.com/ze-software/ze/blob/main/plan/spec-eap-tls-tls10.md), the owner decides which document governs); the Peer-Id and Server-Id derivation and authorization ([`RFC5216-5.2-1`](#rfc5216-5.2-1), [`RFC5216-5.3-2`](#rfc5216-5.3-2), [`RFC5216-5.3-3`](#rfc5216-5.3-3), [`plan/immediate/spec-eap-tls-identity-authorization.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-eap-tls-identity-authorization.md)); and the certificate naming profile on issuers and verifiers ([`RFC5216-5.2-2`](#rfc5216-5.2-2), [`RFC5216-5.2-3`](#rfc5216-5.2-3), [`RFC5216-5.2-4`](#rfc5216-5.2-4), [`plan/spec-eap-tls-certificate-profile.md`](https://github.com/ze-software/ze/blob/main/plan/spec-eap-tls-certificate-profile.md)). [`RFC5216-5.2-5`](#rfc5216-5.2-5) is structural in crypto/x509 and carries a lower-layer annotation. The 12 rows the same walk added that are now proven in both polarities are the Section 2.1.1 Start and first Response, the six Section 2.1.5 fragment-ACK and Identifier obligations, and the four Section 3 packet-field obligations (rfc5216_flight_rules_test.go and rfc5216_fragment_ack_test.go, internal/core/eap). [`RFC5216-5.4-2`](#rfc5216-5.4-2) is no longer among them: the peer re-checks the revocation status of the authenticator certificate once the CHILD_SA gives it a network, over https, and closes the SA when the responder reports it revoked (startServerCertRecheck, [`internal/component/ike/engine/postauth_revocation.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/postauth_revocation.go), reached from runEstablished). It runs whichever TLS version the exchange negotiated, which is what this RFC asks for, and the chain it re-reads is the one the peer accepted (eap.PeerSession.ServerChains). [`RFC5216-5.4-1`](#rfc5216-5.4-1) is no longer among them either: checkChainRevocation ([`internal/core/eap/revocation.go`](https://github.com/ze-software/ze/blob/main/internal/core/eap/revocation.go)) checks every certificate on each verified chain except the trust anchor against the crl leaf-list its CA publishes, on the authenticator through tls.Config.VerifyConnection and on the peer through serverChainCheck.verifyConnection, and it is proven in both polarities on TLS 1.2, which is the version this RFC governs.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 31 | one part of the gated population |
| Annotated instead of tested | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **47** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (31):** [`RFC5216-3-1`](#rfc5216-3-1), [`RFC5216-2.1.1-1`](#rfc5216-2.1.1-1), [`RFC5216-2.1.1-2`](#rfc5216-2.1.1-2), [`RFC5216-2.1.5-1`](#rfc5216-2.1.5-1), [`RFC5216-5.4-1`](#rfc5216-5.4-1), [`RFC5216-5.4-2`](#rfc5216-5.4-2), [`RFC5216-2.1.3-1`](#rfc5216-2.1.3-1), [`RFC5216-2.1.3-3`](#rfc5216-2.1.3-3), [`RFC5216-2.1.3-4`](#rfc5216-2.1.3-4), [`RFC5216-2.1.3-5`](#rfc5216-2.1.3-5), [`RFC5216-2.1.3-6`](#rfc5216-2.1.3-6), [`RFC5216-2.1.3-7`](#rfc5216-2.1.3-7), [`RFC5216-2.1.3-8`](#rfc5216-2.1.3-8), [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3), [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4), [`RFC5216-2.1.1-5`](#rfc5216-2.1.1-5), [`RFC5216-2.1.1-6`](#rfc5216-2.1.1-6), [`RFC5216-2.1.1-8`](#rfc5216-2.1.1-8), [`RFC5216-2.1.1-9`](#rfc5216-2.1.1-9), [`RFC5216-2.1.1-10`](#rfc5216-2.1.1-10), [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1), [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2), [`RFC5216-2.1.5-3`](#rfc5216-2.1.5-3), [`RFC5216-2.1.5-4`](#rfc5216-2.1.5-4), [`RFC5216-2.1.5-5`](#rfc5216-2.1.5-5), [`RFC5216-2.1.5-6`](#rfc5216-2.1.5-6), [`RFC5216-2.1.5-7`](#rfc5216-2.1.5-7), [`RFC5216-3-3`](#rfc5216-3-3), [`RFC5216-3-4`](#rfc5216-3-4), [`RFC5216-3-5`](#rfc5216-3-5), [`RFC5216-3-6`](#rfc5216-3-6)

**Annotated instead of tested (16):** [`RFC5216-3-2`](#rfc5216-3-2), [`RFC5216-2.4-1`](#rfc5216-2.4-1), [`RFC5216-2.4-2`](#rfc5216-2.4-2), [`RFC5216-2.4-3`](#rfc5216-2.4-3), [`RFC5216-2.4-4`](#rfc5216-2.4-4), [`RFC5216-2.4-5`](#rfc5216-2.4-5), [`RFC5216-2.3-1`](#rfc5216-2.3-1), [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7), [`RFC5216-2.4-7`](#rfc5216-2.4-7), [`RFC5216-5.2-1`](#rfc5216-5.2-1), [`RFC5216-5.2-2`](#rfc5216-5.2-2), [`RFC5216-5.2-3`](#rfc5216-5.2-3), [`RFC5216-5.2-4`](#rfc5216-5.2-4), [`RFC5216-5.2-5`](#rfc5216-5.2-5), [`RFC5216-5.3-2`](#rfc5216-5.3-2), [`RFC5216-5.3-3`](#rfc5216-5.3-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5216-3-1` | The L bit (length included) is set to indicate the presence of the four-octet TLS Message Length field, and MUST be set for the first fragment of a fragmented TLS message or set of messages. (§3, Flags) | MUST | 3 | **positive:** `unit/verify` [`TestTLSFragmenterFirstFragmentHasLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L395). **negative:** `unit/verify` [`TestTLSFragmenterMiddleAndLastFragmentsHaveNoLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L475) |
| `RFC5216-3-2` | Implementations of this specification MUST set the reserved bits to zero (§3, Flags) | MUST | 3 | **positive:** `unit/verify` [`TestTLSFragmentReservedFlagBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L494). **negative:** no negative test. **{single-polarity}:** the flags byte is assembled only from L/M in nextFragment (and S alone in Start), so no code path can set reserved bits 3-7 and only the positive assertion is reachable (internal/core/eap/eap_tls.go:106, :190) |
| `RFC5216-2.4-1` | The version offered by the peer MUST correspond to TLS v1.0 or later. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L364). **negative:** no negative test. **{single-polarity}:** the peer's tls.Config forces MinVersion TLS 1.2, so its ClientHello always offers at least TLS 1.0 and no path can offer lower (internal/core/eap/peer.go:334) |
| `RFC5216-2.4-2` | The version offered by the server MUST correspond to TLS v1.0 or later. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L367). **negative:** no negative test. **{single-polarity}:** the server's tls.Config forces MinVersion TLS 1.2, so any ServerHello it emits is at least TLS 1.0 and it never negotiates lower (internal/core/eap/eap_tls.go:167) |
| `RFC5216-2.4-3` | However, during the EAP-TLS conversation the EAP peer and server MUST NOT request or negotiate compression. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L370). **negative:** no negative test. **{single-polarity}:** both tls.Config values delegate to Go crypto/tls, which implements no TLS compression and always sends null compression, so the prohibition holds structurally with no knob to falsify (internal/core/eap/eap_tls.go:163, peer.go:330) |
| `RFC5216-2.4-4` | Since the ciphersuite negotiated within EAP-TLS applies only to the EAP conversation, TLS ciphersuite negotiation MUST NOT be used to negotiate the ciphersuites used to secure data. (§2.4) | MUST NOT | 2.4 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L375). **negative:** no negative test. **{single-polarity}:** the only value ze extracts from the TLS session is the 64-octet MSK fed to the IKEv2 AUTH payload; the ESP/data-plane ciphers come from independent IKE SA proposal negotiation, never from the TLS ciphersuite (internal/core/eap/eap_tls.go:268) |
| `RFC5216-2.4-5` | To ensure interoperability, EAP-TLS peers and servers MUST support the TLS [RFC4346] mandatory-to-implement ciphersuite: TLS_RSA_WITH_3DES_EDE_CBC_SHA (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the server sets no CipherSuites and requires TLS 1.2+, so it inherits Go's secure defaults which classify 3DES as insecure and do not enable TLS_RSA_WITH_3DES_EDE_CBC_SHA, leaving the RFC's mandatory-to-implement ciphersuite unavailable (internal/core/eap/eap_tls.go:163, peer.go:330) |
| `RFC5216-2.1.1-1` | If the EAP server sent a certificate_request message in the preceding EAP-Request packet, then unless the peer is configured for privacy (see Section 2.1.4) the peer MUST send, in addition, certificate and certificate_verify messages. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L349). **negative:** `unit/verify` [`TestEAPTLSAuthenticatorRequiresClientCert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L444) |
| `RFC5216-2.1.1-2` | The EAP-TLS conversation will then begin, with the peer sending an EAP-Response packet with EAP-Type=EAP-TLS. The data field of that packet will encapsulate one or more TLS records in TLS record layer format, containing a TLS client_hello handshake message. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestEAPTLSPeerFirstResponseCarriesClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L89). **negative:** `unit/verify` [`TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L231) |
| `RFC5216-2.1.5-1` | As a result, an EAP-TLS implementation MUST provide its own support for fragmentation and reassembly. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestTLSFragmenterRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L348). **negative:** `unit/verify` [`TestTLSReassemblyRejectsOversized`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L444) |
| `RFC5216-5.3-1` | it SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation, including the ability to retrieve intermediate certificates that may be necessary to validate the peer certificate. For details, see Section 4.2.2.1 of [RFC3280]. Where the EAP-TLS server is unable to retrieve intermediate certificates, either it will need to be pre-configured with the necessary intermediate certificates to complete path validation or it will rely on the EAP-TLS peer to provide this information as part of the TLS handshake (see Section 7.4.6 of [RFC4346]). In contrast to the EAP-TLS server, the EAP-TLS peer may not have Internet connectivity. Therefore, the EAP-TLS server SHOULD provide its entire certificate chain minus the root to facilitate certificate validation by the peer. The EAP-TLS peer SHOULD support validating the server certificate using RFC 3280 [RFC3280] compliant path validation. (§5.3) | SHOULD | 5.3 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L355). **negative:** `unit/verify` [`TestEAPTLSPeerRejectsUntrustedServerChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L497). **negative:** `unit/verify` [`TestEAPTLSPeerWithoutCARefusesToStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L532). **negative:** `unit/verify` [`TestEAPTLSServerRejectsUntrustedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L468) |
| `RFC5216-5.4-1` | EAP-TLS peer and server implementations MUST support the use of Certificate Revocation Lists (CRLs); for details, see Section 3.3 of [RFC3280]. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS12RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L353). **negative:** `unit/verify` [`TestEAPTLS12CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L384) |
| `RFC5216-5.4-2` | peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L396). **positive:** `unit/verify` [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L253). **negative:** `unit/verify` [`TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L416). **negative:** `unit/verify` [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L283) |
| `RFC5216-2.1.3-1` | The EAP Server MUST reply with an EAP-Failure packet since server authentication failure is a terminal condition. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216ServerRepliesEAPFailureToPeerAlert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L119). **negative:** `unit/verify` [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L202) |
| `RFC5216-2.1.3-3` | To ensure that the peer receives the TLS alert message, the EAP server MUST wait for the peer to reply with an EAP-Response packet. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L95). **negative:** `unit/verify` [`TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L361) |
| `RFC5216-2.1.3-4` | it MAY contain an EAP-Response packet with EAP-Type=EAP-TLS and no data, in which case the EAP-Server MUST send an EAP-Failure packet and terminate the conversation. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L101). **positive:** `unit/verify` [`TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L208). **negative:** `unit/verify` [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L207) |
| `RFC5216-2.1.3-5` | If the peer authenticates successfully, the EAP server MUST respond with an EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new TLS session, one or more TLS records containing TLS change_cipher_spec and finished handshake messages. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L229). **negative:** `unit/verify` [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L333) |
| `RFC5216-2.1.3-6` | To ensure that the EAP Server receives the TLS alert message, the peer MUST wait for the EAP Server to reply before terminating the conversation. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216PeerRepliesBeforeItTerminates`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L41). **negative:** `unit/verify` [`TestRFC5216PeerDoesNotWaitWhenItSentNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L168) |
| `RFC5216-2.1.3-7` | If the EAP server authenticates successfully, the peer MUST send an EAP-Response packet of EAP-Type=EAP-TLS, and no data. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L237). **negative:** `unit/verify` [`TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L386) |
| `RFC5216-2.1.3-8` | The EAP Server then MUST respond with an EAP-Success message. (§2.1.3) | MUST | 2.1.3 | **positive:** `unit/verify` [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L242). **negative:** `unit/verify` [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L339) |
| `RFC5216-2.3-1` | Key_Material = TLS-PRF-128(master_secret, "client EAP encryption", client.random \|\| server.random) MSK = Key_Material(0,63) EMSK = Key_Material(64,127) (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L360). **positive:** `unit/verify` [`TestRFC5216MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L112). **negative:** no negative test. **{single-polarity}:** both server and peer export the 64-octet MSK via ExportKeyingMaterial with the exact RFC label 'client EAP encryption', the only key the IKEv2 lower layer consumes (internal/core/eap/eap_tls.go:268, peer.go:381) |
| `RFC5216-2.1.1-3` | Once having received the peer's Identity, the EAP server MUST respond with an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS, the Start (S) bit set, and no data. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216StartAnswersTheIdentityResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L70). **negative:** `unit/verify` [`TestRFC5216NoStartBeforeTheIdentityIsReceived`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L102) |
| `RFC5216-2.1.1-4` | If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L35). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L75) |
| `RFC5216-2.1.1-5` | If the session matches the peer's, then the ciphersuite MUST match the one negotiated during the handshake protocol execution that established the session. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L36). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L76) |
| `RFC5216-2.1.1-6` | If the EAP server is not resuming a previously established session, then it MUST include a TLS server_certificate handshake message, and a server_hello_done handshake message MUST be the last handshake message encapsulated in this EAP-Request packet. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L37). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L77) |
| `RFC5216-2.1.1-7` | The certificate message contains a public key certificate chain for either a key exchange public key (such as an RSA or Diffie-Hellman key exchange public key) or a signature public key (such as an RSA or Digital Signature Standard (DSS) signature public key). In the latter case, a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/tls; internal/core/eap/eap_tls.go::newTLSMethod sets no CipherSuites, so the default ECDHE suites of crypto/tls decide that a server_key_exchange is written and no value Ze writes chooses it |
| `RFC5216-2.1.1-8` | If the peer supports EAP-TLS and is configured to use it, it MUST respond to the EAP-Request with an EAP-Response packet of EAP- Type=EAP-TLS. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216ConfiguredPeerAnswersEAPTLSWithEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L132). **negative:** `unit/verify` [`TestRFC5216PeerNotConfiguredForEAPTLSDoesNotAnswerWithEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L165) |
| `RFC5216-2.1.1-9` | If the preceding server_hello message sent by the EAP server in the preceding EAP-Request packet did not indicate the resumption of a previous session, the data field of this packet MUST encapsulate one or more TLS records containing a TLS client_key_exchange, change_cipher_spec, and finished messages. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L38). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L78) |
| `RFC5216-2.1.1-10` | If the preceding server_hello message sent by the EAP server in the preceding EAP-Request packet indicated the resumption of a previous session, then the peer MUST send only the change_cipher_spec and finished handshake messages. (§2.1.1) | MUST | 2.1.1 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L39). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L79) |
| `RFC5216-2.1.2-1` | If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message. (§2.1.2) | MUST | 2.1.2 | **positive:** `unit/verify` [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L40). **negative:** `unit/verify` [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L80) |
| `RFC5216-2.1.5-2` | When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216PeerAcksEachServerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L199). **negative:** `unit/verify` [`TestRFC5216PeerAcksEachServerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L205) |
| `RFC5216-2.1.5-3` | The EAP server MUST wait until it receives the EAP-Response before sending another fragment. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L241). **negative:** `unit/verify` [`TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L247) |
| `RFC5216-2.1.5-4` | In order to prevent errors in processing of fragments, the EAP server MUST increment the Identifier field for each fragment contained within an EAP-Request, and the peer MUST include this Identifier value in the fragment ACK contained within the EAP-Response. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L273). **negative:** `unit/verify` [`TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L279) |
| `RFC5216-2.1.5-5` | Similarly, when the EAP server receives an EAP-Response with the M bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS and no data. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216ServerAcksEachPeerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L306). **negative:** `unit/verify` [`TestRFC5216ServerAcksEachPeerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L312) |
| `RFC5216-2.1.5-6` | The EAP peer MUST wait until it receives the EAP-Request before sending another fragment. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L349). **negative:** `unit/verify` [`TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L355) |
| `RFC5216-2.1.5-7` | In order to prevent errors in the processing of fragments, the EAP server MUST increment the Identifier value for each fragment ACK contained within an EAP-Request, and the peer MUST include this Identifier value in the subsequent fragment contained within an EAP- Response. (§2.1.5) | MUST | 2.1.5 | **positive:** `unit/verify` [`TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L382). **negative:** `unit/verify` [`TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L388) |
| `RFC5216-2.4-7` | EAP-TLS implementations MUST support TLS v1.0. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** both roles set MinVersion TLS 1.2 and RFC 8996 forbids TLS 1.0, so the owner decides which document governs; plan/spec-eap-tls-tls10.md |
| `RFC5216-3-3` | The Identifier field MUST be changed on each Request packet. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5216IdentifierChangesOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L186). **negative:** `unit/verify` [`TestRFC5216IdentifierChangesOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L190) |
| `RFC5216-3-4` | Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5216PaddingPastTheLengthIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L227). **negative:** `unit/verify` [`TestRFC5216PaddingPastTheLengthIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L232) |
| `RFC5216-3-5` | Implementations of this specification MUST set the reserved bits to zero, and MUST ignore them on reception. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC5216ReservedFlagBitsAreIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L271). **negative:** `unit/verify` [`TestRFC5216ReservedFlagBitsAreIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L276) |
| `RFC5216-3-6` | The Identifier field is one octet and MUST match the Identifier field from the corresponding request. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5216ResponseIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L348). **negative:** `unit/verify` [`TestRFC5216ResponseIdentifierIsCopiedNotCounted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L373) |
| `RFC5216-5.2-1` | Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** eapTLSPeerName reads the subject and never the subjectAltName, and no Server-Id is exported; plan/immediate/spec-eap-tls-identity-authorization.md |
| `RFC5216-5.2-2` | If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no verifier refuses an empty-subject leaf whose subjectAltName is not critical; plan/spec-eap-tls-certificate-profile.md |
| `RFC5216-5.2-3` | Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no Ze issuer takes an NAI to write as an rfc822Name; plan/spec-eap-tls-certificate-profile.md |
| `RFC5216-5.2-4` | The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no verifier refuses a subject carrying an emailAddress RDN; plan/spec-eap-tls-certificate-profile.md |
| `RFC5216-5.2-5` | Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN). (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Go crypto/x509; internal/component/pki/ca.go::LoadOrGenerateRoot issues every subject as a pkix.Name and x509.ParseCertificate accepts no other encoding, so no value Ze writes can make a non-empty subject anything but a distinguished name |
| `RFC5216-5.3-2` | Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither role authorizes the Peer-Id or Server-Id against configuration; plan/immediate/spec-eap-tls-identity-authorization.md |
| `RFC5216-5.3-3` | When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the peer sets no expected server identity, so no RFC 2818 Section 3.1 comparison runs; plan/immediate/spec-eap-tls-identity-authorization.md |
| `RFC5216-5.4-3` | EAP-TLS peer and server implementations SHOULD also support the Online Certificate Status Protocol (OCSP) (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-5.4-4` | EAP-TLS peers and servers SHOULD implement Certificate Status Request messages (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.4-6` | EAP-TLS peers and servers SHOULD also support and be able to negotiate the following TLS ciphersuites: TLS_RSA_WITH_RC4_128_SHA [RFC4346] TLS_RSA_WITH_AES_128_CBC_SHA [RFC3268] (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5216-2.1.3-2` | in which case the EAP server MAY allow the EAP-TLS conversation to be restarted, or it MAY contain an EAP-Response packet with EAP-Type=EAP-TLS and no data, in which case the EAP-Server MUST send an EAP-Failure packet and terminate the conversation. It is up to the EAP server whether to allow restarts, and if so, how many times the conversation can be restarted. An EAP Server implementing restart capability SHOULD impose a per-peer limit on the number of restarts, so as to protect against denial-of-service attacks. (§2.1.3) | MAY | 2.1.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5216-2.4-5`](#rfc5216-2.4-5) To ensure interoperability, EAP-TLS peers and servers MUST support the TLS [RFC4346] mandatory-to-implement ciphersuite: TLS_RSA_WITH_3DES_EDE_CBC_SHA (§2.4) | {gap}, no test | the server sets no CipherSuites and requires TLS 1.2+, so it inherits Go's secure defaults which classify 3DES as insecure and do not enable TLS_RSA_WITH_3DES_EDE_CBC_SHA, leaving the RFC's mandatory-to-implement ciphersuite unavailable (internal/core/eap/eap_tls.go:163, peer.go:330) |
| [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7) The certificate message contains a public key certificate chain for either a key exchange public key (such as an RSA or Diffie-Hellman key exchange public key) or a signature public key (such as an RSA or Digital Signature Standard (DSS) signature public key). In the latter case, a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place. (§2.1.1) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/tls; internal/core/eap/eap_tls.go::newTLSMethod sets no CipherSuites, so the default ECDHE suites of crypto/tls decide that a server_key_exchange is written and no value Ze writes chooses it |
| [`RFC5216-2.4-7`](#rfc5216-2.4-7) EAP-TLS implementations MUST support TLS v1.0. (§2.4) | {gap}, no test | both roles set MinVersion TLS 1.2 and RFC 8996 forbids TLS 1.0, so the owner decides which document governs; plan/spec-eap-tls-tls10.md |
| [`RFC5216-5.2-1`](#rfc5216-5.2-1) Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName. (§5.2) | {gap}, no test | eapTLSPeerName reads the subject and never the subjectAltName, and no Server-Id is exported; plan/immediate/spec-eap-tls-identity-authorization.md |
| [`RFC5216-5.2-2`](#rfc5216-5.2-2) If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical. (§5.2) | {gap}, no test | no verifier refuses an empty-subject leaf whose subjectAltName is not critical; plan/spec-eap-tls-certificate-profile.md |
| [`RFC5216-5.2-3`](#rfc5216-5.2-3) Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities. (§5.2) | {gap}, no test | no Ze issuer takes an NAI to write as an rfc822Name; plan/spec-eap-tls-certificate-profile.md |
| [`RFC5216-5.2-4`](#rfc5216-5.2-4) The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used. (§5.2) | {gap}, no test | no verifier refuses a subject carrying an emailAddress RDN; plan/spec-eap-tls-certificate-profile.md |
| [`RFC5216-5.2-5`](#rfc5216-5.2-5) Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN). (§5.2) | no test | no test carries this requirement id; annotated {lower-layer}: Go crypto/x509; internal/component/pki/ca.go::LoadOrGenerateRoot issues every subject as a pkix.Name and x509.ParseCertificate accepts no other encoding, so no value Ze writes can make a non-empty subject anything but a distinguished name |
| [`RFC5216-5.3-2`](#rfc5216-5.3-2) Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS. (§5.3) | {gap}, no test | neither role authorizes the Peer-Id or Server-Id against configuration; plan/immediate/spec-eap-tls-identity-authorization.md |
| [`RFC5216-5.3-3`](#rfc5216-5.3-3) When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]. (§5.3) | {gap}, no test | the peer sets no expected server identity, so no RFC 2818 Section 3.1 comparison runs; plan/immediate/spec-eap-tls-identity-authorization.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5216-3-1`](#rfc5216-3-1)

The L bit (length included) is set to indicate the presence of the four-octet TLS Message Length field, and MUST be set for the first fragment of a fragmented TLS message or set of messages. (§3, Flags)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive: first fragment of a multi-fragment message carries L and the 4-octet length; the negative asserts L clear on middle and last fragments, which RFC 5216 does not state (the tag prose calls it a MUST NOT)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTLSFragmenterMiddleAndLastFragmentsHaveNoLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L475) | unit/verify | unproven |
| positive | [`TestTLSFragmenterFirstFragmentHasLength`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L395) | unit/verify | unproven |

### [`RFC5216-3-2`](#rfc5216-3-2)

Implementations of this specification MUST set the reserved bits to zero (§3, Flags)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive. Forbidden: an emitted flags octet with any of bits 3-7 set. TestTLSFragmentReservedFlagBitsAreZero fatals if the S constant or any flags octet nextFragment produces for first (L+M), middle (M), last and single-fragment (L) sends intersects 0x1F. The only other flags octet sent is the literal 0x00 ACK.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTLSFragmentReservedFlagBitsAreZero`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L494) | unit/verify | unproven |

### [`RFC5216-2.4-1`](#rfc5216-2.4-1)

The version offered by the peer MUST correspond to TLS v1.0 or later. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a peer offering a version below TLS 1.0. The tagged assertion reads the NEGOTIATED ps.Version >= TLS10, not the offer, and it cannot go red: crypto/tls has no version below TLS 1.0. What Ze installs for the layer below, the peer tls.Config MinVersion TLS 1.2, is asserted by no tagged unit, so lowering it passes. The single-polarity marker is sound; the positive proves nothing Ze controls.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L364) | unit/verify | unproven |

### [`RFC5216-2.4-2`](#rfc5216-2.4-2)

The version offered by the server MUST correspond to TLS v1.0 or later. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a server offering a version below TLS 1.0. The tagged assertion reads the negotiated ss.Version >= TLS10, which crypto/tls cannot violate, so it cannot go red. The authenticator MinVersion TLS 1.2 assertion lives in TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry, which is not tagged for this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L367) | unit/verify | unproven |

### [`RFC5216-2.4-3`](#rfc5216-2.4-3)

However, during the EAP-TLS conversation the EAP peer and server MUST NOT request or negotiate compression. (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the only assertion is HandshakeComplete on both sides, which cannot fail on compression; the property is structural in crypto/tls and would read more honestly as a lower-layer annotation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L370) | unit/verify | unproven |

### [`RFC5216-2.4-4`](#rfc5216-2.4-4)

Since the ciphersuite negotiated within EAP-TLS applies only to the EAP conversation, TLS ciphersuite negotiation MUST NOT be used to negotiate the ciphersuites used to secure data. (§2.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the assertion is a 64-octet equal non-zero MSK; it cannot fail if the TLS ciphersuite were used to choose the IKE/ESP data ciphers

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L375) | unit/verify | unproven |

### [`RFC5216-2.4-5`](#rfc5216-2.4-5)

To ensure interoperability, EAP-TLS peers and servers MUST support the TLS [RFC4346] mandatory-to-implement ciphersuite: TLS_RSA_WITH_3DES_EDE_CBC_SHA (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.4-5, so no unit is bound to it.

### [`RFC5216-2.1.1-1`](#rfc5216-2.1.1-1)

If the EAP server sent a certificate_request message in the preceding EAP-Request packet, then unless the peer is configured for privacy (see Section 2.1.4) the peer MUST send, in addition, certificate and certificate_verify messages. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive proves the peer certificate reached the authenticator (a peer omitting it would fail the handshake); the negative asserts the authenticator ClientAuth config, which is the certificate_request precondition, not a peer that omits certificate or certificate_verify

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSAuthenticatorRequiresClientCert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L444) | unit/verify | unproven |
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L349) | unit/verify | unproven |

### [`RFC5216-2.1.1-2`](#rfc5216-2.1.1-2)

The EAP-TLS conversation will then begin, with the peer sending an EAP-Response packet with EAP-Type=EAP-TLS. The data field of that packet will encapsulate one or more TLS records in TLS record layer format, containing a TLS client_hello handshake message. (§2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the answer to the Start is not an EAP-Response of Type 13, or its data is not a TLS record carrying client_hello. TestEAPTLSPeerFirstResponseCarriesClientHello fatals on Code != CodeResponse or Type != TypeTLS, and assertHandshakeRecord fails unless the TypeData holds a handshake record of type client_hello. Negative: TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging fatals if a peer whose TLS engine produced nothing sends any Response instead of errTLSClientStalled, so an EAP-TLS Response with no client_hello is refused.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSPeerStalledClientFailsRatherThanAcknowledging`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L231) | unit/verify | unproven |
| positive | [`TestEAPTLSPeerFirstResponseCarriesClientHello`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_flight_test.go#L89) | unit/verify | unproven |

### [`RFC5216-2.1.5-1`](#rfc5216-2.1.5-1)

As a result, an EAP-TLS implementation MUST provide its own support for fragmentation and reassembly. (§2.1.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive round-trips a 3000-octet message over fragments; the negative asserts the reassembly cap on an oversized L length, a neighbouring DoS bound, not an implementation that fails to fragment or reassemble

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTLSReassemblyRejectsOversized`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L444) | unit/verify | unproven |
| positive | [`TestTLSFragmenterRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/eap/peer_test.go#L348) | unit/verify | unproven |

### [`RFC5216-5.3-1`](#rfc5216-5.3-1)

it SHOULD support validating the peer certificate using RFC 3280 [RFC3280] compliant path validation, including the ability to retrieve intermediate certificates that may be necessary to validate the peer certificate. For details, see Section 4.2.2.1 of [RFC3280]. Where the EAP-TLS server is unable to retrieve intermediate certificates, either it will need to be pre-configured with the necessary intermediate certificates to complete path validation or it will rely on the EAP-TLS peer to provide this information as part of the TLS handshake (see Section 7.4.6 of [RFC4346]). In contrast to the EAP-TLS server, the EAP-TLS peer may not have Internet connectivity. Therefore, the EAP-TLS server SHOULD provide its entire certificate chain minus the root to facilitate certificate validation by the peer. The EAP-TLS peer SHOULD support validating the server certificate using RFC 3280 [RFC3280] compliant path validation. (§5.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. both path-validation SHOULDs proven (server refuses untrusted client chain, peer refuses untrusted server chain, peer without anchor refuses to start); the quoted span also carries intermediate-certificate retrieval and the server SHOULD send its chain minus the root, which no tagged unit asserts

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSPeerRejectsUntrustedServerChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L497) | unit/verify | unproven |
| negative | [`TestEAPTLSPeerWithoutCARefusesToStart`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L532) | unit/verify | unproven |
| negative | [`TestEAPTLSServerRejectsUntrustedClientChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L468) | unit/verify | unproven |
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L355) | unit/verify | unproven |

### [`RFC5216-5.4-1`](#rfc5216-5.4-1)

EAP-TLS peer and server implementations MUST support the use of Certificate Revocation Lists (CRLs); for details, see Section 3.3 of [RFC3280]. (§5.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the TLS 1.2 units prove the authenticator refusing a CRL-revoked client certificate and completing with an empty list; the peer half (peer acting on a CRL over TLS 1.2) has no tagged unit for this id

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLS12CompletesWithAnUnrevokedChain`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L384) | unit/verify | revert, verified |
| positive | [`TestEAPTLS12RefusesARevokedClientCertificate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_revocation_test.go#L353) | unit/verify | revert, verified |

### [`RFC5216-5.4-2`](#rfc5216-5.4-2)

peer implementations MUST also support checking for certificate revocation after authentication completes and network connectivity is available (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a peer with no revocation check after authentication. TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked fatals if startServerCertRecheck (called from established.go after the exchange) starts no check or reports no verdict when the OCSP responder says revoked; TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood fatals on any verdict when it says good. The eap units prove the accepted TLS 1.2 chain is kept for the check and none is published on a refused handshake. The first unit's name claims the SA closes; it asserts only the verdict.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L283) | unit/verify | revert, verified |
| negative | [`TestEAPTLSPeerPublishesNoChainWhenTheHandshakeRefusedIt`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L416) | unit/verify | revert, verified |
| positive | [`TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc9190_postauth_test.go#L253) | unit/verify | revert, verified |
| positive | [`TestEAPTLS12PeerKeepsTheChainItAcceptedForTheLaterCheck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc9190_ocsp_test.go#L396) | unit/verify | revert, verified |

### [`RFC5216-2.1.3-1`](#rfc5216-2.1.3-1)

The EAP Server MUST reply with an EAP-Failure packet since server authentication failure is a terminal condition. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the server answering the peer's alert with anything but EAP-Failure, or later than the next packet. TestRFC5216ServerRepliesEAPFailureToPeerAlert fatals if no Failure is sent, errors on any Success, and errors unless failureAt == alertAt+1. Negative: TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate errors if a mutually authenticated conversation draws any Failure.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L202) | unit/verify | unproven |
| positive | [`TestRFC5216ServerRepliesEAPFailureToPeerAlert`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L119) | unit/verify | unproven |

### [`RFC5216-2.1.3-3`](#rfc5216-2.1.3-3)

To ensure that the peer receives the TLS alert message, the EAP server MUST wait for the peer to reply with an EAP-Response packet. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the server concluding (EAP-Failure) on the round it sends its alert, instead of waiting for the peer's Response. alertFlight fatals if the round that records the cause carries CodeFailure, TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure errors unless that round is an EAP-Request with a TLS alert record, and only the later Response draws the Failure. Negative: TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend fatals if a failure with no alert to send reports no error, so the wait is bound to an alert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEAPTLSAuthenticatorReportsTheFailureWithNoAlertToSend`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L361) | unit/verify | unproven |
| positive | [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L95) | unit/verify | revert, verified |

### [`RFC5216-2.1.3-4`](#rfc5216-2.1.3-4)

it MAY contain an EAP-Response packet with EAP-Type=EAP-TLS and no data, in which case the EAP-Server MUST send an EAP-Failure packet and terminate the conversation. (§2.1.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clause 1 (a no-data Response to the alert draws EAP-Failure): TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure errors on last.Code != CodeFailure, and TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure errors unless the Failure follows the alert. Clause 2 (terminate the conversation): no tagged unit feeds a packet after the EAP-Failure or asserts the session is closed, so an authenticator that keeps processing Responses after its Failure passes. The negative (no alert runs to EAP-Success) is sound.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_termination_test.go#L207) | unit/verify | unproven |
| positive | [`TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L101) | unit/verify | revert, verified |
| positive | [`TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_alert_flight_test.go#L208) | unit/verify | unproven |

### [`RFC5216-2.1.3-5`](#rfc5216-2.1.3-5)

If the peer authenticates successfully, the EAP server MUST respond with an EAP-Request packet with EAP-Type=EAP-TLS, which includes, in the case of a new TLS session, one or more TLS records containing TLS change_cipher_spec and finished handshake messages. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the packet before EAP-Success is not an EAP-TLS Request carrying change_cipher_spec then the Finished handshake record, or a rejected peer receives that flight. TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess fatals on a non-Request/non-TLS packet or a bare ACK there, errors unless record types are [change_cipher_spec, handshake], and requires equal non-zero MSKs. Negative: TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected errors on any change_cipher_spec record sent to a rejected peer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L333) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L229) | unit/verify | unproven |

### [`RFC5216-2.1.3-6`](#rfc5216-2.1.3-6)

To ensure that the EAP Server receives the TLS alert message, the peer MUST wait for the EAP Server to reply before terminating the conversation. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the peer terminating on the round it discovers the failure, before the server replies. TestRFC5216PeerRepliesBeforeItTerminates fatals if that round returns Err or no Response, errors if the Response is a bare ACK rather than the alert, and requires the error only from the round that processes the server's EAP-Failure. Negative: TestRFC5216PeerDoesNotWaitWhenItSentNothing errors if a peer that sent nothing parks its error or sends a Response.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerDoesNotWaitWhenItSentNothing`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L168) | unit/verify | unproven |
| positive | [`TestRFC5216PeerRepliesBeforeItTerminates`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_peer_wait_test.go#L41) | unit/verify | unproven |

### [`RFC5216-2.1.3-7`](#rfc5216-2.1.3-7)

If the EAP server authenticates successfully, the peer MUST send an EAP-Response packet of EAP-Type=EAP-TLS, and no data. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the peer answering the server's closing flight with anything but a no-data EAP-TLS Response. TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess errors unless bareEAPTLSResponse(peerSent[successAt-1]). Negative: TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse errors if a peer refusing the server sends the no-data Response as its last packet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L386) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L237) | unit/verify | unproven |

### [`RFC5216-2.1.3-8`](#rfc5216-2.1.3-8)

The EAP Server then MUST respond with an EAP-Success message. (§2.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the server answering the peer's no-data Response with anything but EAP-Success. TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess errors unless serverSent[successAt].Code == CodeSuccess directly after that Response. Negative: TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected errors on any Success to a rejected peer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216NoClosingFlightOrSuccessWhenThePeerIsRejected`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L339) | unit/verify | unproven |
| positive | [`TestRFC5216SuccessfulTerminationSendsFlightAckThenSuccess`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_success_flight_test.go#L242) | unit/verify | unproven |

### [`RFC5216-2.3-1`](#rfc5216-2.3-1)

Key_Material = TLS-PRF-128(master_secret, "client EAP encryption", client.random || server.random) MSK = Key_Material(0,63) EMSK = Key_Material(64,127) (§2.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. MSK = first 64 octets of the TLS 1.2 export under the literal label, control label differs; EMSK = Key_Material(64,127) is derived by exportEAPTLSKeys but no tagged unit asserts it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEAPTLSMutualAuthHandshakeSucceeds`](https://github.com/ze-software/ze/blob/main/internal/core/eap/eap_tls_handshake_test.go#L360) | unit/verify | unproven |
| positive | [`TestRFC5216MSKIsTheExportUnderTheRFCLabel`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_msk_label_test.go#L112) | unit/verify | unproven |

### [`RFC5216-2.1.1-3`](#rfc5216-2.1.1-3)

Once having received the peer's Identity, the EAP server MUST respond with an EAP-TLS/Start packet, which is an EAP-Request packet with EAP-Type=EAP-TLS, the Start (S) bit set, and no data. (§2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: after the Identity the server sends anything but a Request of Type 13 with only the S flag, or sends a Start before an Identity. TestRFC5216StartAnswersTheIdentityResponse fatals on Code != CodeRequest, Type != TypeTLS, or TypeData != {S}; TestRFC5216NoStartBeforeTheIdentityIsReceived fatals if Begin opens with an EAP-TLS packet or a stale Identity Response draws any packet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216NoStartBeforeTheIdentityIsReceived`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC5216StartAnswersTheIdentityResponse`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L70) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-4`](#rfc5216-2.1.1-4)

If the peer's sessionId is null or unrecognized by the server, the server MUST choose the sessionId to establish a new session. (§2.1.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The tagged units assert session-ticket keys and resumption-off config, which prove the ticket rule, not the sessionId choice this sentence names. Gap: crypto/tls, as newTLSMethod (internal/core/eap/eap_tls.go) configures it, sets ServerHello.sessionId only in doResumeHandshake (echoing the client), so a TLS 1.2 full handshake sends an empty session_id and the server chooses no sessionId for the new session.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L35) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-5`](#rfc5216-2.1.1-5)

If the session matches the peer's, then the ciphersuite MUST match the one negotiated during the handshake protocol execution that established the session. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a resumed session negotiating a ciphersuite other than the one that established it. No tagged assertion reads a resumed ConnectionState.CipherSuite against the original: the units assert only UnwrapSession nil with resumption on and set with it off, which decides whether resumption happens, not which suite it uses. crypto/tls enforces the match, but no unit drives a resumption or observes it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L76) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L36) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-6`](#rfc5216-2.1.1-6)

If the EAP server is not resuming a previously established session, then it MUST include a TLS server_certificate handshake message, and a server_hello_done handshake message MUST be the last handshake message encapsulated in this EAP-Request packet. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the units assert one installed certificate and no config without a parsable certificate; the second clause, server_hello_done last in that EAP-Request, is not asserted by any tagged unit

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L37) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-7`](#rfc5216-2.1.1-7)

The certificate message contains a public key certificate chain for either a key exchange public key (such as an RSA or Diffie-Hellman key exchange public key) or a signature public key (such as an RSA or Digital Signature Standard (DSS) signature public key). In the latter case, a TLS server_key_exchange handshake message MUST also be included to allow the key exchange to take place. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.1.1-7, so no unit is bound to it.

### [`RFC5216-2.1.1-8`](#rfc5216-2.1.1-8)

If the peer supports EAP-TLS and is configured to use it, it MUST respond to the EAP-Request with an EAP-Response packet of EAP- Type=EAP-TLS. (§2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a peer configured for EAP-TLS answering the EAP-TLS Request with anything but an EAP-TLS Response (a Nak, another type). TestRFC5216ConfiguredPeerAnswersEAPTLSWithEAPTLS fatals on Code != CodeResponse or Type != TypeTLS and on TypeData without a handshake record. Negative for the condition: TestRFC5216PeerNotConfiguredForEAPTLSDoesNotAnswerWithEAPTLS fatals if an MS-CHAPv2 peer answers with Type 13 and wantLegacyNak requires the Nak.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerNotConfiguredForEAPTLSDoesNotAnswerWithEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC5216ConfiguredPeerAnswersEAPTLSWithEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L132) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-9`](#rfc5216-2.1.1-9)

If the preceding server_hello message sent by the EAP server in the preceding EAP-Request packet did not indicate the resumption of a previous session, the data field of this packet MUST encapsulate one or more TLS records containing a TLS client_key_exchange, change_cipher_spec, and finished messages. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive asserts ClientAuth=RequireAndVerify and one peer certificate, which is the 2.1.1-1 certificate clause, not client_key_exchange, change_cipher_spec and finished in the peer flight

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L78) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L38) | unit/verify | revert, verified |

### [`RFC5216-2.1.1-10`](#rfc5216-2.1.1-10)

If the preceding server_hello message sent by the EAP server in the preceding EAP-Request packet indicated the resumption of a previous session, then the peer MUST send only the change_cipher_spec and finished handshake messages. (§2.1.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a peer answering a resuming server_hello with more than change_cipher_spec and finished (certificate, client_key_exchange, certificate_verify). The units assert only that the peer installs a ClientSessionCache with resumption on and none with it off; no resumed conversation is driven and no peer flight is decoded, so a peer sending a full flight on resumption passes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L79) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L39) | unit/verify | revert, verified |

### [`RFC5216-2.1.2-1`](#rfc5216-2.1.2-1)

If the EAP server is resuming a previously established session, then it MUST include only a TLS change_cipher_spec message and a TLS finished handshake message after the server_hello message. (§2.1.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a resuming server sending certificate, server_key_exchange or certificate_request after server_hello. The units assert only ticket installation (SessionTicketsDisabled false, UnwrapSession nil) and the refusal when resumption is off; no resumed flight is driven or its handshake types read, so extra messages after server_hello pass.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216TLSConfigRefusesWhatWouldBreakTheFlights`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_tls_config_test.go#L40) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-2`](#rfc5216-2.1.5-2)

When an EAP-TLS peer receives an EAP-Request packet with the M bit set, it MUST respond with an EAP-Response with EAP-Type=EAP-TLS and no data. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the peer answering an M-set Request with data or with anything but an EAP-TLS Response. TestRFC5216PeerAcksEachServerFragmentWithNoData errors unless every M-set Request is answered by bareEAPTLSResponse, and errors if the last fragment (M clear) draws the no-data ACK instead of the TLS flight; driveFragmentedFlight fatals unless both sides fragment and the MSKs match.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerAcksEachServerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L205) | unit/verify | revert, verified |
| positive | [`TestRFC5216PeerAcksEachServerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L199) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-3`](#rfc5216-2.1.5-3)

The EAP server MUST wait until it receives the EAP-Response before sending another fragment. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the server sending another fragment before the ACK for the current one arrives. driveFragmentedFlight feeds a Response carrying the previous Identifier after every server fragment and fatals if it draws any packet or moves outOffset; TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck fatals unless the packet after each fragment is the continuation Request carrying data with neither S nor L.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L247) | unit/verify | revert, verified |
| positive | [`TestRFC5216ServerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L241) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-4`](#rfc5216-2.1.5-4)

In order to prevent errors in processing of fragments, the EAP server MUST increment the Identifier field for each fragment contained within an EAP-Request, and the peer MUST include this Identifier value in the fragment ACK contained within the EAP-Response. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 forbidden: a fragment Request whose Identifier is not the previous plus one; TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem errors on req.Identifier != prev+1. Clause 2 forbidden: an ACK carrying another Identifier; the same test errors on ack.Identifier != req.Identifier, and driveFragmentedFlight fatals if a stale-Identifier ACK draws a packet.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L279) | unit/verify | revert, verified |
| positive | [`TestRFC5216ServerFragmentIdentifiersIncrementAndTheAckEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L273) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-5`](#rfc5216-2.1.5-5)

Similarly, when the EAP server receives an EAP-Response with the M bit set, it MUST respond with an EAP-Request with EAP-Type=EAP-TLS and no data. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the server answering an M-set Response with data or with a non-EAP-TLS Request. TestRFC5216ServerAcksEachPeerFragmentWithNoData errors unless each M-set Response is followed by a Request of Type 13 with TypeData exactly 0x00, and errors if the peer's last fragment is answered with an ACK instead of the closing flight.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerAcksEachPeerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L312) | unit/verify | revert, verified |
| positive | [`TestRFC5216ServerAcksEachPeerFragmentWithNoData`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L306) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-6`](#rfc5216-2.1.5-6)

The EAP peer MUST wait until it receives the EAP-Request before sending another fragment. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the peer sending its next fragment before an EAP-Request arrives. driveFragmentedFlight feeds the peer an undefined-Code packet after each of its fragments and fatals if probe.Response != nil or peer.outOffset moves; TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck fatals unless the packet after each fragment is a continuation Response carrying TLS data without L.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L355) | unit/verify | revert, verified |
| positive | [`TestRFC5216PeerSendsTheNextFragmentOnlyInAnswerToTheAck`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L349) | unit/verify | revert, verified |

### [`RFC5216-2.1.5-7`](#rfc5216-2.1.5-7)

In order to prevent errors in the processing of fragments, the EAP server MUST increment the Identifier value for each fragment ACK contained within an EAP-Request, and the peer MUST include this Identifier value in the subsequent fragment contained within an EAP- Response. (§2.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause 1 forbidden: an ACK Identifier that is not the previous Request plus one; TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem errors on ack.Identifier != serverSent[i].Identifier+1. Clause 2 forbidden: the peer fragment after an ACK carries another Identifier; the same test errors on next.Identifier != ack.Identifier, and driveFragmentedFlight fatals if the authenticator accepts a fragment re-numbered with a stale Identifier.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L388) | unit/verify | revert, verified |
| positive | [`TestRFC5216ServerAckIdentifiersIncrementAndTheNextFragmentEchoesThem`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_fragment_ack_test.go#L382) | unit/verify | revert, verified |

### [`RFC5216-2.4-7`](#rfc5216-2.4-7)

EAP-TLS implementations MUST support TLS v1.0. (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-2.4-7, so no unit is bound to it.

### [`RFC5216-3-3`](#rfc5216-3-3)

The Identifier field MUST be changed on each Request packet. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Request that repeats the Identifier of the packet before it. TestRFC5216IdentifierChangesOnEveryRequest errors on cur.Identifier == prev.Identifier for every Request of a full TLS 1.2 conversation and fatals if fewer than 3 Requests ran; the negative pins that EAP-Success (not a Request) repeats the answered Identifier.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216IdentifierChangesOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L190) | unit/verify | revert, verified |
| positive | [`TestRFC5216IdentifierChangesOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L186) | unit/verify | revert, verified |

### [`RFC5216-3-4`](#rfc5216-3-4)

Octets outside the range of the Length field should be treated as Data Link Layer padding and MUST be ignored on reception. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: octets past the Length field read as packet data. TestRFC5216PaddingPastTheLengthIsIgnored decodes a packet with seven padding octets shaped like flags and a TLS record header and errors if TypeData differs from the unpadded TypeData, if its length exceeds Length less 5, or if the padding bytes appear inside TypeData; the unpadded decode is the positive.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216PaddingPastTheLengthIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L232) | unit/verify | revert, verified |
| positive | [`TestRFC5216PaddingPastTheLengthIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L227) | unit/verify | revert, verified |

### [`RFC5216-3-5`](#rfc5216-3-5)

Implementations of this specification MUST set the reserved bits to zero, and MUST ignore them on reception. (§3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Reception clause enforced: TestRFC5216ReservedFlagBitsAreIgnoredOnReception sets all five reserved bits on a Start (peer fatals unless it answers with a client_hello) and on a whole client_hello (authenticator fatals on a bare ACK, i.e. a reserved bit read as M, or on a non-handshake answer). The quoted sentence also carries "MUST set the reserved bits to zero", and no unit tagged RFC5216-3-5 asserts the flags octet a sender emits has bits 3-7 clear; that clause is RFC5216-3-2 and its proof is not tagged here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ReservedFlagBitsAreIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L276) | unit/verify | revert, verified |
| positive | [`TestRFC5216ReservedFlagBitsAreIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L271) | unit/verify | revert, verified |

### [`RFC5216-3-6`](#rfc5216-3-6)

The Identifier field is one octet and MUST match the Identifier field from the corresponding request. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Response whose Identifier differs from the Request it answers. TestRFC5216ResponseIdentifierMatchesTheRequest errors on res.Identifier != serverSent[i].Identifier over a full conversation; TestRFC5216ResponseIdentifierIsCopiedNotCounted fatals if Requests 0x80 then 0x05 are not answered 0x80 then 0x05, which refuses a peer that counts its own Identifiers.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5216ResponseIdentifierIsCopiedNotCounted`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L373) | unit/verify | revert, verified |
| positive | [`TestRFC5216ResponseIdentifierMatchesTheRequest`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc5216_flight_rules_test.go#L348) | unit/verify | revert, verified |

### [`RFC5216-5.2-1`](#rfc5216-5.2-1)

Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-1, so no unit is bound to it.

### [`RFC5216-5.2-2`](#rfc5216-5.2-2)

If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-2, so no unit is bound to it.

### [`RFC5216-5.2-3`](#rfc5216-5.2-3)

Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-3, so no unit is bound to it.

### [`RFC5216-5.2-4`](#rfc5216-5.2-4)

The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-4, so no unit is bound to it.

### [`RFC5216-5.2-5`](#rfc5216-5.2-5)

Where it is non-empty, the subject name field MUST contain an X.500 distinguished name (DN). (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.2-5, so no unit is bound to it.

### [`RFC5216-5.3-2`](#rfc5216-5.3-2)

Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5216-5.3-2, so no unit is bound to it.

### [`RFC5216-5.3-3`](#rfc5216-5.3-3)

When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]. (§5.3)

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
