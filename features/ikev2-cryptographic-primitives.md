# IKEv2 Cryptographic Primitives

## Meta

| Field | Value |
|-------|-------|
| Name | IKEv2 Cryptographic Primitives |
| Kind | library |
| Scope | complete |
| Level | experimental |
| Components | internal/component/ike/crypto |
| Real-path tests | test/ipsec/ipsec-sa-installed.ci |
| Interop | ipsec/ike-aes-ccm16, ipsec/psk-site-to-site |
| RFCs | rfc7296 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: checked DH groups 14, 19 and 20 (DH_MODP_2048, DH_ECP_256, DH_ECP_384 in internal/component/ike/crypto/transform.go) and the AES-CCM and GCM transform ids there |
| Defect review | 2026-10-07: audit found no open immediate spec against the primitives; the ESN-in-IKE_SA question is held in an open spec not yet confirmed against a peer; journal rows naming a Component, not each re-verified here: feature-test-missing-build-tag.md:4, silent-fall-through.md:58 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

DH groups 14 (MODP 2048), 19 (ECP 256) and 20 (ECP 384), PRF (SHA-256/384/512), integrity (HMAC-SHA-256/384/512), encryption (AES-CBC, AES-GCM-16 128/256, and AES-CCM 8/12/16 for the IKE SA only), SKEYSEED derivation, key expansion (RFC 7296 Section 2.14). <!-- source: internal/component/ike/crypto/ -- cryptographic primitives -->
