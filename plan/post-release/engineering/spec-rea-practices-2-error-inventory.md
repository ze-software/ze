# Spec: rea-practices-2-error-inventory

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Measures compliance with `ai/rules/user-facing-errors.md`; it changes no shipped behaviour by itself |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/post-release/engineering/spec-rea-practices-0-umbrella.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

REA keeps an inventory of every user-visible error message (REA
`docs/user-visible-errors.csv`): location, trigger, current copy, user risk,
harm rank, proposed replacement, implementation status and verification
result. Ze has the rule every message must meet (`ai/rules/user-facing-errors.md`:
what went wrong, why, and the exact command that fixes it) but nothing that
shows which messages meet it.

Goal: an inventory Ze derives from the code rather than keeps by hand, so it
cannot drift from the messages it lists (`ai/rules/principles.md`, "Every
fact MUST be declared once"). It lists each user-visible message, its
surface, and whether it meets the rule, and it becomes the worklist for
fixing the ones that do not.

Open at research: how messages are found in the code (error constructors,
`ze doctor` reports, CLI refusals, MCP and API error answers), and whether a
harm ranking can be derived or must be a judged field.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/user-facing-errors.md` - the rule the inventory measures
  → Constraint: [to be written at research]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `ai/rules/user-facing-errors.md` - states the message rule; no inventory exists

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [found at research] | | | |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters]

### Transformation Path
1. [Stage 1]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [to be written at research] | | No |

### Integration Points
- [to be written at research]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Most user-visible messages can be found mechanically from the code | REA builds its inventory by hand; Ze's rule forbids a hand copy | The inventory needs a registration point for messages, a larger change | Research survey of error construction sites | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [to be written at design] | → | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [to be written at design] | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be written at design] | | | |

## Files to Modify
- [to be written at design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [to be written at design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs named at design, for example `./le test unit <group>`
