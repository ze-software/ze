# Session Start Rules

Rationale: `.claude/rationale/session-start.md`

**BLOCKING:** Complete at start of EVERY session.

## TOP 6 RULES

| # | Rule |
|---|------|
| 1 | Read selected spec FIRST (`cat .claude/selected-spec`) |
| 2 | Know source before writing code (use file digests) |
| 3 | No code without understanding (can you name 3 related files?) |
| 4 | TDD: test must FAIL first |
| 5 | Preserve existing behavior (document current format BEFORE changing) |
| 6 | Confirm file paths before editing (Glob/Grep to verify) |

## Checklist

```
[ ] 1. Read .claude/selected-spec
[ ] 2. Read docs/plan/<spec-name> (if selected)
[ ] 3. Read .claude/session-state.md (if exists)
[ ] 4. Check git status
[ ] 5. THEN start working
```
