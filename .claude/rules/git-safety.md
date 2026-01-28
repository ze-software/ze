---
paths:
  - "*"
---

# Git Safety

## Commit Rules
- ONLY commit when user explicitly says "commit"
- Run `make test-all` before commit - ALL must pass

## Before Any Commit
```bash
make test-all           # ALL must pass (lint + test + functional + exabgp)
git status              # Review changes
git diff --staged       # Review what's staged
```

**BLOCKING:** Never commit with ANY lint issues, even pre-existing ones. Fix lint issues first or ask user for guidance.

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
