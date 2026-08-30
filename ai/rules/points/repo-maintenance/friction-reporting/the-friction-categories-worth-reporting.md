---
kind: directive
level: MUST
stage:
---
**A friction report MUST state five things: what happened, why it is likely to recur, what it cost, whether a rule change would prevent it, and the specific fix. Any category below MUST be reported.**

| Category | Examples |
|----------|----------|
| **Problem pattern** | The same mistake, rejected edit, missing wiring, misunderstood boundary, or unexpected failure appears more than once or is likely to recur |
| **Rule gap** | Existing rules did not say what to do, gave conflicting guidance, or made the wrong path look valid |
| **Missing docs** | Had to investigate something a page was owed: file purpose, data flow step, registration pattern, gate behavior |
| **Stale info** | Rule or doc references deleted/renamed files, describes a pattern the code no longer follows |
| **Tooling friction** | Hook rejects valid code, linter config does not match rules, native action behaves unexpectedly |
| **Wasted effort** | Searched in the wrong place, duplicated existing functionality, misunderstood a layer boundary |
