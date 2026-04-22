# Stale Comments

**BLOCKING:** When changing code behavior, update or remove comments that
described the old behavior. A comment that no longer matches the code is
worse than no comment.

## Checklist

| Change | Action |
|--------|--------|
| Function signature changes (return type, params) | Update all doc comments on the function |
| Control flow changes (new branch, removed path) | Update inline comments describing the flow |
| Error handling changes | Update comments explaining error propagation |
| Callers change behavior | Update comments at the call site |

## Do Not

- Leave a comment that describes one specific case when the code now handles multiple cases.
- Keep a comment about "returns X" when the function now returns Y.
- Add "also does Z" to an existing comment that says "does X". Rewrite to cover both.
