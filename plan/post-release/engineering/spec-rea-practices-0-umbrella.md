# Spec: rea-practices-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Practices adopted from another agent-built project; none fixes incorrect shipped behaviour, so none holds the first release |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas asked which practices of REA ("Reverse Engineer Anything", an MCP
server and CLI built mostly by agents, local checkout
`/Users/thomas/Code/github.com/morluto/rea`) Ze could learn from, then asked
for a spec that brings them into Ze. He chose an umbrella with child specs,
and the MCP work in `post-release/features/`.

REA practices compared with Ze on 2026-11-08, and where each one goes:

| # | REA practice | REA source | Ze today | Home |
|---|--------------|------------|----------|------|
| 1 | The MCP tool catalog is measured as context cost on real agent clients, compatibility profiles keep it loadable, and per-client lanes prove a real client loads it | REA `docs/mcp-catalog-evaluation.md`, `docs/development/client-verification.md`, `docs/reference/mcp-contracts.md` "Compact input schema profile" | Every operator command reaches MCP as a derived tool (`ai/META-DECISIONS.md`), and nothing measures what the catalog costs an agent or tests a real client | `plan/post-release/features/spec-rea-practices-1-mcp-catalog-cost.md` |
| 2 | An inventory of every user-visible error, ranked by harm, with current copy, proposed copy, status and verification | REA `docs/user-visible-errors.csv` (126 rows) | `ai/rules/user-facing-errors.md` states the rule; nothing measures which messages meet it | `plan/post-release/engineering/spec-rea-practices-2-error-inventory.md` |
| 3 | A lifecycle regression scenario list that each start-up, cancellation or cleanup change selects from | REA `docs/development/testing.md` "Lifecycle regressions" | `ai/rules/goroutine-lifecycle.md` covers goroutines, not plugin process supervision or reload scenarios | Done: `ai/rules/testing.md`, "Lifecycle Regressions" (points under `ai/rules/points/testing/lifecycle-regressions/`), and the testing trigger now names plugin supervision, config reload and daemon stop |
| 4 | Fix the tool's result before writing skill or doc advice about it | REA `AGENTS.md` "Tool and Test Changes" | Close to `ai/rules/user-facing-errors.md`, not stated for MCP and agent-facing output | Done: `ai/rules/writing.md`, "Documentation", point `fix-the-tool-output-before-writing-advice-about-it` |

Items 3 and 4 are rules about how agents work, so they land as rule edits,
not as specs. Items 1 and 2 are each their own spec because they belong in
different work categories (shipped binary versus `le` tooling).

This umbrella closes when both children are closed and the two rule edits
are committed.

## Children

| Child | Owns |
|-------|------|
| `plan/post-release/features/spec-rea-practices-1-mcp-catalog-cost.md` | Item 1 |
| `plan/post-release/engineering/spec-rea-practices-2-error-inventory.md` | Item 2 |

## Required Reading

Owned by each child.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `ai/rules/user-facing-errors.md` - the rule item 2 measures

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [filled by each child] | | | |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters] Owned by each child.

### Transformation Path
1. Owned by each child.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Owned by each child | - | No |

### Integration Points
- Owned by each child.

## Risks & Assumptions

Owned by each child.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Owned by each child | → | - | - |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Both children closed | Each child's ACs demonstrated in its own spec | None |
| AC-2 | Rule edits for items 3 and 4 | Each lands in `ai/rules/` through `ai/rules/rule-format.md`, and `./le ai rules update condensed` regenerates the indexes | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Owned by each child | - | - | |

## Files to Modify
- Owned by each child.

## Implementation Steps

1. Close child 1, then child 2, each in its own session.
2. Land the rule edits for items 3 and 4.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Each child names its targeted runs, for example `./le test functional plugin`
