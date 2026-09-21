# Spec: eap-tls-tls10

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

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC5216-2.4-7 | "EAP-TLS implementations MUST support TLS v1.0." (RFC 5216 Section 2.4) | `internal/core/eap/eap_tls.go::newTLSMethod` and `internal/core/eap/peer.go::tlsClientConfig` both set `MinVersion: tls.VersionTLS12`, so neither role offers or accepts TLS 1.0 |

This requirement is one a LATER standard forbids, and the spec exists so the owner
decides which document governs rather than an agent. RFC 8996 (BCP 195, "Deprecating
TLS 1.0 and TLS 1.1") Section 2: "TLS 1.0 MUST NOT be used", and it updates RFC 5216
by name. RFC 9190 Section 2.1.1 updates this document's TLS version text for TLS 1.3.
Ze's ceiling is TLS 1.3 (`rfc9190_version_cap_test.go`) and its floor is TLS 1.2 on
both roles.

Two outcomes are possible and both are one word from the owner:

| Outcome | What lands |
|---|---|
| Implement | `MinVersion` becomes `tls.VersionTLS10` on both roles behind an operator leaf that defaults to off, with a tagged pair proving a TLS 1.0 exchange completes when the leaf is on and is refused when it is off |
| Decline under RFC 8996 | the row carries `{not-applicable: RFC 8996 Section 2 forbids TLS 1.0 and updates RFC 5216; the owner declined it on <date>}` and `rfc/short/rfc8996.md` is the summary that carries the obligation Ze meets instead |

Until the owner answers, the row is a `{gap}` naming this spec (owner ruling,
2026-09-21: every MUST is a requirement and nothing is declined by an agent).

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ipsec.md` - "EAP-TLS with TLS 1.2 needs RFC 7627", the current floor

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - Section 2.4
- [ ] `rfc/short/rfc9190.md` - Section 2.1.1, the version text that updates this document

## Current Behavior (MANDATORY)

- [ ] `internal/core/eap/eap_tls.go` - `newTLSMethod` sets `MinVersion: tls.VersionTLS12`
- [ ] `internal/core/eap/peer.go` - `tlsClientConfig` sets `MinVersion: tls.VersionTLS12`

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
| the EAP-TLS version leaf | → | `newTLSMethod` and `tlsClientConfig` | `TestRFC5216TLS10IsOfferedOnlyWhenEnabled` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | RFC5216-2.4-7 carries a tagged positive and negative test, or the owner's decline under RFC 8996 on its row | `./le rfc check` prints no line for it |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5216TLS10IsOfferedOnlyWhenEnabled` | `internal/core/eap/rfc5216_tls10_test.go` | a TLS 1.0 exchange completes with the leaf on and is refused with it off | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/core/eap/eap_tls.go` - the authenticator floor
- `internal/core/eap/peer.go` - the peer floor
- `rfc/short/rfc5216.md` - the row loses its `{gap}`

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

`rfc/short/rfc5216.md` row RFC5216-2.4-7.

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
