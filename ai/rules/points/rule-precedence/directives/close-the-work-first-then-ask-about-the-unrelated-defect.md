---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/rule-precedence.md
excepted-by: rule-precedence/directives/start-a-fix-agent-for-an-investigated-defect-without-asking
---
**Closing comes first, and the same question decides the order as well as the verdict: a defect the goal does not depend on MUST NOT be fixed on the way to closing the work in hand.** `completion.md` makes you the owner of a defect you walked into, and it does not make you its owner this minute. Work that was finished but never landed is the most expensive failure this repo has, and an unrelated fix folded into a closing commit is its usual cause: it costs the commit its single focus and the review its scope, and it restarts the gates that were already green.
**The route is fixed and it has three steps: MUST write one row in `plan/journal/<class>.md` and list it in the owned journal rows table of the spec that will fix it, close the work in hand, and stop.** MUST NOT record it anywhere else, and MUST NOT ask Thomas whether to implement it (`completion.md`). MUST NOT fix it after closing either, unless its investigation is complete and the next point's conditions hold. Rows accumulate by problem class, and a class that collects rows earns its fix in a deliberate pass over the journal.
**Every open row MUST be owned: the owning spec is an existing one whose mechanism fits the defect, or a new one homed per `planning.md`.** `./le commit create` refuses a commit that adds or changes an open row no spec's table lists (`plan/journal/README.md`, "Owned rows"). Resolving rows follows `plan/post-release/engineering/spec-journal-resolution.md`.
**Two finds are fixed on the spot, and only these two: a test that is wrong about what it asserts, and code related to the problem in hand.** A red test or a red gate is not a third: `pre-release.md` routes it. A defect that blocks the goal of the work in hand is not a find at all. It MUST be fixed, because there is no closing the work in hand around it (`completion.md`).
