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
| Updated | 2026-11-11 |

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
| 1 | The MCP tool catalog is measured as context cost on real agent clients, compatibility profiles keep it loadable, and per-client lanes prove a real client loads it | REA `docs/mcp-catalog-evaluation.md`, `docs/development/client-verification.md`, `docs/reference/mcp-contracts.md` "Compact input schema profile" | Every operator command reaches MCP as a derived tool (`ai/META-DECISIONS.md`), and nothing measures what the catalog costs an agent or tests a real client | Done: `spec-rea-practices-1-mcp-catalog-cost`, closed 2026-11-11; `le test mcp` verb `tools-size`, `test/plugin/mcp-tools-list-structural-limits.ci`, and the owner-run lane `docs/contributing/mcp-client-verification.md` |
| 2 | An inventory of every user-visible error, ranked by harm, with current copy, proposed copy, status and verification | REA `docs/user-visible-errors.csv` (126 rows) | `ai/rules/user-facing-errors.md` states the rule; nothing measures which messages meet it | Done: `spec-rea-practices-2-error-inventory`, closed 2026-11-10; `./le arch error-inventory report` and `check` (`docs/contributing/error-inventory.md`) |
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
| `spec-rea-practices-1-mcp-catalog-cost` (closed 2026-11-11) | Item 1 |
| `spec-rea-practices-2-error-inventory` (closed 2026-11-10) | Item 2 |

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

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Child 1 closed | `git log --oneline -- plan/post-release/features/spec-rea-practices-1-mcp-catalog-cost.md` shows the removal commit 264db5ca6a |
| Child 2 closed | `git log --oneline -- plan/post-release/engineering/spec-rea-practices-2-error-inventory.md` shows the removal commit 3e6af8384a |
| Item 3 rule edit | `grep -n "Lifecycle Regressions" ai/rules/testing.md`; `ls ai/rules/points/testing/lifecycle-regressions/` |
| Item 4 rule edit | `ls ai/rules/points/writing/documentation/fix-the-tool-output-before-writing-advice-about-it.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| No code of its own | The umbrella's closure commit carries prose only: this spec and one handover citation. Each child ran its own security review over its own code |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Any feature, config, CLI, API, plugin, wire, SDK, RFC, test-infrastructure, comparison, architecture, metadata, metric, inventory or example change? | No: this umbrella changes no source file. Each child carried its own page edits (child 1: a6199d11d8, b619cc8edd; child 2: fdd671e32f, 0866eb06f6, f35639c2fe) | - |
| 16 | Changed source files referenced by doc anchors | No: `./le spec citation anchors spec plan/post-release/engineering/spec-rea-practices-0-umbrella.md` lists none | - |

## Implementation Summary

### What Was Implemented
- Nothing in this spec: the umbrella owns no code. Its four practices landed
  through two child specs and one rule commit, listed under Implementation
  Audit.

### Bugs Found/Fixed
- None in this closure.

### Documentation Updates
- None by this closure. The children's pages: `docs/contributing/mcp-client-verification.md`,
  `docs/architecture/mcp/overview.md`, `docs/functional-tests.md` (child 1, a6199d11d8, b619cc8edd);
  `docs/contributing/error-inventory.md`, `docs/architecture/cli/error-surface.md` (child 2, fdd671e32f, 0866eb06f6, f35639c2fe).
- `plan/handover/36-rea-practices-2-and-doctor-children.md` now cites this spec by bare stem, so commit B leaves no dangling path.

### Deviations from Plan
- Child 1's lane round recorded Codex 0.167.1 as not yet supported: it opens
  with the `2025-06-18` revision's `initialize`, which Ze refuses with `-32020`.
  Ze speaks only the current MCP revision by owner decision
  (`ai/META-DECISIONS.md`, 8ad50e88b9), so this is a recorded result, not
  unfinished work. Claude Code 2.1.322 connected and called `ze_reference`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Item 1: MCP catalog measured as context cost, real-client lane | Done | `internal/test/cli/cmd_mcp_calls.go` `measureToolsSize` (verb `tools-size`), `test/plugin/mcp-tools-list-structural-limits.ci`, `docs/contributing/mcp-client-verification.md` "Results" | a6199d11d8, b619cc8edd, 264db5ca6a |
| Item 2: user-visible error inventory | Done | `internal/le/arch/errorinventory/actions.go` `runReport`, `runCheck`; `docs/contributing/error-inventory.md` | fdd671e32f, 0866eb06f6, 10c6647f6d, f35639c2fe, 3e6af8384a |
| Item 3: lifecycle regression scenario list | Done | `ai/rules/testing.md` "Lifecycle Regressions"; `ai/rules/points/testing/lifecycle-regressions/` (two points); trigger row in `ai/rules/TRIGGERS.md` | ad28186f0b |
| Item 4: fix the tool's result before writing advice | Done | `ai/rules/points/writing/documentation/fix-the-tool-output-before-writing-advice-about-it.md`, `ai/rules/writing.md` | ad28186f0b |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Child 1 removed in 264db5ca6a (closure record b619cc8edd); child 2 removed in 3e6af8384a (closure record f35639c2fe). Each child's ACs are demonstrated in its own closure record | |
| AC-2 | Done | ad28186f0b adds both points through their manifests (`ai/rules/points/testing/manifest.md`, `ai/rules/points/writing/manifest.md`) and the regenerated `ai/rules/TRIGGERS.md` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Owned by each child | Done | Child 1: `internal/component/mcp/tools_test.go`, `internal/test/cli/cmd_mcp_test.go`, `test/plugin/mcp-tools-list-structural-limits.ci`. Child 2: the `internal/le/arch/errorinventory` package tests | Run by each child's own closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Owned by each child | Done | See Requirements from Task |

### Audit Summary
- **Total items:** 4 requirements, 2 ACs
- **Done:** 6
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Item 1: what the MCP catalog costs an agent is measured, and a real client is shown to load it | functional test plus a recorded real-client run | `test/plugin/mcp-tools-list-structural-limits.ci` runs `tools-size` (`measureToolsSize`) against a live daemon and asserts its summary line; `docs/contributing/mcp-client-verification.md` "Results" records Claude Code 2.1.322 connecting and calling `ze_reference` at d54128e6f8, and Codex 0.167.1 refused for its older revision |
| Item 2: which user-visible messages meet `ai/rules/user-facing-errors.md` is measured | tool plus commit gate | `./le arch error-inventory report` lists every message site (`runReport`); `./le arch error-inventory check` refuses an added message site that names no fix (`runCheck`, 0866eb06f6); page `docs/contributing/error-inventory.md` |
| Item 3: a lifecycle change selects its regression scenarios from a list | rule | `ai/rules/testing.md` "Lifecycle Regressions"; the testing trigger in `ai/rules/TRIGGERS.md` names plugin supervision, config reload and daemon stop, so an agent changing them is routed to the list |
| Item 4: a misleading tool output is fixed, not explained around | rule | Point `fix-the-tool-output-before-writing-advice-about-it`, which covers CLI answers, MCP tool results, `./le` diagnostics, hook refusals and `ze doctor` reports |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every item of the Task table is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` over this spec and the handover file (path in the closure report) |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | closure record against the tree and git log (every SHA, path and symbol re-read), citation integrity for commit B |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none | - | - |

### Run 1
0 BLOCKER, 0 ISSUE. The change set is this spec and one handover line; it
carries no Go, YANG or templ, so the style pass has nothing to judge.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/mcp-tools-list-structural-limits.ci` | yes | `ls` 2026-11-11; line 21 runs `tools-size` |
| `docs/contributing/mcp-client-verification.md` | yes | `ls` 2026-11-11; "Results" carries the Claude Code and Codex rows |
| `docs/contributing/error-inventory.md` | yes | `ls` 2026-11-11 |
| `ai/rules/points/testing/lifecycle-regressions/` | yes | `ls`: `select-every-lifecycle-scenario-the-change-reaches.md`, `the-lifecycle-scenarios-and-where-each-outcome-is-declared.md` |
| `ai/rules/points/writing/documentation/fix-the-tool-output-before-writing-advice-about-it.md` | yes | `git ls-files` 2026-11-11 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Both children closed | `git show --name-only 264db5ca6a 3e6af8384a` deletes each child spec; `ls plan/*/*/spec-rea-practices-*` lists only this umbrella |
| AC-2 | Rule edits landed through the point format | `grep -n "Lifecycle Regressions" ai/rules/testing.md` line 49; `grep fix-the-tool-output ai/rules/points/writing/manifest.md` line 13; ad28186f0b touches `ai/rules/TRIGGERS.md` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `tools-size` over a live daemon (child 1) | `test/plugin/mcp-tools-list-structural-limits.ci` | read: line 21 sends `tools-size`, line 53 asserts the summary line; run green in child 1's closure (b619cc8edd) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none | confirmed | The umbrella declares no A-N; each child resolved its own |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No page changes owed by the umbrella | `./le spec citation anchors` lists none; no source file in the change set | yes |
| Handover 36 citation | `grep -rn spec-rea-practices-0-umbrella plan/ docs/` shows only the bare stem after this edit | yes |
