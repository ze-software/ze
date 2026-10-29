# Spec: fttx-edge-unproven-row-review

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | docs |
| Priority | P2 |
| Release reason | Accurate support claims: misclassified rows lower the fttx-edge release profile's figure and misstate what Ze owes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Review only the fttx-edge MUST rows that currently count against Ze's figure:
the Ze-owned rows not proven (`{gap}`, `{partial}`, or no tagged test with a
verifying discrimination record). On the live tree on 2026-10-29 that is 189
rows (529 Ze-owned, 340 proven). This is not a full audit: rows already
proven, and rows already excluded, are outside the review.

Sort each row into exactly one bin:

| Bin | Meaning | What follows |
|-----|---------|--------------|
| Misclassified | The row belongs in another section (OS met, OS blocked, declined, not applicable) or its note misreads the RFC | Corrected in the summary, citing evidence |
| Implemented, untested | Ze's code already performs the obligation; no tagged test with a recorded break proves it | Handed to `spec-fttx-edge-test-quick-wins`; a `{gap}` or `{partial}` note on such a row is removed here, because the figure never proves an annotated gap |
| Real gap | Ze does not perform the obligation | Stays a correctness gap; owned by the spec that implements it |

Every reclassification cites checkable evidence: the RFC sentence with its
section, read in `rfc/full/`, and the producing code or the kernel behaviour.
A row whose evidence cannot be established stays as it is.

Known case: RFC5382-4.3-1 (and 4.3-2, 4.3-3) is described as "ze installs
nothing that holds an unsolicited inbound SYN", but RFC 5382 Section 4.3 only
forbids responding within 6 seconds; a silent drop meets all three MUSTs. Its
note and its claim that the router answers with an immediate RST are
unverified.

Known case outside this population, recorded for the owner: RFC 9915 "The
client MUST include an Option Request option (ORO)" is filed as
feature-declined, which a MUST sentence cannot make optional. It inflates the
proposed Ze figure rather than lowering it.

Related: `plan/post-release/features/spec-nat44-customer-napt.md` owns the
RFC 4787 / 5382 implementation; `plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md`
owns RFC 4861 Ze-owned gaps. This spec only classifies.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - annotation kinds, coverage figures, the linux-limit annotation
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: a `{gap}` or `{partial}` row is never proven, whatever records its tags carry; a row moved to "implemented, untested" must lose that annotation or no test can raise the figure

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
