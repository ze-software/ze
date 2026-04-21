---
name: ze-commit
description: Use when working in the Ze repo and the user asks for ze-commit or for help preparing a scoped commit safely. Verify the intended scope, run the right pre-commit checks, and generate a commit script instead of staging or committing directly.
---

# Ze Commit

This skill is for safe commit preparation in the Ze repo.

## Workflow

1. Inspect `git status` and `git diff --stat` to understand the full change set.
2. Work out the task scope and list any modified files that do not belong to it.
3. If `.go` files are in scope, run `make ze-verify-changed` before preparing the commit.
4. If `.claude/` files are in scope, run a lightweight health check for stale file references, broken skill references, broken `ai/INDEX.md` targets, stale memories, and missing hook scripts.
5. Draft a concise commit message based on the actual changes.
6. Write `tmp/commit-<session>.sh` that performs the required `git add` and `git commit`.
7. Before showing the script, present a `Remaining After This Commit` table that lists omitted ACs, open deferrals, TODOs, excluded files, and known gaps.

## Rules

- Never run `git add` or `git commit` directly.
- If scope is unclear, ask instead of guessing.
- Do not include spec or docs changes unless they are part of the task.
