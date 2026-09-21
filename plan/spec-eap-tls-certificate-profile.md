# Spec: eap-tls-certificate-profile

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 5216 Section 5.2 profiles the NAMING of an EAP-TLS certificate. Two of its
sentences bind whoever ISSUES a certificate and one binds whoever generates one with a
Network Access Identifier. Ze's own issuers (`internal/component/pki/ca.go` and
`internal/core/selfcert/selfcert.go`) always write a non-empty subject CN, take no NAI
and write no emailAddress RDN, so today Ze never produces a certificate that triggers
either condition; and Ze's verifiers (`internal/core/eap/peer_chain.go::verifyPeerCertificate`,
`internal/core/eap/eap_tls.go::newTLSMethod` `VerifyConnection`) do not check either
condition on a certificate somebody else issued. The feature is the profile enforced on
BOTH sides: the issuers write it and the verifiers refuse a leaf that breaks it. Each
row is a requirement until the owner declines the feature (owner ruling, 2026-09-21).

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC5216-5.2-2 | "If subject naming information is present only in the subjectAltName extension of a peer or server certificate, then the subject field MUST be an empty sequence and the subjectAltName extension MUST be critical." (Section 5.2) | Issuers never write an empty subject; verifiers do not refuse an empty-subject leaf whose SAN is non-critical |
| RFC5216-5.2-3 | "Conforming implementations generating new certificates with Network Access Identifiers (NAIs) MUST use the rfc822Name in the subject alternative name field to describe such identities." (Section 5.2) | No Ze issuer takes an NAI; the feature is an NAI parameter on the EAP-TLS client certificate issue path that writes it as an rfc822Name SAN |
| RFC5216-5.2-4 | "The use of the subject name field to contain an emailAddress Relative Distinguished Name (RDN) is deprecated, and MUST NOT be used." (Section 5.2) | Issuers never write it; verifiers do not refuse a leaf carrying one |

RFC5216-5.2-5 ("Where it is non-empty, the subject name field MUST contain an X.500
distinguished name") is structural in Go crypto/x509 on both the issuing and the
parsing side and carries a `{lower-layer}` annotation rather than a row here.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ipsec.md` - "EAP-TLS with TLS 1.3", the certificates each role presents

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - Section 5.2

## Current Behavior (MANDATORY)

- [ ] `internal/component/pki/ca.go` - the leaf issuer writes a subject CN and no SAN naming profile
- [ ] `internal/core/eap/peer_chain.go` - `verifyPeerCertificate` checks the chain and revocation only

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Skeleton: filled at design.

### Transformation Path
Skeleton: filled at design.

### Boundaries Crossed
Skeleton: filled at design.

### Integration Points
Skeleton: filled at design.

### Architectural Verification
Skeleton: filled at design.

## Risks & Assumptions

### Assumptions
Skeleton: filled at design.

### Risks
Skeleton: filled at design.

## Blast Radius

Skeleton: filled at design.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an EAP-TLS exchange presenting a leaf outside the profile | → | the profile check on each role | `TestRFC5216CertificateProfileIsEnforced` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | RFC5216-5.2-2, RFC5216-5.2-3 and RFC5216-5.2-4 each carry a tagged positive and negative test, or the owner's decline on their rows | `./le rfc check` prints no line for them |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5216CertificateProfileIsEnforced` | `internal/core/eap/rfc5216_certificate_profile_test.go` | each role refuses a leaf outside the profile and Ze's issuer writes a leaf inside it | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/core/eap/peer_chain.go` - the profile check on the peer
- `internal/core/eap/eap_tls.go` - the profile check on the authenticator
- `internal/component/pki/ca.go` - the NAI on the issue path
- `rfc/short/rfc5216.md` - the three rows lose their `{gap}`

## Files to Create

### Integration Checklist
Skeleton: filled at design.

### Documentation Update Checklist (BLOCKING)
Skeleton: filled at design.

## Implementation Steps

Skeleton: filled at design.

### Critical Review Checklist
Skeleton: filled at design.

### Deliverables Checklist
Skeleton: filled at design.

### Security Review Checklist
Skeleton: filled at design.

### Failure Routing
Skeleton: filled at design.

## Design Insights

Skeleton: filled at design.

## Key Design Decisions

Skeleton: filled at design.

## Known Limitations

Skeleton: filled at design.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc5216.md` rows RFC5216-5.2-2, RFC5216-5.2-3 and RFC5216-5.2-4.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
