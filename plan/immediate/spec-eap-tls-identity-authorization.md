# Spec: eap-tls-identity-authorization

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

Both EAP-TLS roles verify the peer's CHAIN and its revocation status, and neither
authorizes the IDENTITY the certificate carries. RFC 5216 Section 5.2 defines that
identity (Peer-Id and Server-Id, taken from the subjectAltName when present) and
Section 5.3 obliges each side to validate that it is appropriate and authorized for
EAP-TLS. An authenticator today accepts any client certificate its CA issued, and a
peer accepts any server certificate its trust anchor issued: an operator on the first
release meets this as a wrong answer, which is why the spec sits in `plan/immediate/`.

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC5216-5.2-1 | "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName." (Section 5.2) | `internal/core/eap/eap_tls.go::eapTLSPeerName` returns `PeerCertificates[0].Subject.String()` and never reads the subjectAltName; no Server-Id is exported on the peer side. `internal/core/eap/nai.go::certificateNAIs` reads the SAN for the RFC 9190 NAI only |
| RFC5216-5.3-2 | "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS." (Section 5.3) | Authenticator: `eap_tls.go::newTLSMethod` `VerifyConnection` checks chain and revocation (`revocation.go::checkChainRevocation`) only. Peer: `peer_chain.go::verifyPeerCertificate` checks chain and revocation. Neither authorizes a Peer-Id or Server-Id against configuration |
| RFC5216-5.3-3 | "When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]." (Section 5.3) | `peer.go::tlsClientConfig` sets no `ServerName`, so no RFC 2818 Section 3.1 comparison is performed; the feature is an expected server identity leaf on the peer compared with the RFC 2818 rules |

The design derives Peer-Id and Server-Id from the SAN when present and the subject
otherwise, exports both, carries them to the IKE carrier's log and authorization, and
adds an operator allow-list or realm check on each side.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ipsec.md` - "EAP-TLS with TLS 1.3", the identity the carrier logs today

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - Sections 5.2 and 5.3
- [ ] `rfc/short/rfc9190.md` - Section 2.2, the NAI the peer's certificate carries

## Current Behavior (MANDATORY)

- [ ] `internal/core/eap/eap_tls.go` - `eapTLSPeerName` reads the subject only
- [ ] `internal/core/eap/peer_chain.go` - `verifyPeerCertificate` checks the chain only

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
| an EAP-TLS exchange whose certificate carries a SAN | → | `eapTLSPeerName` and the authorization check on each role | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | RFC5216-5.2-1, RFC5216-5.3-2 and RFC5216-5.3-3 each carry a tagged positive and negative test | `./le rfc check` prints no line for them |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` | `internal/core/eap/rfc5216_identity_test.go` | the Peer-Id and Server-Id come from the SAN when present, and an identity outside the operator's list is refused on each role | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/core/eap/eap_tls.go` - `eapTLSPeerName` reads the SAN first
- `internal/core/eap/peer_chain.go` - the Server-Id export and check
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

`rfc/short/rfc5216.md` rows RFC5216-5.2-1, RFC5216-5.3-2 and RFC5216-5.3-3.

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
