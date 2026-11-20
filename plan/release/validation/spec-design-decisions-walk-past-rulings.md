# Spec: Thomas's past design rulings become Design decisions rows

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | The gate of `spec-design-needs-owner-approval` protects only a decision written as a row of a design page's `## Design decisions` section, and every ruling Thomas made before 2026-11-20 lives elsewhere, so none of them is protected until it is a row (`plan/README.md`, `release/validation/`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-20 |

## Task

Thomas chose option (a) of C-7 in `spec-design-needs-owner-approval` on
2026-11-19: "a child spec in `release/validation` that walks them and proposes
each row for you to confirm". The decisions he made before that spec live in
`plan/records/`, `plan/learned/DESIGN-HISTORY.md` and door lines. Until a past
decision is a row of the page that documents its subject, no edit gate owes
its reading, no spec owes it a verdict, and no commit refuses its removal.
That spec wrote one row only: the 2026-08-21 pipe decision in
`docs/architecture/api/commands.md`.

This spec walks those records, and for each ruling proposes one row (the
decision as he stated it, what it forbids, its Source, its Date) on the page
that documents its subject, for Thomas to confirm before it is written.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The page format | `docs/architecture/README.md`, "Page format" | Defines the `## Design decisions` section, its columns and who may change a row | Extends: each confirmed ruling becomes a row in that format |
| The section check | `internal/le/doc/consistency/decisions.go` `JudgeDecisionRow`, `./le doc consistency` | Refuses a row with an empty cell, a bad Date or a Source that does not resolve | Does not change: every proposed row passes it before it is written |
| The decision records | `plan/records/`, `plan/learned/DESIGN-HISTORY.md` | Hold Thomas's rulings as history | Does not apply as a home: each becomes the Source cell of its row |
| The derivation | `./le spec design documents` (`internal/le/spec/design/documents.go` `derive`) | Lists the decision rows a spec owes a verdict on | Does not change: each new row is owed by every spec whose files link its page |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/README.md`, "Page format" - the row format
  → Constraint: a row is added by the main session, and changes only with Thomas
- [ ] `docs/contributing/spec-workflow.md`, "Design approval" - what reads the rows

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/doc/consistency/decisions.go` - `ParseDecisions`, `JudgeDecisionRow`: the parser and the row judgment every proposed row must pass

**Behavior to preserve:**
- The 2026-08-21 row of `docs/architecture/api/commands.md`

**Behavior to change:**
- Each confirmed past ruling becomes a row of the page that documents its subject

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A record under `plan/records/`, a row of `plan/learned/DESIGN-HISTORY.md`, a door line

### Transformation Path
1. The walk reads a ruling and finds the page that documents its subject
2. The main session presents the proposed row to Thomas
3. On his confirmation the row is written, with the record as its Source

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Record to design page | A row the main session writes on Thomas's confirmation | No |

### Integration Points
- The `## Design decisions` section of each page that documents a ruling's subject

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le doc consistency` over each page that gains a row | → | `checkDesignDecisions` | to be named at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | | the rows are documentation, judged by the existing `design-decisions` check | |

## Files to Modify
- The design pages that document each ruling's subject, named at design

## Implementation Steps
1. **Phase: walk** -- list every ruling with its record and the page that documents its subject
2. **Phase: rows** -- each row Thomas confirms is written, and `./le doc consistency` judges it

## Checklist

### Goal Gates (MUST pass)
- [ ] Every ruling of the walk is a confirmed row or named as declined by Thomas
- [ ] `./le test unit pkg path ./internal/le/doc/consistency/` green, and `./le doc consistency` reports no `design-decisions` finding

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
