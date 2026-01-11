# Hook Errors

**BLOCKING:** Hook validation errors MUST be fixed before proceeding.

## Rules

1. **Never ignore** `PostToolUse:Edit hook error` or `PostToolUse:Write hook error`
2. **Never claim "complete" or "done"** if hooks failed
3. **Fix immediately** - don't continue with other work
4. **Re-run the edit** after fixing to verify hook passes

## What Hook Errors Mean

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 0 | Success | Continue |
| 1 | Non-blocking failure | Fix before claiming done |
| 2 | Blocking failure | Operation rejected, must fix |

## Common Spec Validation Errors

| Error | Fix |
|-------|-----|
| Missing required section | Add the exact section header (e.g., `## Files to Modify`) |
| Missing checklist item | Add exact text (e.g., `Tests written`, not `Tests written (foo)`) |
| RFC summary not found | Run `/rfc-summarisation` first |
| Table format required | Use `| Col1 | Col2 | Col3 |` format |

## Why This Matters

Hook validation ensures specs follow project standards. Ignoring errors leads to:
- Inconsistent documentation
- Missing required information
- Failed reviews
