# Simplest Correct Solution

**When:** choosing how to fix a defect or build a feature, and whenever a change adds an abstraction, an option, or a layer the problem in hand does not need
**Severity:** blocking
**Related:** architecture, completion, quality, rule-precedence

## Directives

**A fix MUST be the simplest solution that is fully correct, it MUST sit at the ROOT of the defect, and nothing in the change MUST exist for a problem you were not asked to solve.** A special case bolted onto shared infrastructure is not the simpler option: it adds a branch AND leaves the defect live for every caller the special case does not name.
**"Simplest" is measured in what the next developer holds in their head, never in line count: they meet the code cold and say what it does and where to change it, in about 30 seconds, with no second file open.** A dense expression they have to simulate fails that measure exactly as a five-file framework does, so write the version that is boring to read.

**Simplicity governs the SHAPE of the answer. It MUST NOT govern the EXTENT of its correctness.** Two readings, and the second governs: the first reads "simplest" as permission to do less, with fewer acceptance criteria, fewer RFC MUSTs and a narrower test; the second reads it as the instruction to solve the whole problem with the least machinery.
**The only budget this rule cuts is machinery. It MUST NOT cut correctness, conformance, tests, guards, or error handling, so it is never the reason for a `may I skip it` question, a deferral row, or a partial implementation.**
**The simplest fully correct design is usually the HARDEST one to find, so you MUST budget thinking time for it, and not seeing it MUST NOT be read as a license to ship the complicated shape and call it pragmatic.** When you ship anything other than the most obvious implementation, write one line naming the simpler design and the requirement it failed: an unexplained abstraction reads as habit, and the next reader keeps it because they cannot prove it is unnecessary.

**When your diff carries one of these shapes, you MUST write the simpler answer instead:**

| Shape in the diff | The simpler answer |
|-------------------|--------------------|
| An interface with one implementation, a parameter with one caller, or a generic mechanism built for one call site | The concrete type, no parameter, and a fix at that call site. The second use case is when you add it |
| A config option or flag nobody asked for | Pick the correct behavior and make it the behavior. An option is a permanent branch, a schema entry, a doc line and a test matrix |
| A wrapper that transforms nothing, a branch for a state the caller cannot produce, or scaffolding for uncommissioned work | Pass the data, delete the branch, write nothing. Where the state is reachable it is an error path and gets an error |
| A retry, cache, pool or worker with no measured problem, or a rewrite where a small change restores correctness | Do the direct thing and measure. A design you believe is wrong is a spec, never an unasked rewrite |

**Complexity another rule REQUIRES is not over-engineering, this rule MUST NOT license its removal, and you MUST name the rule that requires it.** Buffer-first encoding is more complex than the naive version on purpose, and a guard fails closed even when the open version is shorter. When no rule requires the machinery and no second use case exists, it is yours to justify or delete.
