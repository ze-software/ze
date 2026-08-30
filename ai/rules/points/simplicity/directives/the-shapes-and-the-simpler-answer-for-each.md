---
kind: directive
level: MUST
stage:
---
**When your diff carries one of these shapes, you MUST write the simpler answer instead:**

| Shape in the diff | The simpler answer |
|-------------------|--------------------|
| An interface with one implementation, a parameter with one caller, or a generic mechanism built for one call site | The concrete type, no parameter, and a fix at that call site. The second use case is when you add it |
| A config option or flag nobody asked for | Pick the correct behavior and make it the behavior. An option is a permanent branch, a schema entry, a doc line and a test matrix |
| A wrapper that transforms nothing, a branch for a state the caller cannot produce, or scaffolding for uncommissioned work | Pass the data, delete the branch, write nothing. Where the state is reachable it is an error path and gets an error |
| A retry, cache, pool or worker with no measured problem, or a rewrite where a small change restores correctness | Do the direct thing and measure. A design you believe is wrong is a spec, never an unasked rewrite |
