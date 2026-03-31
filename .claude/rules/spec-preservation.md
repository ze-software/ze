# Spec Preservation

Rationale: `.claude/rationale/spec-preservation.md`

Completed specs become learned summaries in `plan/learned/NNN-<name>.md`.

**Extract into summary:** Context (problem + goal), decisions (with rejected alternatives), consequences (enables/constrains going forward), gotchas, files changed.
**Discard:** Audit tables, checklists, post-compaction instructions, BLOCKING markers, status columns, template scaffolding.

The original spec in `plan/` is deleted after the summary is written, but the completed spec MUST be committed to git first so it is preserved in history.

**Two-commit sequence (BLOCKING):**
1. **Commit A:** code + tests + docs + completed spec (with filled audit tables). This preserves the spec in git history.
2. **Commit B:** `git rm plan/spec-<name>.md` + add `plan/learned/NNN-<name>.md`. The learned summary replaces the spec.

Never delete the spec without committing it first. A spec that was never committed is lost forever -- its audit tables, verification evidence, and design decisions cannot be reviewed.

Principle: transform scaffolding into knowledge. See `plan/learned/METHODOLOGY.md` for the extraction recipe and `planning.md` for the summary format.
