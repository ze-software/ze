---
name: Implementation workflow cycle
description: The standard work cycle is implement -> review -> fix -> commit -> repeat. Follow this order, do not skip steps.
type: feedback
originSessionId: 12c17d56-79f0-4d8f-9f97-e9ec9e771ad7
---
Standard workflow cycle for spec implementation:
1. `/ze-implement` to pick and start a spec
2. Work on the spec or one of its sections
3. When code is ready to commit: `/ze-review` to catch issues
4. Fix all issues the review finds
5. `/ze-commit` to prepare the commit script
6. Repeat from step 2 for the next section

**Why:** User wants a disciplined review-before-commit loop. Code should never be committed without passing review first.

**How to apply:** After finishing implementation work, always run `/ze-review` before `/ze-commit`. Do not skip the review step or combine it with commit. Fix every issue the review surfaces before moving to commit.
