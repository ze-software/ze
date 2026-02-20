# Git Safety Rationale

Why: `.claude/rules/git-safety.md`

## Why Pre-Commit Gate Exists
"Should pass" is not evidence. Run it, paste it, ask. Never commit with ANY lint issues, even pre-existing ones.

## Work Preservation Procedure
When tests fail or approach isn't working:
1. Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`
2. ASK user: "Tests failing. Options: (a) keep debugging, (b) save and try different approach, (c) revert?"
3. WAIT for response before any destructive action

## Scope Discipline
- Only include files related to the current task unless explicitly told otherwise
- Always confirm scope with `git diff --stat` before running `git commit`
- Never include unrelated changes (e.g., spec files when fixing editor bugs)

## Codeberg CLI Examples
```bash
tea pr list
tea pr create --title "..." --description "..."
tea issue list
tea issue create --title "..."
```
