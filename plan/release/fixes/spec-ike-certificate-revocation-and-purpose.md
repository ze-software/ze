# Spec: ike-certificate-revocation-and-purpose

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P0 |
| Release reason | IKE certificate authentication is a security boundary of the vpn-endpoint release profile. A revoked certificate, or one issued for another purpose, authenticates an IKE SA today, and EAP-TLS revocation trusts a CRL it cannot fully read: each is a fail-open boundary |
| RFC gaps | RFC5280-6.1.3-1, RFC5280-4.2.1.12-2, RFC5280-5.2-1, RFC5280-5.2.5-1, RFC5280-5.3-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the RFC 5280 path-validation gaps that the 2026-10-26 summary walk
recorded for certificates Ze VERIFIES. Each is a defect in a capability Ze
ships (IKEv2 certificate authentication, EAP-TLS revocation), so each is owed.

| Row | Producer | What is missing |
|-----|----------|-----------------|
| RFC5280-6.1.3-1 | `internal/component/ike/engine/cert_payload.go::getRemoteCert` | `x509.Certificate.Verify` checks signature, validity and name chaining; nothing checks condition (a)(3), revocation status, of an IKE peer chain. EAP-TLS runs `internal/core/eap/revocation.go::checkChainRevocation`, IKE runs nothing |
| RFC5280-4.2.1.12-2 | `getRemoteCert` | `KeyUsages: ExtKeyUsageAny` disables the EKU check entirely, and the leaf's key usage bits are never read, so a certificate whose extensions name no purpose IKE fills authenticates |
| RFC5280-5.2-1, RFC5280-5.2.5-1, RFC5280-5.3-1 | `internal/core/eap/revocation.go::crlSet.currentListFrom` | Any CRL `x509.ParseRevocationList` accepts decides status. Go records CRL and entry extensions without judging their critical flag (`crypto/x509/parser.go` `ParseRevocationList` handles AuthorityKeyId, CRLNumber and ReasonCode only), and Ze does not judge it either, so a delta CRL or an issuing-distribution-point CRL is read as complete |

RFC 5280 text, `rfc/full/rfc5280.txt`:

- Section 6.1.3: "The certificate MUST satisfy each of the following:" with (a)(3) "At the current time, the certificate is not revoked."
- Section 4.2.1.12: "If a certificate contains both a key usage extension and an extended key usage extension, then both extensions MUST be processed independently and the certificate MUST only be used for a purpose consistent with both extensions."
- Section 5.2: "If a CRL contains a critical extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of certificates."
- Section 5.3: "If a CRL contains a critical CRL entry extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of any certificates."

RFC5280-4.2.1.12-3 is a MAY on the issuer and is not a defect. The comment in
`getRemoteCert` records why `ExtKeyUsageAny` was chosen (gen-pki.sh leaves no
EKU; strongSwan's `--flag ike` writes id-kp-ipsecIKE): the fix keeps those
certificates working while refusing a leaf whose extensions exclude IKE.

Design must read strongSwan (`libstrongswan/credentials/credential_manager.c`,
the revocation and `x509_flag_t` checks) and RFC 4945 Sections 5.1.3.2 and
5.1.3.12 before choosing the key usage and EKU rule, and decide how an IKE peer
configures its CRL source (the EAP-TLS `crl` leaf in
`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` is the existing shape).
The CRL fix lands first because the IKE revocation check reuses that code.

`internal/component/pki/tls.go::verifyEntryChain` and `pki/store.go` also pass
`ExtKeyUsageAny`; they validate a certificate Ze SERVES, not a peer, and the
design states whether the same rule applies there.

## Required Reading

- `rfc/short/rfc5280.md`, `rfc/full/rfc5280.txt` Sections 4.2.1.3, 4.2.1.12, 5.2, 5.2.5, 5.3, 6.1.3
- `rfc/short/rfc7296.md` Section 3.6 and `rfc/full/rfc4945.txt` Section 5 (not in the tree on 2026-10-26: fetch it first)
- `docs/architecture/ike/` pages naming certificate authentication, and the EAP-TLS revocation page

## Current Behavior (MANDATORY)

**Source files read:** (2026-10-26; design re-reads each before any decision)
- [ ] `internal/component/ike/engine/cert_payload.go` - `getRemoteCert`: `cert.Verify` with the CA pool, peer intermediates and `ExtKeyUsageAny`, then the remote-id check; no revocation, no key usage
- [ ] `internal/core/eap/revocation.go` - `checkChainRevocation` and `crlSet.currentListFrom`: signature and freshness of each CRL, no critical-extension judgement
- [ ] `internal/component/pki/tls.go` - `verifyEntryChain`, same `ExtKeyUsageAny` on a served chain

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE_AUTH request or response carrying CERT payloads; EAP-TLS Certificate message

### Transformation Path
1. IKE message decode, CERT payloads into `sa.RemoteCertChainRaw`
2. `getRemoteCert` path validation, then AUTH payload verification
3. EAP-TLS `VerifyConnection` into `checkChainRevocation`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to IKE engine | CA name and CRL source per peer | No |

### Integration Points
- `pki.GetCA` for the anchor; the EAP-TLS CRL set for revocation

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A purpose rule too strict refuses certificates operators already use (gen-pki.sh, strongSwan `--flag ike`) | The strongSwan interop suite goes red | Admit absent EKU, anyExtendedKeyUsage and id-kp-ipsecIKE, per RFC 4945 Section 5.1.3.12 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH with a revoked peer certificate | → | `getRemoteCert` revocation check | `test/ipsec/ipsec-cert-revoked.ci` (to write) |
| IKE_AUTH with a leaf whose EKU excludes IKE | → | `getRemoteCert` purpose check | `test/ipsec/ipsec-cert-wrong-purpose.ci` (to write) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5280Section613RevokedIKEPeerRefused` | `internal/component/ike/engine/` | AC-1 | |
| `TestRFC5280Section42112IKEPurpose` | `internal/component/ike/engine/` | AC-2 | |
| `TestRFC5280Section52UnprocessedCriticalCRL` | `internal/core/eap/` | AC-3 | |

Each tagged unit carries a discrimination record (`./le rfc record`). Scoped
evidence: `./le test unit` over the changed packages and the named `.ci` tests.

## Files to Modify
- `internal/component/ike/engine/cert_payload.go` - revocation and purpose checks
- `internal/core/eap/revocation.go` - critical-extension refusal
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - per-peer CRL source, if design adds one
- `rfc/short/rfc5280.md` - remove the five `{gap}` annotations

## Implementation Steps

1. CRL critical-extension refusal (AC-3), since IKE revocation reuses it
2. IKE revocation check (AC-1)
3. IKE purpose check (AC-2)

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-26 | `getRemoteCert` checks no revocation, EAP-TLS does |
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-10-26 | `getRemoteCert` `ExtKeyUsageAny` disables every purpose check |
| `library-default-left-in-place.md` | 2026-10-26 | `crlSet.currentListFrom` uses a CRL with an unprocessed critical extension |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | An IKE peer whose certificate, or an intermediate, is on the configured CRL fails IKE_AUTH with AUTHENTICATION_FAILED, and the log names the certificate and the CRL |
| AC-2 | An IKE peer leaf whose key usage or EKU excludes IKE fails IKE_AUTH; a leaf with no EKU, anyExtendedKeyUsage or id-kp-ipsecIKE still authenticates |
| AC-3 | A CRL carrying a critical CRL or entry extension Ze cannot process is not used: status is reported unknown and the configured unknown-status action applies, for EAP-TLS and IKE alike |
| AC-4 | Each row is implemented, its `{gap}` removed from `rfc/short/rfc5280.md`, and it carries positive and negative tagged tests with discrimination records |
