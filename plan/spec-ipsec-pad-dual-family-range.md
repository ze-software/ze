# Spec: ipsec-pad-dual-family-range

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC4301-4.4.3.3-1 | "(A peer may be authorized for both address types, so there MUST be provision for both a v4 and a v6 address range.)" (Section 4.4.3.3) | A peer's PAD entry is ONE `remote-id` string (`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` leaf `remote-id`, read at `ipsec/config.go` into `AuthConfig.RemoteID`, matched by `internal/component/ike/engine/remote_id.go::addressEntryMatches`, which parses one address or one prefix). A peer authorized for an IPv4 range and an IPv6 range needs two peer entries, which are two IKE peers rather than one authorization |

The change is `remote-id` becoming a leaf-list, `AuthConfig.RemoteID` a slice, and
`remoteIDMatches` admitting an identity any entry admits. It touches six producers
(`ipsec/config.go`, `ipsec/validate.go`, `engine/cert_payload.go`, `engine/remote_id.go`
at three sites) and the tests that pin the string form, which is why it is scheduled
rather than done in the 2026-09-21 pass (owner ruling: every MUST is a requirement,
implemented when S-sized and scheduled with a spec otherwise). The certificate binding
stays per entry: `certificateCarriesIdentity` binds the asserted identity, not the list.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ipsec.md` - "Remote identity", the one-value form this spec widens

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Section 4.4.3.3

## Current Behavior (MANDATORY)

- [ ] `internal/component/ike/engine/remote_id.go` - `addressEntryMatches` parses one address or one prefix

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
| `set vpn ipsec site-to-site peer <p> authentication remote-id <v4-range>` and `<v6-range>` | → | `remoteIDMatches` | `TestRFC4301PADAdmitsBothAddressFamilies` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | RFC4301-4.4.3.3-1 carries a tagged positive and negative test | `./le rfc check` prints no line for it |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4301PADAdmitsBothAddressFamilies` | `internal/component/ike/engine/rfc4301_pad_family_test.go` | one peer entry admits an IPv4 and an IPv6 asserted address, and refuses an address outside both ranges | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `remote-id` becomes a leaf-list
- `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` over the list
- `rfc/short/rfc4301.md` - the row loses its `{gap}`

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

`rfc/short/rfc4301.md` row RFC4301-4.4.3.3-1.

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
