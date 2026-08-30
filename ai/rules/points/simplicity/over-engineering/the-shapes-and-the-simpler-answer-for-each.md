---
kind: directive
level: MUST
stage:
---
**When your diff carries one of these shapes, you MUST write the simpler answer instead:**

| Shape in the diff | The simpler answer |
|-------------------|--------------------|
| An interface with one implementation | The concrete type. Two use cases earn an abstraction, and the second use case is when you add it (`ai/rules/architecture.md`) |
| A config option or flag nobody asked for | Pick the correct behavior and make it the behavior. An option is a permanent branch, a schema entry, a doc line, and a test matrix (`ai/rules/config.md`) |
| A new generic mechanism built to fix one call site | Fix that call site |
| A parameter added so callers can choose, with one caller | Pass nothing. Add the parameter when a second caller needs a different value |
| A wrapper, adapter, or layer that transforms nothing | Pass the data. An identity wrapper is a rename with a cost (`ai/rules/architecture.md`) |
| A branch for a state the caller cannot produce | Delete the branch. Where the state is reachable, it is an error path and gets an error (`ai/rules/evidence.md`) |
| A retry, cache, pool, or worker added with no measured problem | Do the direct thing. Measure, then add the machinery the measurement asks for |
| A rewrite where a small change restores correctness | Make the small change. A design you believe is wrong is a spec, never an unasked rewrite |
| Scaffolding for a feature that is planned but not commissioned | Write nothing. The commissioned requirement is the only source that can justify the code |
