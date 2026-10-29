# Spec: closure-review-covers-spec-commits

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | - |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas's rule (`ai/META-DECISIONS.md`, "Ze is pre-release"): work is committed
as soon as a piece of it is complete, so a spec's implementation lands as
several commits, not one at closure. The closure review gate was built for the
old shape: `CheckReview` (`internal/le/commit/review.go`) judges the review
artifact's coverage against the closure commit's own paths only, so code a
spec committed before closure reaches `main` and is never covered by the
review. `ai/rules/planning.md` now states that gap.

Thomas chose the fix (2026-10-29): the closure review covers everything the
spec committed since it started, not only the closure commit. Intermediate
commits owe no review of their own.

What has to exist, to be designed at RESEARCH and DESIGN:

| Need | Why |
|------|-----|
| Each commit records the spec it belongs to | No commit carries its spec today (no `Spec:` trailer), so nothing can list a spec's commits. `./le commit create` knows the session's claimed spec (`./le spec current`) |
| The closure gate gathers the reviewable files of all the spec's commits plus the closure commit | So the review artifact must cover the whole spec's change set, judged against the files' current hashes |
| A refusal names the uncovered files and the exact `./le spec review` command | `ai/rules/user-facing-errors.md` |

Open for DESIGN: how a commit made by an agent that claimed no spec is
attributed, and what the gate does with a spec whose earlier commits predate
the trailer.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - commit keywords, refusals
  → Constraint: filled at RESEARCH
- [ ] `docs/contributing/spec-workflow.md` - closure commits A and B
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/review.go` - `closureStem`, `CheckReview`, `judgeReviewCoverage`
  → Constraint: coverage is judged over the `paths` of the commit under construction only

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters] filled at RESEARCH

### Transformation Path
- filled at RESEARCH

### Boundaries Crossed
- filled at RESEARCH

### Integration Points
- filled at RESEARCH

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| filled at DESIGN | → | | |

## Acceptance Criteria

| AC ID | Input / Trigger | Expected Behavior |
|-------|-----------------|-------------------|
| filled at DESIGN | | |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| filled at DESIGN | | | |

## Files to Modify
- filled at DESIGN

### Integration Checklist
- filled at RESEARCH

### Documentation Update Checklist (BLOCKING)
- filled at RESEARCH

## Implementation Steps
- filled at DESIGN

## Owned journal rows

| Row | Journal file |
|-----|--------------|
| - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le verify changed` evidence for the change set
