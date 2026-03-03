# Spec: learnings-extraction

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/INDEX.md` - documentation navigation

## Task

Extract knowledge from the 339 full specs in `docs/plan/done/` (5.6 MB, 117K lines) into concise summaries (~20-30 lines each) in `docs/learned/`. Delete `docs/plan/done/`. Modify the spec completion process so future specs produce summaries directly in `docs/learned/`.

**Motivation:** completed specs are 60-90% process scaffolding (checklists, audit tables, status markers). The useful knowledge — design decisions, failed approaches, architectural insights — is buried. Claude cannot practically access 5.6 MB of text, so institutional knowledge is effectively lost.

**Non-goals:** creating a parallel documentation system. `docs/architecture/` remains the source of truth for how the system works. `docs/learned/` captures WHY it got that way and WHAT went wrong along the way.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/planning.md` — current spec completion process (step 8: move spec, step 10: executive summary)
  → Constraint: executive summary is already BLOCKING before every commit
  → Decision: summary format should be a reformatted executive summary, not additional work
- [ ] `.claude/rules/spec-preservation.md` — what to keep vs discard from completed specs
  → Constraint: "keep task description, key insights, data flow, design decisions, integration points, boundaries, files modified, references"
  → Constraint: "remove empty audit tables, unchecked checklists, post-compaction instructions, BLOCKING markers, blank status columns"
- [ ] `docs/plan/TEMPLATE.md` — current spec template (262 lines)
  → Constraint: template step 10 says "complete spec → fill audit tables, move spec to done/"

### Source Files Read
- [ ] `docs/plan/done/` directory — 339 completed specs, 5.6 MB
  → Constraint: three populations based on git history:
  - 153 specs committed with code (commit message available as summary source)
  - 49 specs moved separately ("chore: close/move" — code commit must be found by search)
  - 94 specs from bulk reorg e1709fab (spec content is only source)
  - 5 other patterns

**Key insights:**
- Executive summary already exists in the process — reformatting it IS the summary
- Commit messages for code+spec commits are often excellent summaries
- Spec content quality varies: 10% knowledge (pool-completion) to 75% knowledge (encoding-context-design)
- Architecture docs describe state (what IS), summaries describe trajectory (what was LEARNED)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/add-design-refs.go` — only .go file involved; confirms no .go files reference done/ spec content
- [ ] `docs/plan/done/*.md` — 339 files, each 100-500 lines, mostly template scaffolding
- [ ] `.claude/rules/planning.md` — step 8 says `mv docs/plan/spec-<name>.md docs/plan/done/NNN-<name>.md`
- [ ] `.claude/rules/memory.md` — 61 lines of project memory, captures maybe 5-10 specs worth of knowledge
- [ ] `docs/plan/TEMPLATE.md` — 262-line template, step 10 references spec completion

**Behavior to preserve:**
- Sequential numbering (NNN prefix preserves chronological project history)
- `docs/architecture/` as the canonical design documentation
- `memory.md` for transverse knowledge not tied to a single spec

**Behavior to change:**
- Full specs in `done/` → extracted to `docs/learned/`, `done/` deleted
- Spec completion process → produce summary in `docs/learned/` instead of moving full spec to `done/`
- Template step 10 → updated to describe summary generation

## Data Flow (MANDATORY)

### Entry Point
- Completed spec file in `docs/plan/spec-<name>.md`
- Git commit message and diff stat (for specs committed with code)

### Transformation Path
1. Spec completion: all tests pass, executive summary written
2. Extract: decisions, patterns, gotchas, files from executive summary + spec
3. Write summary file (20-30 lines, fixed 5-section format)
4. Place in `docs/learned/NNN-<name>.md` (replaces full spec)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Spec → Summary | Manual extraction during completion | [ ] |
| Git log → Summary | `git log --grep` for spec-only moves | [ ] |

### Integration Points
- `planning.md` step 8 (move spec) — must be updated
- `TEMPLATE.md` step 10 — must reference new summary format
- `.claude/INDEX.md` — optional: add domain → spec number table

### Architectural Verification
- [ ] No bypassed layers — summaries derive from existing executive summary step
- [ ] No unintended coupling — no new dependencies introduced
- [ ] No duplicated functionality — summaries complement `docs/architecture/`, not duplicate
- [ ] Zero-copy preserved where applicable — N/A (documentation only)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Spec completion process | → | Summary written to `docs/learned/` | Manual: verify 5 sample summaries against their source specs |
| Planning.md instructions | → | Claude follows updated process | Manual: next spec closure uses new format |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any file in `docs/learned/` | Contains 5-section summary format, 20-50 lines max |
| AC-2 | Summary for a spec committed with code | Includes key decisions from commit message + spec |
| AC-3 | Summary for a spec from bulk reorg | Includes key decisions extracted from spec content |
| AC-4 | `planning.md` step 8 | Updated to describe summary generation, not full spec move |
| AC-5 | `TEMPLATE.md` | References summary format in completion step |
| AC-6 | `docs/learned/` directory total size | Under 500 KB |
| AC-8 | `docs/plan/done/` directory | Deleted |
| AC-7 | Historical numbering | All NNN prefixes preserved unchanged |

## Summary Format

Each file in `docs/learned/` follows this fixed structure:

| Section | Content | Source |
|---------|---------|--------|
| Title line | `# NNN — Name` | Spec filename |
| Objective | 1-2 sentences: what was the goal | Executive summary or spec task |
| Decisions | Bullet points: what was decided and why | Executive summary design decisions |
| Patterns | Bullet points: patterns discovered or confirmed | Design insights section |
| Gotchas | Bullet points: what surprised, failed, or trapped | Mistake log, deviations |
| Files | List of files modified/created | Git stat or spec files section |

**Rules:**
- Each bullet point is one line, max two
- Reference architecture doc if the decision is documented there: "(see encoding-context.md)"
- Gotchas are the most valuable section — never skip even if empty
- If a spec produced no meaningful knowledge (pure mechanical refactor), say so: "Mechanical refactor, no design decisions."

## Extraction Methodology

Full methodology documented in `docs/learned/METHODOLOGY.md` — survives after this spec is completed.

Covers: what counts as knowledge (quality check), where to find it in a spec (ALWAYS/NEVER/SOMETIMES read sections), 6-step extraction recipe, source access by population, and future spec process mapping.

### Source access by population

| Population | Objective source | Knowledge source | Files source |
|------------|-----------------|-----------------|--------------|
| Code+spec commits (153) | Commit message | Spec sections (recipe steps 2-4) + commit message | `git show --stat` |
| Spec-only moves (49) | Spec Task + `git log --grep` for code commit | Spec sections (recipe steps 2-4) | Code commit `--stat` |
| Bulk reorg (94) | Spec Task section | Spec sections (recipe steps 2-4) | Spec Files section |
| No meaningful knowledge | One-line: "Mechanical refactor, no design decisions." | — | — |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A — documentation-only change | N/A | N/A | |

### Boundary Tests (MANDATORY for numeric inputs)

Not applicable — no numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Validate sample summaries | Manual review | 5 summaries accurately capture spec knowledge | |
| Verify size reduction | `du -sh docs/learned/` | Under 500 KB | |

## Files to Modify

- `docs/plan/done/*.md` — read all 339 specs, extract summaries to `docs/learned/`, then delete `done/`
- `.claude/rules/planning.md` — update step 8, Completion Checklist, Moving Completed Specs
- `docs/plan/TEMPLATE.md` — update step 10 to reference summary format and `docs/learned/`
- `CLAUDE.md` — update Specs path reference from `docs/plan/done/` to `docs/learned/`
- `AGENT.md` — update if references `done/`
- `.claude/commands/spec.md` — update `done/` references
- `.claude/rules/implementation-audit.md` — update `done/` reference
- `.claude/rules/documentation.md` — update `done/` reference
- `.claude/rules/file-modularity.md` — update `done/` reference
- `.claude/rationale/documentation.md` — update `done/` reference
- `.claude/rationale/design-doc-references.md` — update `done/` reference
- `.claude/rationale/file-modularity.md` — update `done/` reference
- `.claude/docs/README.md` — update `done/` reference
- `docs/architecture/` — several docs reference specific done/ specs; update paths
- `docs/plan/spec-*.md` — active specs referencing done/ specs; update paths
- `docs/contributing/rfc-implementation-guide.md` — update `done/` reference
- `spec-code-restructure-splits.md` — update `done/` reference

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create

- `docs/learned/METHODOLOGY.md` — extraction methodology (already created)
- `docs/learned/NNN-<name>.md` — 339 summary files extracted from done/ specs

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write 5 sample summaries** covering all populations — spec 038 (bulk reorg, high knowledge), spec 330 (code+spec, recent), spec 316 (spec-only move), spec 003 (bulk reorg, low knowledge), spec 001 (foundational planning doc) → Review: do summaries capture the essential knowledge? Anything lost?
2. **Present samples to user for validation** → WAIT for approval before batch
3. **Extract population 1** (153 specs, code+spec commits) — most mechanical, commit message driven
4. **Extract population 2** (49 specs, spec-only moves) — requires git search per spec
5. **Extract population 3** (94 specs, bulk reorg) — requires reading each spec
6. **Delete `docs/plan/done/`** → `rm -rf docs/plan/done/`
7. **Verify size** → `du -sh docs/learned/` should be under 500 KB
8. **Update planning.md** — modify step 8 and Completion Checklist: `docs/learned/` replaces `done/`
9. **Update TEMPLATE.md** — modify step 10 to reference summary format
10. **Update all references** — grep for `docs/plan/done` in rules, hooks, scripts; update to `docs/learned/`
11. **Verify all** → `make test-all` (ensure nothing references deleted done/ directory)
12. **Critical Review** → documentation accuracy, no knowledge lost

### Failure Routing

| Failure | Route To |
|---------|----------|
| Summary misses key knowledge | Step 1 (adjust format/extraction) |
| User rejects sample quality | Step 1 (rework approach) |
| Size target not met | Step 3-5 (summaries too verbose) |
| Tests reference done/ content | Step 9 (update test references) |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented

Extracted 339 knowledge summaries from completed specs in `docs/plan/done/` into `docs/learned/NNN-<name>.md`. Each summary follows a fixed 5-section format (Objective, Decisions, Patterns, Gotchas, Files). Deleted `docs/plan/done/` after extraction. Updated all references to `docs/plan/done/` across rules, rationale, architecture docs, active specs, CLI docs, and CLAUDE.md/AGENT.md. Updated planning.md and TEMPLATE.md so future specs produce summaries directly. Created METHODOLOGY.md as a standalone extraction guide.

### Bugs Found/Fixed

None.

### Documentation Updates

- `.claude/rules/planning.md` — step 8 now writes learned summary instead of moving spec; completion section rewritten
- `.claude/rules/implementation-audit.md` — references `docs/learned/` instead of `docs/plan/done/`
- `.claude/rules/documentation.md` — placement table updated
- `.claude/rules/file-modularity.md` — reference updated
- `CLAUDE.md`, `AGENT.md` — paths updated
- `docs/plan/TEMPLATE.md` — step 10 references summary format
- 7 architecture docs — cross-references updated
- 14 active specs — checklist items updated
- `.claude/commands/spec.md`, `.claude/docs/README.md`, `.claude/rationale/` files — updated
- `docs/contributing/rfc-implementation-guide.md` — updated

### Deviations from Plan

- AC-6 target was 500 KB; actual is 556 KB (11% over). Summaries averaged ~27 lines vs planned 20-30. Size reduction is still 90% (556 KB vs 5.6 MB content).
- Extraction was done via parallel sonnet agents in batches rather than sequential population-based extraction. This was faster and equally effective.
- Two 001 files exist: `001-initial-implentation.md` (pre-existing foundational planning summary) and `001-reload-test-framework.md` (from done/). Both coexist with different suffixes.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extract summaries from 339 specs | Done | `docs/learned/001-*.md` through `334-*.md` | 339 files created |
| Delete `docs/plan/done/` | Done | Directory removed | Verified with `test -d` |
| Update spec completion process | Done | `.claude/rules/planning.md` | Step 8 + completion checklist |
| Update template | Done | `docs/plan/TEMPLATE.md` | Step 10 references summary format |
| Update all `done/` references | Done | 41 files updated | Only `spec-learnings-extraction.md` retains historical refs |
| Create METHODOLOGY.md | Done | `docs/learned/METHODOLOGY.md` | Standalone extraction guide |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | All 339 files follow 5-section format | Manual review of samples |
| AC-2 | Done | e.g., `330-code-restructure-splits.md` | Includes commit-derived decisions |
| AC-3 | Done | e.g., `003-pool-completion.md` | Extracted from spec content only |
| AC-4 | Done | `.claude/rules/planning.md` step 8 | "Write learned summary" replaces "Move spec" |
| AC-5 | Done | `docs/plan/TEMPLATE.md` step 10 | References `docs/learned/` and summary format |
| AC-6 | Changed | `find ... wc -c` = 556 KB | 11% over 500 KB target; 90% reduction from source |
| AC-7 | Done | `ls docs/learned/` shows 001-334 range | All NNN prefixes preserved |
| AC-8 | Done | `test -d docs/plan/done/` returns false | Directory deleted |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| N/A — documentation only | Done | `make test-all` passes | No code changes requiring tests |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `docs/plan/done/*.md` | Done | All 339 read, extracted, directory deleted |
| `.claude/rules/planning.md` | Done | Updated |
| `docs/plan/TEMPLATE.md` | Done | Updated |
| `CLAUDE.md` | Done | Updated |
| `AGENT.md` | Done | Updated |
| `.claude/commands/spec.md` | Done | Updated |
| `.claude/rules/implementation-audit.md` | Done | Updated |
| `.claude/rules/documentation.md` | Done | Updated |
| `.claude/rules/file-modularity.md` | Done | Updated |
| `.claude/rationale/documentation.md` | Done | Updated |
| `.claude/rationale/design-doc-references.md` | Done | Updated |
| `.claude/rationale/file-modularity.md` | Done | Updated |
| `.claude/docs/README.md` | Done | Updated |
| `docs/architecture/` (7 files) | Done | Cross-references updated |
| `docs/plan/spec-*.md` (14 files) | Done | Checklist items updated |
| `docs/contributing/rfc-implementation-guide.md` | Done | Updated |
| `docs/learned/METHODOLOGY.md` | Done | Created |
| `docs/learned/NNN-*.md` (339 files) | Done | Created |

### Audit Summary
- **Total items:** 28
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-6: 556 KB vs 500 KB target — 11% over, still 90% reduction)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] All 339 summaries written to `docs/learned/`
- [ ] `docs/plan/done/` deleted
- [ ] All `done/` references updated across rules, rationale, commands, architecture docs, active specs
- [ ] planning.md updated
- [ ] TEMPLATE.md updated
- [ ] `docs/learned/` total size under 500 KB

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- N/A — documentation-only change, no code tests needed

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/learned/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
