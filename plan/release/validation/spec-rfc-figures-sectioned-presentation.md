# Spec: rfc-figures-sectioned-presentation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Accurate support claims: the published RFC coverage figure divides by rows Ze's code does not owe, so it misstates what Ze has done |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The RFC compliance pages publish one mandatory figure per RFC and per profile:
rows proven over every MUST row, with a "ceiling" beside it. Rows the operating
system performs, rows of an optional feature Ze declined, not-applicable rows
and Linux-limit rows all sit in the denominator. For the fttx-edge profile the
page reads 338 of 747 (45.2%) while Ze's own code owes 529 of those rows. The
figure therefore misreports Ze's state, and the ceiling hides the reason.

Replace the single figure and its ceiling with sections, following the owner
rules in `ai/META-DECISIONS.md` ("Goal and scope": an OS-met requirement
without a test is a testing gap; an OS-blocked requirement is reported in its
own section and never counts against Ze's mandatory coverage or holds a
release).

| Section | Counts | Status values |
|---------|--------|---------------|
| Ze MUST coverage | MUST rows Ze's code owes: total minus OS met, OS blocked, declined, not applicable | proven, correctness gap |
| OS met | MUST rows a lower layer performs, per OS and kernel version | verified (a test checks the OS behaviour on the state Ze installs), testing gap |
| OS blocked | MUST rows Linux cannot provide the primitive for, with a verified limit entry | blocked; never counted against Ze, never holds a release |
| Declined feature | MUST rows conditional on an optional feature Ze does not offer | declined, citing the RFC sentence that makes it optional |
| Not applicable | MUST rows that do not bind Ze's role | listed with evidence, not counted |
| Optional features | optional features offered over optional features defined | ratio; needs a per-summary declaration of each optional feature, offered or declined |
| Optional requirements | SHOULD / MAY rows proven | proven, not proven |

Presentation, as agreed against the mockup
(`tmp/session/2026-10-29-0316d5ab-6d80-4948-9be1-17812c7573b8/scratch/rfc-figures-mockup-fttx.md`):

| Surface | What it shows |
|---------|---------------|
| Profile summary | One line per section, plus an "All MUST rows" line equal to the sum of the five MUST sections |
| RFC list (`?profile=<name>&rfc=<stems>#rfc-list`) | One row per RFC: MUST total, then a column pair per section; a Total row at the bottom whose five MUST section columns add up to the MUST total, with a check line |
| RFC detail page | Each requirement with level, section, status and evidence, ending in a total that matches the RFC's row in the list |

Counts are summed across RFCs and divided once; percentages are never averaged.
The ceiling presentation is removed, not kept beside the sections
(`ai/rules/no-layering.md`). Published figures are corrected on the surfaces
that carried them, saying what the old figure counted.

Related: `plan/release/validation/spec-excluded-requirement-owes-an-observable-test.md`
adds the gate that enforces the OS-met test; this spec shows its absence as a
testing gap. Neither waits on the other.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "Coverage figures", "The linux-limit annotation"
  → Constraint: filled at RESEARCH
- [ ] `docs/contributing/gh-pages.md` - quality pages, profile pages and the filtered RFC list
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: `figuresByStem` keeps `{not-applicable}`, `{lower-layer}`, `{feature-declined}` and `{linux-limit}` rows in `Total` and never proves them "whatever records their tags carry"; an OS-met "verified" status needs a new proof predicate for those rows, separate from Ze's `Proven`
  → Constraint: `{gap}` and `{partial}` rows are never proven even with a verifying record; this stays for the Ze section
  → Decision: the code comment cites an owner ruling of 2026-10-25 that "annotating a row may not raise its figure"; the new owner rules supersede it for the headline, so the comment and `docs/contributing/rfc-conformance-gates.md` "Coverage figures" are rewritten with the code
  → Constraint: `rollupOf` sums only walked members and reports `NotWalked`; the sectioned rollup keeps that rule

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
