# Spec: fixit-learned-staleness-measurement

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`path_problem` in `scripts/dev/learned_staleness.py` tests a citation with a
filesystem existence check, so the dead-reference count is a property of the
checkout it runs in rather than of the corpus.

A developer tree holds gitignored build outputs and generated files that a
fresh CI clone never has, so `--write-baseline` run in a dev checkout records a
ceiling CI cannot reach. Measured 2026-08-08: 317 locally against 330 for
`origin/main` extracted with `git archive`. All 13 extra findings are citations
to paths that are never tracked: six under `test/perf/results/`, `CLAUDE.md`,
and three directories under `.claude/`. None of them is decay, so
`learned_repath.py` cannot help; it only repoints files git recorded as renamed.

**This collides with a blocking rule and the collision must be resolved before
any fix lands.** `ai/rules/planning.md` states the ceiling is drained, never
removed and never widened, while `--raise-baseline` is literally a widening.
The route that avoids the collision is to drain those 13 citations first, after
which the tracked-set count lands at 317 and no raise is needed.

**Status note:** this work was handed to another session on 2026-08-08 with a
reproduction recipe. This spec is the durable record, not a second claim on the
work. Check with that session before starting.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - orientation for the owning layer
  -> Decision: [fill at design time]
  -> Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- [fill at design time]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/learned_staleness.py` - the existence test and the ceiling
- [ ] `scripts/dev/learned_repath.py` - consumes the same dead-path set, so a signature change lands in both

**Behavior to preserve:**
- The shrink-only ceiling idiom. The defect is the measurement, not the ratchet.

**Behavior to change:**
- [fill at design time]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
[fill at design time]

### Transformation Path
[fill at design time]

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| [fill at design time] | [fill at design time] | [fill at design time] |

### Integration Points
[fill at design time]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| [fill at design time] | -> | [fill at design time] | [fill at design time] |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| [fill at design time] | scripts/dev/learned_staleness_test.py | dev and CI counts agree |

### Functional Tests
- [ ] `scripts/dev/learned_staleness_test.py` covers a tracked path and an untracked-but-present path

## Files to Modify

Not yet determined. The owning file follows from the diagnosis, which is
the first job of whoever takes this spec.

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Checklist

### Goal Gates (MUST pass)
- [ ] The same commit yields the same count in a dev checkout and a fresh clone
- [ ] The never-widened rule is honoured, or an explicit owner ruling is recorded

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit
