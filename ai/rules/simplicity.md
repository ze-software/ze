# Simplest Correct Solution

**When:** choosing how to fix a defect or build a feature, and whenever a change adds an abstraction, an option, or a layer the problem in hand does not need
**Severity:** blocking
**Related:** architecture, completion, quality, rule-precedence

## Directives

**A fix MUST be the simplest solution that is fully correct. Nothing in the change exists for a problem you were not asked to solve.**
**"Simplest" is measured in what the next developer needs to hold in their head to change this code safely. It is not measured in line count, and a change that removes lines by making the reader work harder has failed the measure.**

**Simplicity governs the SHAPE of the answer. It MUST NOT govern the EXTENT of its correctness.**
**Two readings, and the second one governs. The first reads "simplest" as permission to do less: fewer acceptance criteria, fewer RFC MUSTs, fewer cases handled, a narrower test. The second reads it as the instruction to solve the whole problem with the least machinery. Cutting correctness to reach a smaller diff is scope reduction, banned at rung 3 of `ai/rules/rule-precedence.md`, and an RFC MUST is owed in full at rung 2.**
**Quality is 0% compromise. The only budget this rule cuts is machinery. It MUST NOT cut correctness, conformance, tests, guards, or error handling.**
**So this rule is never the reason for a `may I skip it` question, a deferral row, or a partial implementation. It is a reason to delete machinery, never a reason to delete behavior.**

**The simplest fully correct design is usually the HARDEST one to find. You MUST budget thinking time for it. The first shape that works is rarely the simplest one, and a large diff is more often the cheap answer than the good one.**
**When you cannot see the simple design, that is the signal to think longer, to read more of the existing code, or to ask. You MUST NOT treat this as a license to ship the complicated one and call it pragmatic.**
**If you ship a shape you are not happy with, you MUST name the simpler shape you looked for and say what stops it.**

**MVP means the smallest COMPLETE answer to the problem in hand. "Minimum" qualifies the machinery. "Viable" means every case the problem covers works, and every acceptance criterion MUST have code and a test.**
**A partial answer is not an MVP. It is unfinished work, and `ai/rules/completion.md` governs it.**

**The simplest fully correct fix MUST be at the ROOT of the defect. A special case bolted onto shared infrastructure is not the simpler option: it adds a branch AND leaves the defect live for every caller the special case does not name.**
**Depth and size point the same way here. A one-line fix at the root beats a guard at three call sites, and the `/ze-review` altitude check reports the guard as the finding.**

**A second problem you see while you fix the first MUST get its OWN spec. It MUST NOT become an extra branch, flag, parameter, or abstraction folded into this fix.**
**Other problems MUST NOT be ignored, and the route is already fixed: you MUST write the spec, close the work in hand, ask Thomas whether that spec runs, and stop (`ai/rules/completion.md`). Generalizing this fix "so it covers that too" is the version of the same failure that leaves no spec behind.**

**The next-developer test is the acceptance test for this rule. A developer who meets the code cold MUST be able to say what it does and where to change it, in about 30 seconds, with no second file open.**
**A change that fails that test is not finished.**

**When you choose anything other than the most obvious implementation, you MUST write one line saying what the simpler design was and which requirement it failed. You MUST put it in the spec when a spec exists, and in the code comment when it does not.**
**An unexplained abstraction reads as habit, and the next reader cannot tell habit from a requirement. That reader then keeps the machinery because they cannot prove it is unnecessary.**

## What Over-Engineering Looks Like

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
| Scaffolding for a feature that is planned but not commissioned | Write nothing. `c_yagni` in `.claude/hooks/pretool-writeedit.py` refuses the comment form of this |

## Simple Is Not Less

**Complexity another rule REQUIRES is not over-engineering, and this rule MUST NOT license its removal.** Buffer-first encoding, pool dedup, and zero-copy forwarding are more complex than the naive version on purpose (`ai/rules/performance.md`). An RFC MUST is owed in full (`ai/rules/rfc-compliance.md`). A guard fails closed even when the open version is shorter (`ai/rules/evidence.md`).
**You MUST name the rule that requires the machinery. When no rule requires it and no second use case exists, the machinery is yours to justify or delete.**

**Short is not simple. A dense expression the reader needs to simulate to understand fails this rule exactly as a five-file framework does, and it fails it in the less visible direction.**
**You MUST write the version that is boring to read.**

## Rationale

Two failures were already named in this repository and neither had an owning rule. `ai/rules/architecture.md` carried YAGNI and Simplicity as two rows in a 19-row design-principles table, and `ai/rules/quality.md` asked "Simplest solution? Over-engineered?" as one review question. Both are read after the design exists. Neither says what to do at the moment the shape of a fix is chosen, and neither is reachable from a trigger.

The standard is stated as a pair because each half fails alone. "Simplest" alone reads as permission to solve less of the problem, which rung 2 and rung 3 of `ai/rules/rule-precedence.md` already forbid. "Fully correct" alone is satisfied by any amount of machinery, which is how an interface with one implementation and an option nobody asked for enter the tree and stay there.

The cost this removes is paid by the next reader. Somebody who did not write the machinery cannot delete it safely, because the absence of a reason is not proof there was none. That is why `name-the-simpler-design-you-rejected` asks for one line. It lets the next reader delete the machinery, or keep it, on evidence.
