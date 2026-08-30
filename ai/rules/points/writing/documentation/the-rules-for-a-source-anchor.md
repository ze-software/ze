---
kind: directive
level: MUST
stage:
---
**A source anchor ties a factual claim to the code that produces it, and every rule below MUST be met:**

| Rule | Detail |
|------|--------|
| When to add | Every paragraph with a factual claim: syntax, field names, behavior, data structures |
| Format | `<!-- source: <relative-path> -- <symbol-or-topic> -->`, for example `<!-- source: internal/component/bgp/reactor/forward_pool.go -- ForwardPool -->` |
| Placement | After the paragraph or table row carrying the claim. NEVER inside a fenced code block: place it after the closing fence, because an HTML comment renders as visible text inside one |
| When editing docs | Verify the existing anchors still match reality, and fix a stale one |
| When changing code | Check whether any doc has an anchor pointing at the changed file, and update it when the claim is now wrong |
| Granularity | One anchor per factual paragraph or table. Not every sentence, and not every file |

Anchors are invisible in rendered markdown, and they are what lets a later session verify the page.
