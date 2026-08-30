---
kind: directive
level: MUST NOT
stage:
---
**These phrases MUST NOT appear in a code comment. Each one names a silent approximation:**

| Banned | Usually means |
|--------|---------------|
| "for now we just truncate" | Silent data loss; reject at verify |
| "close enough approximation" | Not the operator's config; reject |
| "MVP only handles the first N" | Classes beyond N silently missing; reject |
| "best-effort translation" | Pick one: exact, or reject |
| "future optimization can batch them" (when the un-batched path is wrong) | Fix correctness first |
