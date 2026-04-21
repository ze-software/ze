---
name: ze-audit
description: Use when working in the Ze repo and the user asks for ze-audit or a pre-implementation audit of the selected spec. Read the current spec, compare every requirement, planned file, and planned test against the codebase, and report what is already done, partial, or missing without making changes.
---

# Ze Audit

This skill is repo-specific. If `CLAUDE.md` or `ai/INDEX.md` are missing, say the workflow only applies to the Ze repository and stop.

## Workflow

1. Read `tmp/session/selected-spec`, then open the matching file under `plan/`.
2. Extract the task, acceptance criteria, TDD plan, file lists, and wiring checks.
3. Search code, tests, docs, and recent git history for evidence of each item.
4. Report a table with `requirement | status | evidence | note`, using `Done`, `Partial`, or `Missing`.
5. End with counts and the best implementation order based on dependencies.

## Rules

- Read-only.
- Prefer exact evidence with `file:line`.
- Do not implement or fix anything during the audit.
