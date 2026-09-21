# Spec: ipsec-spd-selection

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

Three RFC 4301 Section 4.4.1 obligations describe SPD SELECTION features Ze does not
offer today. Each is a feature the owner can decline in one word; until he does, each
is a requirement (owner ruling, 2026-09-21: every MUST is a requirement, and nothing is
declined by an agent). Ze holds ONE SPD (`internal/component/ike/ipsec/spd_policy.go::parseSPDPolicies`
reads one `policy` list under the `ipsec` container, with no interface or VRF tag), no
local user or application principal, and no name-valued selector.

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC4301-4.4.1-5 | "However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing." (Section 4.4.1) | Ze has one SPD: `ipsec/spd_policy.go::parseSPDPolicies`. The feature is a per-interface or per-VRF SPD with a selection function keyed on the packet's ingress or egress; the section makes it optional: "An IPsec implementation MAY support multiple SPDs" |
| RFC4301-4.4.1-8 | "However, the system administrator MUST be able to specify whether or not a user or application can override (default) system policies." (Section 4.4.1) | No user or application principal exists in the SPD model: `ipsec/spd_policy.go::SPDPolicy` carries no override leaf. Ze is a router with no local application requesting IPsec, so the feature is an `override` leaf the administrator sets and a request path that honours it |
| RFC4301-4.4.1.1-5 | "All IPsec implementations MUST support this use of names." (Section 4.4.1.1, the Name selector used to select an SPD entry during IKE) | Ze selects a peer's PROTECT entries by the peer the session belongs to, and `engine/remote_id.go::remoteIDMatches` authorizes the asserted name in the PAD. `ipsec.SPDPolicy` carries no name selector, so an operator BYPASS or DISCARD entry cannot be selected by the IKE identity of the peer |

Each feature lands as its own phase with its own tagged pair per requirement id. A
feature the owner declines is recorded on its row as `{feature-declined: "<the RFC
sentence making it optional>"; <producer>}` where the RFC makes it optional, and
`{not-applicable: <the owner's words>}` where it does not.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the one-SPD model this spec widens

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Section 4.4.1 and 4.4.1.1, the three rows above

## Current Behavior (MANDATORY)

- [ ] `internal/component/ike/ipsec/spd_policy.go` - one policy list, no SPD tag, no name selector, no override leaf

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
| `set vpn ipsec policy <name> ...` | → | `parseSPDPolicy` | `TestRFC4301SPDSelectionReachesTheDataplane` |

## Acceptance Criteria

| AC | Criterion | Assertion |
|----|-----------|-----------|
| AC-1 | Each requirement id above carries a tagged positive and negative test, or the owner's decline on its row | `./le rfc check` prints no line for it |

## End-to-End User Stories

Skeleton: filled at design.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4301SPDSelectionReachesTheDataplane` | `internal/component/ike/engine/rfc4301_spd_selection_test.go` | each feature above, one tagged pair per requirement id | |

### Boundary Tests (numeric inputs)
Skeleton: filled at design.

### Functional Tests
Skeleton: filled at design.

### Interop Tests (Scope: protocol)
Skeleton: filled at design.

## Files to Modify

- `internal/component/ike/ipsec/spd_policy.go` - the selector and override leaves
- `rfc/short/rfc4301.md` - the three rows lose their `{gap}`

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

`rfc/short/rfc4301.md` rows RFC4301-4.4.1-5, RFC4301-4.4.1-8 and RFC4301-4.4.1.1-5.

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
