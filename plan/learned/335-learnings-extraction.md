# 335 — Learnings Extraction

## Objective

Replace 339 completed specs in `plan/done/` (5.6 MB, 117K lines) with concise knowledge summaries in `plan/learned/` (~556 KB, 9.4K lines). Update the spec completion process so future specs produce summaries directly.

## Decisions

- Fixed 5-section format: Objective, Decisions, Patterns, Gotchas, Files — consistent structure enables scanning
- Three documentation layers: `docs/architecture/` = how system IS (state), `plan/learned/` = what was LEARNED building it (trajectory), `plan/` = active planning
- Standalone METHODOLOGY.md survives after spec deletion as the extraction recipe reference
- Parallel batch extraction via sonnet agents rather than sequential population-based processing — faster for 339 files

## Patterns

- Quality check: "If I deleted this entry, would a future session miss something that code alone cannot tell them?"
- Specs are 60-90% scaffolding; knowledge is in Task, Core Insight, Key Design Decisions, Deviations, and Mistake Log sections
- Commit messages for code+spec commits are often better summaries than the spec itself
- Batch size of ~8 specs per agent is reliable; 24 can exceed context limits for large specs

## Gotchas

- `du -sh` reports filesystem block sizes, not actual content size — use `find -exec cat {} + | wc -c` for accurate measurement
- Two 001 files coexist: the pre-existing foundational planning summary and the extracted reload-test-framework summary
- Reference update scope was larger than expected: 41 files outside `done/` referenced it, spanning rules, rationale, architecture docs, active specs, and contributing guides
- Some agents failed with "Prompt is too long" when batch-processing large specs — needed smaller batches

## Files

- Created: `plan/learned/*.md` (339 summaries + METHODOLOGY.md)
- Deleted: `plan/done/` (339 full specs)
- Updated: `.claude/rules/planning.md`, `implementation-audit.md`, `documentation.md`, `file-modularity.md`, `CLAUDE.md`, `AGENT.md`, `plan/TEMPLATE.md`, `.claude/commands/spec.md`, `.claude/docs/README.md`, `.claude/rationale/` (3 files), `docs/architecture/` (7 files), `plan/spec-*.md` (14 files), `docs/contributing/rfc-implementation-guide.md`
