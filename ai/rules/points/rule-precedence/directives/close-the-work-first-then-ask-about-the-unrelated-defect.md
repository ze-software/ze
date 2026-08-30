---
kind: directive
level: MUST NOT
stage:
---
**Closing comes first, and the same question decides the ORDER as well as the verdict: a defect the goal does NOT depend on MUST NOT be fixed on the way to closing the work in hand.** `completion.md` makes you the owner of a defect you walked into, and it does not make you its owner this minute. Work that was finished but never landed is the most expensive failure this repo has, and an unrelated fix folded into a closing commit is its usual cause: it costs the commit its single focus and the review its scope, and it restarts the gates that were already green.
**The route is fixed and it has three steps: MUST write ONE row in `plan/journal/<class>.md`, close the work in hand, and stop (owner directive, 2026-08-10).** MUST NOT write a spec for it, MUST NOT open a deferral row, and MUST NOT ask Thomas whether to implement one (`completion.md`). MUST NOT fix it after closing either. Rows accumulate by problem class, and a class that collects rows earns its fix in a deliberate pass over the journal.
**Three finds are fixed on the spot, and `completion.md` names all three: a defect that stops a test or a gate from passing, a test that is wrong about what it asserts, and code related to the problem in hand.** The blocking defect is the first of them, because there is no closing the work in hand around it.
