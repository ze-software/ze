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

One defect met on 2026-11-12 while
`spec-single-owner-lifecycle-5-session-end-locals` was implemented. It has a
journal row (Owned journal rows). Nothing here is designed yet.

| Part | Producer | What the agent meets today | Why it is owed |
|------|----------|----------------------------|----------------|
| A unit HEAD never held is judged weakened | `Proposed` and `proposedFindings` (`internal/le/test/weakened/proposed.go`), read by the `pretool-writeedit` hook | The check refused an agent's removal of a test helper it wrote minutes earlier in a tracked test file. HEAD does not hold the helper, so nothing committed could be weakened | A false refusal is a hook defect (`ai/META-DECISIONS.md`, "How agents work"), and it blocks the author's own repair |

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
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/le/test/weakened/` and `./le ai hooks unit`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
