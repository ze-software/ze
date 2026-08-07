---
kind: table
level:
stage:
---
| Rule | Detail |
|------|--------|
| When to add | Every paragraph with a factual claim (syntax, field names, behavior, data structures) |
| Format | `<!-- source: <relative-path> -- <symbol-or-topic> -->` |
| Placement | After the paragraph or table row containing the claim. **NEVER inside fenced code blocks** (between ` ``` ` delimiters) -- place after the closing fence. Inside code blocks, HTML comments render as visible text. |
| When editing docs | Verify existing anchors still match reality. Fix stale ones |
| When changing code | Check if any doc has an anchor pointing to the changed file. Update if claim is now wrong |
| Granularity | One anchor per factual paragraph or table. Not every sentence, not every file |
