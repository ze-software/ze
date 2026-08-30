---
kind: directive
level: MUST
stage:
---
**You MUST state a severity only after you have read the path that produces it.** "This
becomes a risk if we change X" and "this happens today" are different claims,
and only the second one earns priority.

**Correctness and reachability are two questions, and they are answered in
opposite directions.** Reading the producer settles what the code does. It says
nothing about whether anything runs it, and a function can be wrong in a way an
RFC forbids while harming nobody because no shipped configuration reaches it. So
a severity MUST rest on both: the producer read inward, and the call graph
walked outward to a caller a shipped build reaches. Neither is inferred from the
shape of the code.

**The outward walk is short, and it stops at the first gate that can refuse:** a
registration with a single registrant, a handler that declines a whole class of
input, a state no configuration can set. A path whose only callers are tests is
not reached at all. Where the walk cannot be completed, MUST report reachability
as unestablished rather than let the producer's correctness stand in for it.

**The verdict decides who acts, not how much the finding is worth.** Reachable
and wrong is a defect that ships. Unreachable and wrong is a feature that was
never wired, and it belongs to the owner as a scope question rather than to the
queue as a repair, because the larger fact is that the feature is absent.

**A green suite is not evidence of reachability, and neither is a passing gate.**
Both run the code deliberately, which is the property in question: a test can
call anything, and production cannot.

**Work that lands on an unreachable path MUST say so where it is recorded**, so
the next reader inherits the fact rather than the impression that the behavior
is live. Such code is worth keeping and worth labelling. What it MUST NOT be
called is fixed.
