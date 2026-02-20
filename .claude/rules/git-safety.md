---
paths:
  - "*"
---

# Git Safety

Rationale: `.claude/rationale/git-safety.md`

## Commit Rules

Only commit when user says "commit". Only include task-related files.

## Before Any Commit (BLOCKING)

```
[ ] 1. Run `make ze-verify` — paste FULL output. If ANY failure: STOP.
[ ] 2. Run `git status` + `git diff --stat` — show user what's committed
[ ] 3. ASK user: "Ready to commit?" — WAIT for explicit yes
```

Never commit with lint issues. Never commit without pasting output.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`, `git stash drop`, `git push --force`

## Before Destructive Actions

Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`
Then ASK user.

## Codeberg CLI

Use `tea` for PRs, issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`
