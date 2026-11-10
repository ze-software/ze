# Spec: weekly-fragment-measure-verdict

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Development tooling for the weekly public update; no shipped behaviour changes |
| Scope | tooling |
| Depends | two weekly updates recorded in `plan/records/weekly-fragment-measure.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The news-fragment gate (`internal/le/commit/newsfragment.go`,
`newsFragmentRefusal`) and the fragment format
(`website/changes/fragments/README.md`) landed on two assumptions that only the
weekly updates written after them can test. Thomas agreed the test: over the
next two weekly updates, record per published item which of the five facts
came from a fragment, from research, or failed verification
(`plan/records/weekly-fragment-measure.md`, written by
`ai/skills/ze-weekly-update.md` Phase 1 step 9).

This spec owns reading those two tables and acting on them:

| Assumption | Holds when | If it does not |
|------------|------------|----------------|
| Agents write specific fragments when the format demands operator-visible Before and After | The measure's Before column reads mostly `fragment`, few `failed` | Tighten the format or the gate, and say how in this spec |
| Fixes outside a spec are covered well enough by the skill instruction (`plan/journal/README.md`) | Few published fixes carry no fragment | Design a check that a `fixed:` journal row with an operator-visible fix carries a fragment |

It also records the count of fragment-less fixes in the two measured weeks.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| News-fragment gate | `internal/le/commit/newsfragment.go` `newsFragmentRefusal`, `parseNewsFragment` | Refuses a closure without a fragment or `no-news`; judges every carried fragment's shape | Extends, if the measure shows the format too weak |
| The measure record | `plan/records/weekly-fragment-measure.md` | Holds one table per weekly update | Reads it: the evidence this spec judges |
| Weekly skill | `ai/skills/ze-weekly-update.md` Phase 1 steps 4, 8, 9 | Reads fragments, verifies their claims, records the measure | Extends, if the measure shows research still carries the facts |
| Journal fix guidance | `plan/journal/README.md`, after "The Fix marker" | Instructs a fixing agent to write a fragment; no check | Replaced by a check only if the measure shows fixes leak |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the design every le change keeps
  → Constraint: a commit gate that refuses wrongly blocks every concurrent session at once (`plan/learned/HOOK-FRICTION.md`)
- [ ] `website/changes/fragments/README.md` - the fragment format the measure judges
- [ ] `plan/records/weekly-fragment-measure.md` - the evidence
  → Constraint: nothing is designed before two update tables exist there

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/newsfragment.go` - `newsFragmentRefusal`, `newsHeadings`, `parseNewsFragment`: the gate and the format declaration
- [ ] `ai/skills/ze-weekly-update.md` - Phase 1 steps 4, 8 and 9: how the measure is recorded

**Behavior to preserve:**
- Every closure says whether operators see a change, through a fragment or `no-news`

**Behavior to change:**
- Decided by the measure; nothing is decided before it

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The two measure tables in `plan/records/weekly-fragment-measure.md`

### Transformation Path
1. Read both tables; count `fragment`, `research`, `failed` per fact, and items with no fragment
2. Judge each assumption in the Task table
3. Design the tightening, if any, in this spec

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Repository → this spec | The measure file | No |

### Integration Points
- To be decided by the measure

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at design |
| No unintended coupling (components stay isolated) | No | to be filled at design |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled at design |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, tooling |
| Registration over hardcoding | No | to be filled at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Agents write specific fragments when the format demands operator-visible Before and After | Thomas's agreement (spec-weekly-news-fragments) | Fragments become filler | The two-week measure | unvalidated |
| A-2 | Fixes outside a spec are covered well enough by the skill instruction | Thomas's choice; no marker exists | User-visible fixes reach the week with no fragment | Count fragment-less fixes in the two measured weeks | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The weekly writer skips step 9, so no table is recorded | The measure file holds no table after an update | The skill's Done-when names the measure |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible in Ze; the weekly update keeps needing research |
| How is it reverted? | A revert of whatever tightening this spec lands |
| Who else touches this path? | Every session's `./le commit create`; the weekly writer |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| To be decided by the measure | → | | |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Two update tables in `plan/records/weekly-fragment-measure.md` | This spec records a verdict on A-1 and A-2 with the counts | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be decided by the measure | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | | Tooling | |

## Files to Modify
- `plan/records/weekly-fragment-measure.md` - read; the verdict is written here in this spec

## Files to Create
- none until the measure decides

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | N-A | decided by the measure |
| Doctor check for runtime dependencies | N-A | no runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | N-A | decided by the measure |

## Implementation Steps

1. **Phase: Verdict** -- read the two tables, judge A-1 and A-2, design any tightening

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Waits for two weekly updates.

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
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label unit-pkg command go test ./internal/le/commit/` when the gate changes (no `./le test unit` group covers `internal/le`; `./le test functional runner` is unaffected)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-weekly-fragment-measure-verdict.md` with `no-news "<reason>"` unless a fragment was written
</content>
</invoke>
