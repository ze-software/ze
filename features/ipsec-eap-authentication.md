# IPsec EAP Authentication

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec EAP Authentication |
| Kind | protocol |
| Scope | partial |
| Scope gaps | the RFC 5216 3DES cipher-suite MUST is not met, RFC 3748 is Partial in the ledger with open defects in spec-ike-eap-rfc-defects |
| Level | experimental |
| Components | internal/core/eap, internal/component/ike/engine/eap_auth.go |
| Real-path tests | test/ipsec/ipsec-eap-md5-challenge.ci, test/ipsec/ipsec-eap-nak-unacceptable-type.ci, test/ipsec/ipsec-eap-tls13-ocsp-required.ci, test/ipsec/ipsec-eap-tls13-ocsp-stapling.ci, test/ipsec/ipsec-eap-tls-clienthello.ci |
| Interop | ipsec/eap-mschapv2, ipsec/eap-tls, ipsec/eap-tls13, ipsec/eap-nak-method-negotiation, ipsec/responder-eap-mschapv2, ipsec/responder-eap-tls13, ipsec/responder-eap-tls13-revoked-client |
| RFCs | rfc3748, rfc5216, rfc9190 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: every source anchor resolves (internal/core/eap/, nai.go anonymousNAI, eap_auth.go eapMethodType and warnKeylessEAPModes); MD5-Challenge, Nak and OCSP stapling each have a listed test |
| Defect review | 2026-10-07: open: spec-ike-eap-rfc-defects, spec-eap-tls-authenticator-empty-message-guard, spec-eap-tls-certificate-revocation, spec-rfc-verdict-fix-ike-eap; journal counter-counts-the-wrong-packets.md row; journal rows naming a Component, not each re-verified here: concurrent-session-corruption.md:24, counter-counts-the-wrong-packets.md:11, diagnosis-parked-until-a-round-the-peer-may-never-send.md:15, documentation-shows-config-the-parser-refuses.md:16, documentation-shows-config-the-parser-refuses.md:31, field-carries-two-meanings.md:17, gate-excludes-part-of-its-population.md:40, gate-fires-outside-its-population.md:48, green-that-could-not-have-been-red.md:234, guard-blocks-its-own-authors-repair.md:5, guard-blocks-its-own-authors-repair.md:8, guard-blocks-its-own-authors-repair.md:10, invariant-enforced-by-an-absent-call-site.md:3, shared-leniency-hides-the-defect.md:3, test-against-broken-path.md:37, test-against-broken-path.md:72, validated-value-discarded-by-its-caller.md:19 |
| Extra criteria | supported: certificate revocation refused against strongSwan = ipsec/responder-eap-tls13-revoked-client; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

EAP-MSCHAPv2 (type 26), EAP-TLS (type 13) and EAP MD5-Challenge (type 4) authentication for road warrior VPN clients inside IKEv2 IKE_AUTH exchange. `authentication { mode eap-md5 }` selects type 4, and it is never a default. Its AUTH payloads come from SK_pi and SK_pr, because the method derives no MSK. Adopting it writes one warning quoting the RFC 7296 Section 2.16 sentence against a method that establishes no shared key. MS-CHAPv2 crypto (NtPasswordHash, ChallengeResponse, MPPE key derivation), TLS handshake in EAP with fragmentation, MSK derivation feeding IKEv2 AUTH payload. Virtual IP pool with dual-stack allocation (IPv4 + IPv6), DNS push via Configuration Payload. As the EAP peer, Ze negotiates the method instead of failing: a Request for an authentication type it does not run draws an RFC 3748 Section 5.3.1 legacy Nak (type 3) naming the configured method, and a Notification Request (type 2) draws a Notification Response with its message in the daemon log. On EAP-TLS the peer sends an anonymous Network Access Identifier rather than its `local-id`, because RFC 9190 Section 2.1.8 forbids a permanent identifier in the cleartext Identity Response: the realm is kept and the username omitted (`@realm`), and an identity carrying no realm the RFC 7542 Section 2.2 grammar accepts becomes the fixed `anonymous`. On EAP-TLS 1.3 the authenticator answers a Certificate Status Request with the OCSP response configured on the certificate it presents (`pki certificate <name> ocsp-response`), which RFC 9190 Section 5.4 makes mandatory; the peer refuses a chain whose stapled status is missing, expired, about another certificate or not good once `certificate-status-request` is set, and it re-checks the authenticator's certificate against that certificate's own OCSP responder over https as soon as the Child SA gives it a network, closing the SA when the responder reports a revocation. <!-- source: internal/core/eap/ -- EAP framework, MSCHAPv2, TLS, MD5-Challenge, pool --> <!-- source: internal/core/eap/nai.go -- anonymousNAI --> <!-- source: internal/component/ike/engine/eap_auth.go -- eapMethodType, warnKeylessEAPModes -->
