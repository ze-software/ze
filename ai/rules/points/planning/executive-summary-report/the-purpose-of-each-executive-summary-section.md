---
kind: table
level:
stage:
---
| Section | Purpose |
|---------|---------|
| Objective | Confirms alignment. If the goal was misunderstood, this is the last chance before it becomes a commit. |
| Changes | Per-file summary with *why*, not just *what*. `git diff --stat` says "planning.md +8 -5", which is useless. "Added modularity check as step 3, renumbered 4-10" is actionable. |
| Design decisions | Choices made during implementation that weren't explicitly dictated. The user should know what was decided on their behalf. "None, all choices were explicit" is valid. |
| Deviations | What differed from spec/plan/instructions and why. "None" is valid. |
| Not done | Explicit scope boundary. Prevents the assumption that everything related was handled. Surfaces deferred items. |
| Risks & observations | Things that might bite later: new coupling, stale references elsewhere, edge cases not covered, follow-up work needed. Start from the spec's Risks table (R-N rows that survived implementation): this section is a copy-forward, not an invention at the end. |
| Verification | What was run, what passed. Not "make ze-test passes" but actual output or specific test names. |
