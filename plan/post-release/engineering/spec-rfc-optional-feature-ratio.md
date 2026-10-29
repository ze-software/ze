# Spec: rfc-optional-feature-ratio

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | - |
| RFC gaps | - |
| Depends | spec-rfc-figures-sectioned-presentation |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Publish, per RFC and per profile, how many of the optional features an RFC
defines Ze offers: "optional features offered / optional features defined"
(the fttx-edge mockup showed 12 / 20, with invented numbers). Thomas chose to
keep this figure beside the per-row SHOULD / MAY states that
`plan/release/validation/spec-rfc-figures-sectioned-presentation.md`
introduces, as its own spec so it does not hold that one up.

The figure counts features, not rows: one optional feature (an RFC option, a
mode, a transport) usually spans several requirement rows, MUST rows
conditional on it included. It needs a per-summary declaration of each
optional feature the RFC defines, whether Ze offers or declines it, and the
RFC sentence that makes it optional. Declining a feature follows
`ai/META-DECISIONS.md`: only a feature Thomas named is declined.

Counts are summed across RFCs and divided once; percentages are never averaged.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "Coverage figures"
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: no feature-level record exists; `{feature-declined}` is a per-row annotation only

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
