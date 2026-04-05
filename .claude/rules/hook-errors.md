# Hook Errors

**BLOCKING:** Fix hook validation errors before proceeding.
Rationale: `.claude/rationale/hook-errors.md`

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Continue |
| 1 | Non-blocking failure | Fix before claiming done |
| 2 | Blocking failure | Operation rejected, must fix |

Never ignore hook errors. Never claim "done" if hooks failed. Re-run after fixing.

| Common Error | Fix |
|-------------|-----|
| Missing required section | Add exact section header |
| Missing checklist item | Add exact text |
| RFC summary not found | Run `/ze-rfc` first |
| Table format required | Use `\| Col \| Col \|` format |
