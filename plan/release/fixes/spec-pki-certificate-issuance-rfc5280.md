# Spec: pki-certificate-issuance-rfc5280

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Ze issues certificates (the self-signed web certificate, the local CA leaf for the appliance and the web server) that break RFC 5280 MUSTs on the issuer side; vpn-endpoint reaches RFC 5280, and a non-conformant certificate is refused by strict validators |
| RFC gaps | RFC5280-4.1.2.4-1, RFC5280-4.2.1.6-8, RFC5280-4.2.1.6-9, RFC5280-4.2.1.6-11 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the RFC 5280 gaps the 2026-10-26 summary walk recorded for certificates
Ze ISSUES. Each is a defect in a shipped capability.

| Row | Producer | What is missing |
|-----|----------|-----------------|
| RFC5280-4.1.2.4-1 | `internal/core/selfcert/selfcert.go::GenerateWebCertWithNames` | The template sets no Subject and the certificate is self-signed, so the issuer field is an empty DN. The pki CA names its issuer `CN=Ze Local CA` and is not affected |
| RFC5280-4.2.1.6-8, -9, -11 | `internal/component/pki/ca.go::Root.IssueLeafFor`, `GenerateWebCertWithNames` | Every host that does not parse as an IP becomes a dNSName SAN unchecked. `x509.CreateCertificate` checks only that it is an IA5String. Names reach it from the appliance `cert-name` (`internal/appliance/ca.go`, prompted in `cmd_init.go`), `--web-cert-name` (`internal/plugins/init/main.go`) and the DNS server's self-signed SANs (`internal/core/dnsserver/tlsmaterial.go`) |

RFC 5280 text, `rfc/full/rfc5280.txt`:

- Section 4.1.2.4: "The issuer field MUST contain a non-empty distinguished name (DN)."
- Section 4.2.1.6: "The name MUST be in the "preferred name syntax", as specified by Section 3.5 of [RFC1034] and as modified by Section 2.1 of [RFC1123]."; "In addition, while the string " " is a legal domain name, subjectAltName extensions with a dNSName of " " MUST NOT be used."; "Unlike the subject field, conforming CAs MUST NOT issue certificates with subjectAltNames containing empty GeneralName fields."

The SAN check belongs at the producer every issuer shares, and the config
leaves that feed it validate at commit time, so a bad name is refused where the
operator typed it, with the value named.

## Required Reading

- `rfc/short/rfc5280.md`, `rfc/full/rfc5280.txt` Sections 4.1.2.4 and 4.2.1.6
- `rfc/full/rfc1034.txt` Section 3.5 and `rfc/full/rfc1123.txt` Section 2.1 (fetch if absent)
- The PKI and web TLS pages named for these files in `ai/CODE-TO-DOCS.md`

## Current Behavior (MANDATORY)

**Source files read:** (2026-10-26; design re-reads each before any decision)
- [ ] `internal/core/selfcert/selfcert.go` - `GenerateWebCertWithNames` template with no Subject; `WebCertHosts` appends extra names unchecked
- [ ] `internal/component/pki/ca.go` - `Root.IssueLeafFor` puts each non-IP host into `DNSNames`
- [ ] `internal/appliance/ca.go` - passes `cfg.TLS.CertName` as an extra name

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Appliance config `cert-name`, `ze init --web-cert-name`, DNS server TLS setup

### Transformation Path
1. Host list built by `WebCertHosts`
2. Template built by `IssueLeafFor` or `GenerateWebCertWithNames`
3. `x509.CreateCertificate`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator input to issuer | config leaf or flag string | No |

### Integration Points
- Every caller of `IssueLeafFor` and `GenerateWebCertWithNames`

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stored appliance config already holds a name the new check refuses | Boot refuses the appliance config | Refuse with the value named and the command that fixes it (`ai/rules/user-facing-errors.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init --web-cert-name " "` | → | DNS SAN validation | functional test to write under `test/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5280Section4124SelfSignedIssuerNonEmpty` | `internal/core/selfcert/` | AC-1 | |
| `TestRFC5280Section4216DNSNameSyntax` | `internal/component/pki/` | AC-2 | |

Each tagged unit carries a discrimination record. Scoped evidence:
`./le test unit` over the changed packages.

## Files to Modify
- `internal/core/selfcert/selfcert.go` - subject DN and SAN validation
- `internal/component/pki/ca.go` - SAN validation
- `rfc/short/rfc5280.md` - remove the four `{gap}` annotations

## Implementation Steps

1. Shared DNS SAN validator, used by both issuers and by the config leaves
2. Non-empty subject DN on the self-signed certificate

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `library-default-left-in-place.md` | 2026-10-26 | `GenerateWebCertWithNames` self-signed issuer is an empty DN |
| `library-default-left-in-place.md` | 2026-10-26 | `IssueLeafFor` and selfcert issue unchecked DNS SANs |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | Every self-signed certificate Ze generates carries a non-empty subject and issuer DN |
| AC-2 | An empty, space-only or syntactically invalid DNS name is refused before issuance, with the value named; valid names and IP SANs are unchanged |
| AC-3 | Each row is implemented, its `{gap}` removed, with positive and negative tagged tests and discrimination records |
