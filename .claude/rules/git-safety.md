---
paths:
  - "*"
---

# Git Safety

## Commit Rules
- ONLY commit when user explicitly says "commit"
- Run `make test-all` before commit — ALL must pass
- Only include files related to the current task unless explicitly told otherwise
- Always confirm the scope with `git diff --stat` before running `git commit`
- Never include unrelated changes (e.g., spec files when fixing editor bugs)

## Before Any Commit (BLOCKING — NO EXCEPTIONS)

**MANDATORY pre-commit gate.** Complete IN ORDER. Do not skip steps.

```
[ ] 1. Run `make ze-verify` (ze-lint + ze-unit-test + ze-functional-test)
      → Paste FULL output to user
      → If ANY failure: STOP. Fix before continuing. Do NOT commit.
[ ] 2. Run `git status` and `git diff --stat`
      → Show user what will be committed
[ ] 3. ASK user: "Verification passed. Ready to commit these files?"
      → WAIT for explicit "yes" / "commit" before running git commit
```

**BLOCKING:** Never commit with ANY lint issues, even pre-existing ones. Fix lint issues first or ask user for guidance.

**BLOCKING:** Never commit without pasting verification output. "Should pass" is not evidence — run it, paste it, ask.

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

## Codeberg CLI

Use `tea` for Codeberg interactions (PRs, issues):
```bash
tea pr list                      # List PRs
tea pr create --title "..." --description "..."
tea issue list
tea issue create --title "..."
```
