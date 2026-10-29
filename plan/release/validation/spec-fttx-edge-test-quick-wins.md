# Spec: fttx-edge-test-quick-wins

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation: the fttx-edge release profile owes proof of its MUST rows, and rows Ze already meets carry no recorded break |
| RFC gaps | - |
| Depends | spec-fttx-edge-unproven-row-review |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Raise the fttx-edge Ze MUST figure and the OS-met verified figure with tests,
not with reclassification.

Input: the "implemented, untested" bin produced by
`spec-fttx-edge-unproven-row-review`, plus the 119 OS-met rows (live tree
2026-10-29).

| Work | Population | Proof owed |
|------|------------|------------|
| Ze quick wins | Rows Ze's code already performs, ranked by rows gained per unit of effort | A tagged test and a recorded break (`./le rfc record`) per row |
| Cheapest of all | Rows whose tagged test exists but has no verifying discrimination record | Record the break only |
| OS-met verification | OS-met rows where a test of what Ze installs plus the kernel behaviour is cheap | A test on Ze's kernel asserting the installed state and the behaviour, with a break of what Ze installs |

The spec ranks the candidates first, then writes the tests in that order.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - tags, discrimination records, `./le rfc record`
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: `{lower-layer}` rows are never proven today, so OS-met tests raise the OS-met verified figure only once `spec-rfc-figures-sectioned-presentation` adds that predicate; until then they are still owed by `ai/rules/rfc-compliance.md` and recorded
  → Constraint: a row is proven only by a verifying discrimination record that is not the `no-break` escape (`DiscriminationVerdict.Proven`)

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
