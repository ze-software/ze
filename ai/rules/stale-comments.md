# Stale Comments

**When:** when changing code behavior
**Severity:** blocking

## Directives

**When you change code behavior, you MUST update or remove the comments that described the old behavior.** A comment that no longer matches the code is worse than no comment.

## Checklist

**Each change below MUST carry its comment update:**

| Change | Action |
|--------|--------|
| Function signature changes (return type, params) | Update all doc comments on the function |
| Control flow changes (new branch, removed path) | Update inline comments describing the flow |
| Error handling changes | Update comments explaining error propagation |
| Callers change behavior | Update comments at the call site |

## Do Not

- MUST NOT leave a comment that describes one specific case when the code now handles multiple cases.
- MUST NOT keep a comment about "returns X" when the function now returns Y.
- MUST NOT add "also does Z" to an existing comment that says "does X". MUST rewrite to cover both.
