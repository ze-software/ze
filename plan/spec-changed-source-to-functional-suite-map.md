# Spec: changed-source functional-suite coverage and CI consumption

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | plan/spec-verify-scope-5-suite-coverage-map.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The original proposal sought a source-path-to-functional-suite binding shared
by local verification and CI. The recorded-map implementation in
`plan/spec-verify-scope-5-suite-coverage-map.md` now supplies the Go-package
selection provider, and its measurements rejected the static-map approaches
this skeleton originally proposed.

This skeleton owns the remaining assessment of that original scope: establish
which changed inputs outside the Go-package answer, including `.ci`, `.et` and
`.wb` test inputs, are covered by the existing selector, and whether CI consumes
the same decision as local verification. Any demonstrated gap is repaired
through the existing provider. If no gap remains, record the evidence for an
owner-reviewed disposition rather than inventing a second implementation.

The ownership boundary is explicit:

| Behaviour | Owner |
|-----------|-------|
| Suite inventory | `Gating` and the catalog in `internal/le/functional/suites.go`, retained by verify-scope-5 |
| Coverage recording, map format/freshness, fail-open selection and local/gating run plan | `plan/spec-verify-scope-5-suite-coverage-map.md` |
| Residual changed-input coverage and CI consumption of that same provider | this spec |
| Aggregate verification cost and freshness ACs | `plan/spec-verify-scope-0-umbrella.md` |

`planRun` and `selectSuites` in `internal/le/functional/suitemap.go` are the
provider. An unanswerable input must remain visible and widen the run; it must
never silently select no coverage. A declarative source-glob map beside the
recorded artifact is no longer an implementation task.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - states the affected-population rule
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/changed/selector.go` and `scope.go` - the changed-package answer consumed by functional selection; research must trace the non-Go input cases before claiming a residual gap
- [ ] `internal/le/functional/suitemap.go` - `selectSuites` consumes `changed.Packages`, widens on unknown packages or an unusable map, and `planRun` supplies the local report and gating decision

**Behavior to preserve:**
- <to be filled>

**Behavior to change:**
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| <to be filled> | <to be filled> | <to be filled> |

### Integration Points
- <to be filled>

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <to be filled> | → | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | <to be filled> | <to be filled> |

### Functional Tests

Tooling only, no daemon code. The driving surface is
`internal/le/functional` and its consumers. Evidence must show that a mapped
change selects the suites recorded as reaching it, that unknown inputs widen
visibly, and that local and CI consumers use the same provider. Reuse the
verify-scope child's proof for behaviours it owns; add no duplicate selector.

## Files to Modify

- `internal/le/` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify worktree` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
