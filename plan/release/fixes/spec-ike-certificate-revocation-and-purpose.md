# Spec: ike-certificate-revocation-and-purpose

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | IKE certificate authentication is a security boundary of the vpn-endpoint release profile. A revoked certificate, or one issued for another purpose, authenticates an IKE SA today, and EAP-TLS revocation trusts a CRL it cannot fully read: each is a fail-open boundary |
| RFC gaps | RFC5280-6.1.3-1, RFC5280-4.2.1.12-2, RFC5280-5.2-1, RFC5280-5.2.5-1, RFC5280-5.3-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

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
- Section 5.2.5: "However, implementations that do not support this extension MUST either treat the status of any certificate not listed on this CRL as unknown or locate another CRL that does not contain any unrecognized critical extensions."
- Section 5.3: "If a CRL contains a critical CRL entry extension that the application cannot process, then the application MUST NOT use that CRL to determine the status of any certificates."

RFC 4945 text, `rfc/full/rfc4945.txt` (fetched 2026-11-10):

- Section 5.1.3.2: "Since we are talking about using the public key to validate a signature, if the KeyUsage extension is present, then at least one of the digitalSignature or the nonRepudiation bits in the KeyUsage extension MUST be set (both can be set as well)."
- Section 5.1.3.12: "Implementations that do support EKU MUST support the following logic for certificate validation:" followed by "If no EKU extension, continue.", "If EKU present AND contains either id-kp-ipsecIKE or anyExtendedKeyUsage, continue." and "Otherwise, reject cert."

RFC5280-4.2.1.12-3 is a MAY on the issuer and is not a defect.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - IKEv2 certificate payload handling (the `// Design:` page of `cert_payload.go`)
  → Constraint: the page owns the certificate-validation description; the revocation and purpose rules are added there in the same commit
- [ ] `docs/architecture/ike/ipsec-11-interop-eap.md` - EAP-TLS certificate revocation
  → Constraint: the critical-extension rule is described on the page that already describes `checkChainRevocation`
- [ ] `docs/architecture/pki/pki-store.md` - the configured store and its `crl` leaf-list
  → Decision: the per-CA `crl` leaf-list is the IKE revocation source; no new leaf
- [ ] `docs/guide/ipsec.md` - operator page for certificate authentication

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5280.md` rows 4.2.1.12-2, 5.2-1, 5.2.5-1, 5.3-1, 6.1.3-1
  → Constraint: each `{gap}` is removed only with a tagged positive and negative test and a discrimination record
- [ ] `rfc/full/rfc4945.txt` Sections 5.1.3.2 and 5.1.3.12
  → Decision: the key usage and EKU rule is RFC 4945's, quoted above

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-10)
- [ ] `internal/component/ike/engine/cert_payload.go` - `getRemoteCert`: parses the leaf, refuses an empty `ca-certificate`, builds a pool of the anchor and the peer-supplied intermediates, runs `cert.Verify` with `ExtKeyUsageAny` and discards the returned chains, then binds remote-id. No revocation, no key usage, no EKU judgement. Its comment claims strongSwan's `pki --issue --flag ike` writes id-kp-ipsecIKE: strongSwan's `src/pki/commands/issue.c` has no `ike` flag, and `--flag ikeIntermediate` writes 1.3.6.1.5.5.8.2.2 (`oid.txt` "iKEIntermediate"); the comment is corrected
  → Constraint: the returned chains are what the revocation check walks; every chain is checked, as EAP does
- [ ] `internal/core/eap/revocation.go` - `parseCRLs`, `checkChainRevocation` (TLS-version policy), `crlSet.checkChain` (strict per certificate, anchor excepted), `crlSet.currentListFrom` (issuer name, issuer signature, nextUpdate), `revocationEntry`. No critical-extension judgement. Messages carry an `eap-tls:` prefix and cite RFC 9190
  → Decision: the critical-extension rule lands in `currentListFrom`, which both EAP-TLS and IKE reach
- [ ] `internal/component/pki/config.go` - `crl` leaf-list parsed per CA, signature checked against that CA, DER kept in `CACertEntry.RawCRLs`
- [ ] `internal/component/pki/store.go` - `GetCA`, `CACertEntry.CRLPEM`; no lookup of a CA entry by certificate
- [ ] `internal/component/ike/engine/responder_eap.go` - EAP-TLS revocation source: the `ca-certificate` entry's CRLs plus each `intermediate-certificate` entry's
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the EAP-TLS `session-resumption` and `certificate-status-request` descriptions name the pki `crl`; no IKE-level CRL leaf exists
- [ ] `internal/component/ike/engine/remote_cert_eku_test.go` - `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage` asserts a clientAuth-only and a serverAuth-only leaf authenticate; RFC 4945 Section 5.1.3.12 says reject, so those two cases change (judged by `ze-test-judge`)
- [ ] `test/ipsec/ipsec-eap-*.ci` (six files) - two Ze daemons; the responder's IKE certificate `ze-eap-server` carries EKU serverAuth alone, which the initiator must now refuse. Their PKI is regenerated with the server certificate carrying serverAuth and id-kp-ipsecIKE (no CA key is in the tree, so the whole fixture PKI is reissued; judged by `ze-test-judge`)
- [ ] `internal/component/pki/tls.go::verifyEntryChain`, `pki/store.go` `Validate` - `ExtKeyUsageAny` over a chain Ze SERVES

**Behavior to preserve:**
- An IKE peer whose chain meets no pki ca entry holding a CRL authenticates as today (no revocation source configured), with a Warn naming the missing source
- A leaf with no EKU, anyExtendedKeyUsage or id-kp-ipsecIKE, and no key usage or one with digitalSignature or nonRepudiation, authenticates
- EAP-TLS revocation behavior other than the critical-extension rule

**Behavior to change:**
- IKE refuses a peer chain with a revoked certificate, or one whose status is unknown under a configured source
- IKE refuses a leaf whose key usage lacks digitalSignature and nonRepudiation, or whose EKU names neither id-kp-ipsecIKE nor anyExtendedKeyUsage
- A CRL with a critical CRL or entry extension answers nothing, for EAP-TLS and IKE

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| EAP-TLS CRL gate | `internal/core/eap/revocation.go` `crlSet`, `checkChain`, `currentListFrom` | Strict per-certificate CRL check over a verified chain | extends: adds the critical-extension refusal, makes messages role-neutral, and exports one entry point the IKE engine calls. No second CRL implementation |
| pki `crl` leaf-list | `internal/component/pki/config.go`, `yang/ze-pki-conf.yang` `crl` | Per-CA CRLs, signature-checked at load | extends: becomes the IKE revocation source as well; YANG description updated. No new leaf |
| `CACertEntry.CRLPEM` | `internal/component/pki/store.go` | Concatenated PEM of one entry's CRLs | extends: reused for IKE; one accessor added to find the entry holding a given certificate |
| EAP-TLS source assembly | `internal/component/ike/engine/responder_eap.go` | anchor plus named intermediates | does not apply to IKE unchanged: IKE intermediates come from CERT payloads, so IKE gathers the CRLs of every pki ca entry whose certificate is on the verified chain |
| `x509.Certificate.Verify` | `getRemoteCert` | Path building and signature, validity, name chaining | extends: its returned chains are kept and checked for revocation; `ExtKeyUsageAny` stays on Verify (chain EKU nesting is not an RFC 5280 rule) and the leaf purpose is judged after it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE_AUTH request or response carrying CERT payloads; EAP-TLS Certificate message

### Transformation Path
1. IKE message decode, CERT payloads into `sa.RemoteCertRaw` and `sa.RemoteCertChainRaw` (`storeRemoteCerts`)
2. `getRemoteCert`: Verify, then the RFC 4945 purpose check on the leaf, then revocation over every verified chain, then remote-id
3. EAP-TLS `VerifyConnection` into `checkChainRevocation`, which reaches the same `currentListFrom`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to IKE engine | pki ca entries (certificate and `crl`) read from the pki store at IKE_AUTH | functional `.ci` |
| IKE engine to EAP core | one exported revocation function in `internal/core/eap` | unit test |

### Changed-Wire Census
No wire format changes. The outcome of IKE_AUTH changes: a refused certificate yields AUTHENTICATION_FAILED, as any `getRemoteCert` error does today.

### Integration Points
- `pki.GetCA` for the anchor; a new pki lookup of the ca entry holding a certificate; the EAP CRL code for the check

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the check sits in `getRemoteCert`, the single producer of the peer identity |
| No unintended coupling (components stay isolated) | Yes | `ike/engine` already imports `internal/core/eap` and `pki` |
| No duplicated functionality (extends existing, does not recreate) | Yes | one CRL implementation, in `internal/core/eap` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | not a hot path; once per IKE_AUTH |
| Registration over hardcoding, outbound | N-A | no command, family or handler added |
| Registration over hardcoding, inbound | N-A | no list learns a new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan puts no EKU on a certificate unless `--flag` names one, and writes no id-kp-ipsecIKE | strongSwan `src/pki/commands/issue.c` flag parser, `x509_cert.c` EKU builder | lab certificates would need reissuing | openssl survey of `test/interop-ipsec/**/pki/*.pem`: strongSwan-presented certificates carry no EKU | confirmed |
| A-2 | strongSwan as relying party checks key usage per RFC 4945 5.1.3.2 and no EKU | `pubkey_authenticator.c` `is_compliant_cert`, `x509_cert.c` `X509_IKE_COMPLIANT` | Ze's own certificates could be refused by strongSwan | interop suite | confirmed by source |
| A-3 | Go keeps every CRL and entry extension with its critical flag in `RevocationList.Extensions` and `RevocationListEntry.Extensions` | `crypto/x509/parser.go` `ParseRevocationList` (go1.27.3) | the check would see nothing | unit test with a critical extension | confirmed by source |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A purpose rule too strict refuses certificates operators already use | `.ci` and interop reds | admit absent EKU, anyExtendedKeyUsage and id-kp-ipsecIKE, per RFC 4945 Section 5.1.3.12; the error names the EKU found and the fix |
| R-2 | Interop with peers whose certificate carries serverAuth (with or without iKEIntermediate) and no id-kp-ipsecIKE: strongSwan's Windows-client recipe, Let's Encrypt certificates. strongSwan (no EKU check) and libreswan via NSS `cert_ComputeCertType` ("IPSEC is allowed to use SSL client and server certs as well as email certs") accept them | Ze as relying party refuses such a peer | RFC 4945 Section 5.1.3.12 decides ("Otherwise, reject cert."); documented in `docs/guide/ipsec.md` with the fix (add id-kp-ipsecIKE or anyExtendedKeyUsage). Reported to the owner as a consequence |
| R-3 | A configured CRL past its nextUpdate refuses every IKE peer of that CA | operator report | same rule as EAP-TLS; the error says to publish a fresh list; the YANG description says so |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IKE peers using certificates fail IKE_AUTH (fail closed), or a revoked peer authenticates (fail open, today's state) |
| How is it reverted? | single commit revert |
| Who else touches this path? | EAP-TLS (`internal/core/eap`), pki store |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH with a revoked peer certificate | → | `getRemoteCert` revocation check | `test/ipsec/ipsec-cert-revoked.ci` |
| IKE_AUTH with a leaf whose EKU excludes IKE | → | `getRemoteCert` purpose check | `test/ipsec/ipsec-cert-wrong-purpose.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | IKE peer whose certificate, or an intermediate, is on a current CRL held by a pki ca entry on its verified chain | IKE_AUTH fails: as responder Ze answers AUTHENTICATION_FAILED; as initiator Ze refuses the responder's AUTH, installs no SA and drops the IKE SA, as every `getRemoteCert` refusal does today (`fsm.go` `handleAuthResponse`; RFC 7296 Section 2.21.2: "If the error occurs on the initiator, the notification MAY be returned in a separate INFORMATIONAL exchange", so sending none is permitted). The log names the certificate (subject, serial) and the CRL (issuer, CRL number) | an unrevoked certificate under the same CRL authenticates; a peer whose chain meets no pki ca entry holding a CRL authenticates with a Warn naming the missing source, as strongSwan's default `revocation = relaxed` and EAP-TLS on TLS 1.2 do |
| AC-2 | IKE peer leaf whose key usage has neither digitalSignature nor nonRepudiation, or whose EKU names neither id-kp-ipsecIKE nor anyExtendedKeyUsage | IKE_AUTH fails; the error names the extension found and the fix | a leaf with no EKU, anyExtendedKeyUsage or id-kp-ipsecIKE (beside any other purpose) and with no key usage or a signing bit still authenticates |
| AC-3 | A configured CRL carrying a critical CRL extension or a critical entry extension | the list answers nothing: status is unknown, which refuses the chain under a configured source, for EAP-TLS and IKE alike; the error names the extension OID | another current list from the same issuer without such an extension answers; non-critical unknown extensions are ignored, as Section 5.2 allows |
| AC-4 | Each row | implemented, `{gap}` removed from `rfc/short/rfc5280.md`, positive and negative tagged tests with discrimination records | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | revokes a peer certificate by adding the CA's CRL to `pki ca <name> crl` | config → pki store → IKE_AUTH → `getRemoteCert` → refused | `test/ipsec/ipsec-cert-revoked.ci`, interop `x509-crl-revoked-peer-ze-initiator` |
| 2 | peers with a strongSwan responder whose certificate EKU is serverAuth only | IKE_AUTH → `getRemoteCert` purpose check → refused | `test/ipsec/ipsec-cert-wrong-purpose.ci`, interop `x509-eku-server-auth-only-refused-ze-initiator` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5280Section613RevokedIKEPeerRefused` | `internal/component/ike/engine/rfc5280_ike_revocation_test.go` | AC-1 (leaf revoked, intermediate revoked, unrevoked passes, no source passes, stale list refuses) | |
| `TestRFC5280Section42112IKEPurpose` | `internal/component/ike/engine/rfc5280_ike_purpose_test.go` | AC-2 (KU and EKU each refusing alone, both consistent passing) | |
| `TestRFC5280Section52UnprocessedCriticalCRL` | `internal/core/eap/rfc5280_crl_critical_test.go` | AC-3 (critical CRL extension, critical IDP, critical entry extension; non-critical passes) | |
| `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage` | `internal/component/ike/engine/remote_cert_eku_test.go` | changed: clientAuth-only and serverAuth-only now refused, judged by `ze-test-judge` | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-cert-revoked.ci` | `test/ipsec/` | two daemons, responder certificate on the initiator's CA CRL: no SA, log names certificate and CRL | |
| `ipsec-cert-wrong-purpose.ci` | `test/ipsec/` | two daemons, responder certificate EKU serverAuth only: no SA | |
| six `ipsec-eap-*.ci` | `test/ipsec/` | fixture PKI reissued with the server certificate carrying serverAuth and id-kp-ipsecIKE | |

### Interop Tests (Scope: protocol)
| Scenario | Peer | Proves |
|----------|------|--------|
| `x509-crl-revoked-peer-ze-initiator` | strongSwan | Ze refuses strongSwan's revoked server.pem; no SA |
| `x509-crl-unrevoked-peer-ze-initiator` | strongSwan | same config, CRL not listing it: SA establishes |
| `x509-eku-server-auth-only-refused-ze-initiator` | strongSwan | Ze refuses a strongSwan certificate whose EKU is serverAuth only; no SA |
| `x509-eku-ipsec-ike-ze-initiator` | strongSwan | same with EKU id-kp-ipsecIKE: SA establishes |

## Files to Modify
- `internal/component/ike/engine/cert_payload.go` - revocation and purpose checks; corrected EKU comment
- `internal/core/eap/revocation.go` - critical-extension refusal, role-neutral messages, exported entry point for IKE
- `internal/component/pki/store.go` - lookup of the ca entry holding a certificate
- `internal/component/pki/yang/ze-pki-conf.yang` - `crl` description names IKE
- `internal/component/ike/engine/remote_cert_eku_test.go` - two cases flip (judged)
- `test/ipsec/ipsec-eap-*.ci` (six) - reissued fixture PKI (judged)
- `rfc/short/rfc5280.md` - remove the five `{gap}` annotations
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md`, `docs/architecture/ike/ipsec-11-interop-eap.md`, `docs/architecture/pki/pki-store.md`, `docs/guide/ipsec.md`

## Files to Create
- `internal/component/ike/engine/rfc5280_ike_revocation_test.go`
- `internal/component/ike/engine/rfc5280_ike_purpose_test.go`
- `internal/core/eap/rfc5280_crl_critical_test.go`
- `test/ipsec/ipsec-cert-revoked.ci`, `test/ipsec/ipsec-cert-wrong-purpose.ci`
- `test/interop-ipsec/scenarios/x509-crl-revoked-peer-ze-initiator/`, `x509-crl-unrevoked-peer-ze-initiator/`, `x509-eku-server-auth-only-refused-ze-initiator/`, `x509-eku-ipsec-ike-ze-initiator/`
- `rfc/full/rfc4945.txt`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf; `crl` description only |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | none |
| CLI grammar (keyword before value) | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | No | a CRL past nextUpdate is a configured-file problem; the refusal names it at IKE_AUTH. A doctor check for an expired pki CRL is a separate gap, not owed by these five rows |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | behavior of an existing feature |
| 2 | Config syntax changed? | No | description of `crl` only |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5280.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md`, `ipsec-11-interop-eap.md`, `docs/architecture/pki/pki-store.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` certificate section |

## Implementation Steps

1. **Phase: CRL critical-extension refusal (AC-3)** - `currentListFrom` skips a list carrying any critical CRL or entry extension and reports why; messages become role-neutral; an exported entry point serves IKE
2. **Phase: pki lookup and IKE revocation (AC-1)** - gather the CRLs of the pki ca entries whose certificate is on a verified chain; none: Warn and continue; some: strict check over every chain
3. **Phase: IKE purpose (AC-2)** - RFC 4945 key usage and EKU logic on the leaf; flip the two EKU test cases; reissue the six EAP `.ci` fixture PKIs
4. **Phase: functional and interop tests, docs, summary**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | every verified chain is checked; the anchor is excepted; a stale or critical-extension list is unknown, never "not revoked" |
| Correctness | KU absent is distinguished from KU present with no bits by the extension OID, not by `KeyUsage == 0` |
| Rule: user-facing-errors | each refusal names the certificate, the cause and the fix |
| Rule: rfc-compliance | quotes above the enforcing statements, byte-exact |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| five `{gap}` removed | grep of the five row ids in `rfc/short/rfc5280.md` |
| discrimination records | `rfc/discrimination/rfc5280/` files for each row |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail open | no path where an unreadable or skipped list yields "not revoked" |
| Untrusted input | the peer chooses its chain; every chain Verify returns is checked |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- strongSwan's revocation validator answers TRUE for an unknown or stale status and leaves strictness to `revocation = strict` (`revocation_validator.c` `validate`, `auth_cfg.c` `complies`); its CRL parser refuses a list with an unsupported critical extension while `x509.enforce_critical` is true, the default (`x509_crl.c` `parse`).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-26 | `getRemoteCert` checks no revocation, EAP-TLS does |
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-10-26 | `getRemoteCert` `ExtKeyUsageAny` disables every purpose check |
| `library-default-left-in-place.md` | 2026-10-26 | `crlSet.currentListFrom` uses a CRL with an unprocessed critical extension |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| IKE revocation source is the `crl` of every pki ca entry whose certificate is on the verified chain; any such list makes the check strict for the whole chain | a new per-peer `crl` leaf; only the anchor's CRLs | the pki `crl` leaf-list is the existing shape; IKE intermediates arrive in CERT payloads, so an operator adds an intermediate as a pki ca with its list; strict once configured matches EAP-TLS |
| No source configured: authenticate with a Warn | refuse every certificate peer without a CRL | RFC 5280 Section 6.3 leaves an undetermined status to the application; strongSwan's default (`relaxed`) and libreswan's (`crl-strict=no`) accept it; refusing would break every certificate peer configured today. The Warn keeps the unchecked state visible |
| Critical-extension refusal applies to any critical CRL or entry extension | process issuingDistributionPoint scope | Ze processes none of them; Section 5.2.5 permits treating the status as unknown; strongSwan refuses the same lists |
| RFC 4945 EKU rule (absent, any, id-kp-ipsecIKE) | NSS's wider set (serverAuth, clientAuth, email, iKEIntermediate) | RFC 4945 Section 5.1.3.12 is explicit ("Otherwise, reject cert."); the RFC outranks peer leniency |
| `ExtKeyUsageAny` stays on `Verify`; purpose judged on the leaf only | pass id-kp-ipsecIKE to Verify | Go cannot name ipsecIKE in `KeyUsages`, and EKU nesting through CAs is not an RFC 5280 rule; RFC 4945 states the rule for the end-entity certificate |
| `pki/tls.go::verifyEntryChain` and `pki/store.go` keep `ExtKeyUsageAny` | apply the IKE rule | they validate a certificate Ze SERVES, for every consumer; the purpose is judged by the relying party, which for IKE is the peer (strongSwan checks key usage, A-2) |

## Known Limitations

- Revocation source is configured CRLs only; fetching from CRLDistributionPoints or OCSP for IKE peers is not in these five rows

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit` over `internal/core/eap`, `internal/component/ike/engine` and `internal/component/pki`; `./le test functional ipsec` for the named `.ci` files; `./le test integration interop-ipsec` for the four named scenarios and the existing x509 and EAP scenarios
- [ ] Feature code integrated
- [ ] Integration and Documentation checklists answered
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove` the spec

## Implementation Summary

### What Was Implemented
- Code commit fbdd9a82e7 ("ike: refuse a revoked or wrong-purpose peer certificate").
- `internal/core/eap/revocation.go`: `currentListFrom` skips a list carrying any critical CRL or entry extension (`criticalExtension`) and says why when no list answers. `checkChain`'s messages are role-neutral, and `checkChainRevocation` adds the `eap-tls:` prefix and the RFC 9190 Section 5.4 reference. New export `VerifyChainsNotRevoked`.
- `internal/component/pki/store.go`: new `CAEntriesFor`, which matches an entry by subject and public key.
- `internal/component/ike/engine/remote_cert_checks.go` (new) holds the checks that `getRemoteCert` (`cert_payload.go`) runs after Verify. `checkIKEPurpose` applies RFC 4945 Sections 5.1.3.2 and 5.1.3.12. `checkPeerChainRevocation` and `chainRevocationSource` cover RFC 5280 Section 6.1.3 (a)(3): every verified chain is checked, the check is strict once a pki ca on the chain holds a list, and a Warn fires when none does.
- `rfc/short/rfc5280.md`: gaps removed for 4.2.1.12-1, 4.2.1.12-2, 5.2-1, 5.2.5-1, 5.3-1 and 6.1.3-1. Rows 4.2-2 and 6.1-1 now carry `verifyEntryChain` only. Six discrimination records cover both polarities of each row.

### Bugs Found/Fixed
- The comment in `getRemoteCert` claimed that strongSwan's `--flag ike` writes id-kp-ipsecIKE. strongSwan has no such flag (`issue.c` `parse_flag`). The comment is corrected.
- The two-daemon EAP `.ci` fixtures used a serverAuth-only IKE certificate, which RFC 4945 Section 5.1.3.12 refuses. They are reissued with id-kp-ipsecIKE, approved by the judge.

### Documentation Updates
- `docs/guide/ipsec.md`: two new sections, "What a peer certificate must allow" and "Revoking a peer certificate".
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md`: new subsection "Peer certificate validation".
- `docs/architecture/ike/ipsec-11-interop-eap.md`: the critical-extension rule, and IKE's use of the same judge.
- `docs/architecture/pki/pki-store.md`: who consumes `crl`.
- `docs/architecture/testing/interop.md`: the four scenarios and the recorded red run.
- `internal/component/pki/yang/ze-pki-conf.yang`: the `crl` description.

### Deviations from Plan
- `pki.CAEntriesFor` returns every matching entry (the plan had `CAEntryFor`, one entry), and matches by subject and key rather than by DER. Several entries can hold one CA, and the authority may reissue its certificate.
- RFC5280-4.2.1.12-1 is tagged and has its gap removed as well. Its gap text named the same producer and became false.
- AC-1 is reworded for the initiator role. That role sends no notification, which RFC 7296 Section 2.21.2 allows (MAY).
- The `.ci` pairs became four files: a positive twin for each negative.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The skeleton said strongSwan `--flag ike` writes id-kp-ipsecIKE | strongSwan writes iKEIntermediate (1.3.6.1.5.5.8.2.2) with `--flag ikeIntermediate`, and cannot write id-kp-ipsecIKE at all | strongSwan `issue.c`, `oid.txt` | The comment and the docs are corrected. Lab certificates are issued with openssl |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC5280-6.1.3-1 | Done | `remote_cert_checks.go::checkPeerChainRevocation`, called from `getRemoteCert` (`cert_payload.go`) | strict once a source exists |
| RFC5280-4.2.1.12-2 | Done | `remote_cert_checks.go::checkIKEPurpose`, called from `getRemoteCert` | 4.2.1.12-1 too |
| RFC5280-5.2-1, 5.2.5-1, 5.3-1 | Done | `revocation.go::currentListFrom`, `criticalExtension` | both EAP-TLS and IKE |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC5280Section613RevokedIKEPeerRefused`, `ipsec-cert-revoked.ci`, `ipsec-cert-crl-unrevoked.ci`, interop `x509-crl-revoked-peer-ze-initiator` / `x509-crl-unrevoked-peer-ze-initiator` | |
| AC-2 | Done | `TestRFC5280Section42112IKEPurpose`, `ipsec-cert-wrong-purpose.ci`, `ipsec-cert-purpose-ipsec-ike.ci`, interop `x509-eku-*` | |
| AC-3 | Done | `TestRFC5280Section52UnprocessedCriticalCRL` | the EAP and IKE paths share `checkChain` |
| AC-4 | Done | `rfc/short/rfc5280.md`, `rfc/discrimination/rfc5280/` (6 files) | `./le rfc check` reports no rfc5280 finding |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC5280Section613RevokedIKEPeerRefused` | Done | `internal/component/ike/engine/rfc5280_ike_revocation_test.go` | 7 subtests |
| `TestRFC5280Section42112IKEPurpose` | Done | `internal/component/ike/engine/rfc5280_ike_purpose_test.go` | includes an empty key usage case |
| `TestRFC5280Section52UnprocessedCriticalCRL` | Done | `internal/core/eap/rfc5280_crl_critical_test.go` | |
| `TestEkuPeerCertificateIsNotJudgedByTLSExtendedKeyUsage` | Changed | `remote_cert_eku_test.go` | judge approved |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cert_payload.go`, `revocation.go`, `store.go`, `ze-pki-conf.yang`, `rfc5280.md`, the four docs | Done | in fbdd9a82e7 |
| `ze-ipsec-conf.yang` | Not needed | no new leaf, as the design decided |

### Audit Summary
- **Total items:** 13
- **Done:** 12
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A revoked IKE peer certificate does not authenticate | interop + functional + unit | `x509-crl-revoked-peer-ze-initiator` PASS against strongSwan. With `checkPeerChainRevocation` reverted it FAILS ("ze did not refuse strongSwan's certificate"), and with the file restored it passes. Its twin `x509-crl-unrevoked-peer-ze-initiator` PASS. `ipsec-cert-revoked.ci` is red under the break and green after. Stress: 80 iterations, no failure. Record `RFC5280-6.1.3-1.json` |
| A certificate issued for another purpose does not authenticate | interop + functional + unit | `x509-eku-server-auth-only-refused-ze-initiator` PASS. It goes red under the `checkIKEPurpose` break and green after. `x509-eku-ipsec-ike-ze-initiator` PASS. `ipsec-cert-wrong-purpose.ci` is red under the break and green after. Records `RFC5280-4.2.1.12-1.json` and `RFC5280-4.2.1.12-2.json` |
| A CRL with an unprocessed critical extension decides nothing | unit | `TestRFC5280Section52UnprocessedCriticalCRL`. Records for 5.2-1, 5.2.5-1 and 5.3-1 (revert of `criticalExtension`) |
| Existing certificate peers keep working | interop + functional | `x509-ed25519` (both roles), `x509-legacy-rsa-signature` (both roles), `multiple-auth-ze-initiator`, `eap-tls`, `eap-tls13`, `responder-eap-tls13`, `responder-eap-tls13-revoked-client` and `responder-eap-tls12` all PASS. `ipsec-eap-*` 6/6 PASS |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every row and AC of this spec is done. The RFC5280-4.2-2 and RFC5280-6.1-1 gaps that remain bind `verifyEntryChain`, which is not one of this spec's rows | n/a |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` at closure |
| `./le spec review check` | run by `./le commit create` on the closure commit |
| Rounds | 2 |
| Reviewer lenses used | Round 1: an independent ze-read agent covered fail-open, OIDs, RFC quotes, messages, tag claims, stale comments, docs and interop checkers. Round 2: inline over the round-1 fix hunks |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | No discrimination records | `rfc/discrimination/rfc5280/` | 6 records written by `./le rfc record`, in fbdd9a82e7 |
| 2 | ISSUE | Interop and `.ci` pairs never walked red | `docs/architecture/testing/interop.md` | Red run recorded (2026-11-10) |
| 3 | ISSUE | The 6.1.3-1 positive asserted only `revocation-root` | `rfc5280_ike_revocation_test.go` | Now asserts `pki ca revocation-root` |
| 4 | ISSUE | Detecting key usage presence by OID was untested | `rfc5280_ike_purpose_test.go` | `emptyKeyUsageLeaf` case |
| 5 | ISSUE | The doc overstated what strongSwan refuses | `ipsec-11-interop-eap.md` | Narrowed, from `x509_crl.c` `parse` |
| 6 | ISSUE | The fix text was unusable for an intermediate with no entry | `revocation.go` `errNoListNamesIssuer` | Now says to add the CA as a pki ca |

Round 2 read the round-1 fix hunks against source and found 0 BLOCKER and 0 ISSUE.
- `CAEntriesFor` matching by subject and key only adds candidate lists. `currentListFrom` still requires the chain issuer's name and signature.
- The empty key usage leaf parses to `KeyUsage == 0`, and `checkIKEPurpose` refuses it through `hasCertExtension`.
- NOTEs left as they are:
  - The RFC 9190 reference comes after the fix sentence in EAP messages.
  - Row 6.1.3-1 carries no pointer to the no-source Warn. The Warn is recorded under 6.1-1.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| the four `test/ipsec/ipsec-cert-*.ci`, the three new unit test files, `rfc/full/rfc4945.txt` | yes | `ls` on 2026-11-10 printed each path |
| the four `test/interop-ipsec/scenarios/x509-{crl,eku}-*` directories | yes | `ls -d` listed all four |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | revocation runs on every chain | `getRemoteCert` calls `checkPeerChainRevocation(sa, caName, chains)`. Coordinator run: the engine package passes and the interop revoked scenario passes |
| AC-2 | purpose check runs on the leaf | `getRemoteCert` calls `checkIKEPurpose(cert)`. The interop EKU pair passes |
| AC-3 | a critical extension skips the list | `currentListFrom` calls `criticalExtension(list)`. The eap package passes |
| AC-4 | the gaps are removed and records exist | rows RFC5280-5.2-1 and RFC5280-6.1.3-1 carry no `{gap}`. Six files under `rfc/discrimination/rfc5280/` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_AUTH with a revoked peer certificate | `test/ipsec/ipsec-cert-revoked.ci` | two daemons in x509 mode, a `crl` listing serial 65539. It expects `was revoked by` and fails under the break |
| IKE_AUTH with a leaf whose EKU excludes IKE | `test/ipsec/ipsec-cert-wrong-purpose.ci` | a serverAuth-only responder certificate. It expects `extended key usage` and fails under the break |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | openssl survey of the lab certificates. strongSwan `issue.c` `parse_flag` |
| A-2 | confirmed | `pubkey_authenticator.c` `is_compliant_cert`. Interop: strongSwan accepts Ze's clientAuth leaf |
| A-3 | confirmed | `TestRFC5280Section52UnprocessedCriticalCRL` passes with critical `ExtraExtensions` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide: the purpose rule, revocation, the Warn | `remote_cert_checks.go::checkIKEPurpose`, `checkPeerChainRevocation` | yes |
| ipsec-6: matching by subject and key | `pki/store.go::CAEntriesFor` | yes |
| ipsec-11: strongSwan's critical-extension handling | strongSwan `x509_crl.c` `parse` (cases DELTA_CRL_INDICATOR; default refuses critical) | yes |
| interop.md: the red run | coordinator run on 2026-11-10 after fbdd9a82e7 | yes |
