---
paths:
  - "*"
---

# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

Only commit when user says "commit". Only include task-related files.
Output format: list staged files, excluded files, commit message, and ask — no git diff/stat output.
Post-commit: hash, file count, clean state confirmation.

## Before Any Commit (BLOCKING)

**`make test-all` (timeout 300s) — not `make ze-verify`, not `go test`, not any subset.**

```
[ ] 1. Run `make test-all` — paste FULL output. ANY failure: STOP.
[ ] 2. Run `git status` + `git diff --stat` — show user what's committed
[ ] 3. Executive Summary Report (rules/planning.md) — present to user
[ ] 4. ASK user: "Ready to commit?" — WAIT for explicit yes
```

Never commit with lint issues. Never commit without pasting output.
`make ze-verify` is for development iterations only — it skips fuzz and exabgp tests.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`, `git stash drop`, `git push --force`

## Before Destructive Actions

Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch` — then ASK user.

## Codeberg CLI

Use `tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`
