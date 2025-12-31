---
paths:
  - "*"
---

# Git Safety

## Commit Rules
- ONLY commit when user explicitly says "commit"
- Run `make test && make lint` before commit - ALL must pass
- Update `plan/CLAUDE_CONTINUATION.md` after commit

## Before Any Commit
```bash
make test && make lint  # Must pass
git status              # Review changes
git diff --staged       # Review what's staged
```

## Forbidden Without Explicit Permission
- `git reset` (any form)
- `git revert`
- `git checkout -- <file>`
- `git restore` (to discard changes)
- `git stash drop`
- `git push --force`

## Before Destructive Actions
Save first:
```bash
git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch
```
Then ASK user: "May I run `git reset`? This will discard changes."

## Work Preservation
If tests fail or approach isn't working:
1. Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`
2. ASK user: "Tests failing. Options: (a) keep debugging, (b) save and try different approach, (c) revert?"
3. WAIT for response before any destructive action
