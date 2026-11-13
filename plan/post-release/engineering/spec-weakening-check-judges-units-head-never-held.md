# Spec: weakening-check-judges-units-head-never-held

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling: the pretool test-weakening check refuses an agent's repair of test code that nothing committed holds, so the agent's honest edit is blocked. It ships nothing, so it is post-release engineering. |
| RFC gaps | - |
| Depends | `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (it owns this class's other `Proposed` rows and plans to close them in its changed-set phase; this spec is coordinated with it, or merged into it, at that spec's design pass) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects in how the weakening check and its approval route name a test
unit, all met on 2026-11-12: the first while
`spec-single-owner-lifecycle-5-session-end-locals` was implemented, the second
during an IKE dataplane test cleanup, the third while a test was added to an
untracked IKE dataplane test file. Each has a journal row (Owned journal
rows). Nothing here is designed yet.

| Part | Producer | What the agent meets today | Why it is owed |
|------|----------|----------------------------|----------------|
| A unit HEAD never held is judged weakened | `Proposed` and `proposedFindings` (`internal/le/test/weakened/proposed.go`), read by the `pretool-writeedit` hook | The check refused an agent's removal of a test helper it wrote minutes earlier in a tracked test file. HEAD does not hold the helper, so nothing committed could be weakened | A false refusal is a hook defect (`ai/META-DECISIONS.md`, "How agents work"), and it blocks the author's own repair |
| A method unit cannot be approved when another type shares its bare name | `approvalUnitTexts`, `changedApprovalCarriers` and `ambiguousApprovalCarrier` (`internal/le/rfc/approvalunits.go`), run by `./le test weakened approve`. The detector also names a unit by its bare name and merges a file's same-named functions into one (`unitsByName`, `internal/le/test/weakened/scope.go`) | A unit is `<package>.<Name>` with no receiver. `dataplane.Close` is refused as "ambiguous across carriers" because three test types in one directory declare `Close`, so the judge-approved removal of the dead `fakeESPFormConn.Close` cannot be recorded, and the refusal names no fix | An approved test change cannot land, so dead test code stays. `le rfc record` already resolves `Type.Method` (`scopeIndex.keyText`, `internal/le/rfc/goscope.go`). The unit grammar cuts at the last dot to admit a dotted directory key (`SplitApprovalUnit`), so a receiver-qualified key has to coexist with it |
| A test function only the new text holds is judged a weakening, and a pre-edit approval of a file HEAD lacks goes stale on its own edit | `weakenedUnits` (`internal/le/test/weakened/scope.go`), called by `proposedFindings`; `ApprovalContentAdmits` (`internal/le/rfc/approvalunits.go`) | Adding `TestESPFormFastPathGateFollowsTheKernelRuleset`, whose one `t.Skip` is a root guard, was refused as "adding t.Skip (0 -> 1); the test stops running": the named comparison walks the old text's units only, so the new function's skip reaches the residual through the whole-file count. The judge's approval recorded base `none`, which never equals the recorded current digest, so it was read as a post-edit approval of the pre-edit text and refused the edit it approved | A false refusal is a hook defect, and the approval route that should release it cannot, so a new guarded test cannot be added without a second judge run |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
- [ ] `ai/doors/le-tooling.md` - the hooks
  → Constraint: a hook that refuses wrongly blocks every session at once.

**Key insights:**
- The RFC-tag half of `Proposed` already reads HEAD (`proposedCommitted`). The weakening half does not.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/weakened/proposed.go` - `Proposed` reads HEAD's text only for a whole-file edit of an RFC tag carrier, to find committed tagged units. The weakening verdict comes from `proposedFindings`, which compares the edit's old and new text only and never reads HEAD.

**Behavior to preserve:**
- A weakening of a unit HEAD records is refused until a `ze-test-judge` approval is recorded.

**Behavior to change:**
- A change to a unit HEAD does not record is not judged a weakening.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters]

### Transformation Path
1. [Stage 1]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [boundary] | [how] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [hook call that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

Draft row. The design pass confirms or rewrites it.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An edit removes a test function from a tracked test file, and HEAD's copy of that file holds no such function | The check reports no weakening for it | None |
| AC-2 | An edit removes a test function HEAD records | The check still refuses it without an approval | None |
| AC-3 | A judge's approval names one method of a test type whose bare name another type in the same package also declares | `./le test weakened approve` records it for that method alone; the check accepts the removal of that method and still refuses an unapproved removal of the other type's method | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named in design] | `internal/le/test/weakened/` | AC-1, AC-2 | |

## Files to Modify
- [named in design]

## Files to Create
- [named in design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [named in design]

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/guard-blocks-its-own-authors-repair.md` | 2026-11-12 | `Proposed` refuses removal of a helper HEAD never held |
| `plan/journal/two-owners-share-one-name.md` | 2026-11-12 | `./le test weakened approve` refuses a method unit whose bare name another type shares |
| `plan/journal/hook-existing-patterns-false-positive.md` | 2026-11-12 | `weakenedUnits` refuses a new test function's guard skip; `ApprovalContentAdmits` stales a pre-edit approval of a file HEAD lacks |
| `plan/journal/hook-existing-patterns-false-positive.md` | 2026-11-12 | `commentedAssertionPattern` (`detect`, called by `Proposed`) reads a doc comment naming `t.Errorf` as a commented-out assertion |
| `plan/journal/hook-existing-patterns-false-positive.md` | 2026-11-13 | `weakenedUnits` counts an assertion moved into a same-file helper the unit calls as removed |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/le/test/weakened/`, `./le test unit pkg path ./internal/le/rfc/` (AC-3's resolver) and `./le ai hooks unit`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
