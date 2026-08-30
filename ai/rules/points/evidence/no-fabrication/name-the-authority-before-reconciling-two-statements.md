---
kind: directive
level: MUST
stage:
rationale: plan/journal/claim-outlives-the-evidence-it-cites.md
---
**Before reconciling two statements of one fact, MUST decide which one is AUTHORITATIVE, and MUST say so.** A gate, a sync, or an agent that is told two things disagree, and not which is right, edits whichever is cheaper to edit. Cheaper almost always means the prose, so the true half is the half that gets rewritten.

Nothing about derivation causes this. A generated artifact and its source fail the same way as two hand-written statements: the failure is the absence of a declared ranking, not the direction the copy flowed. So this MUST be settled for any pair, and settling it MUST NOT be left to whoever meets the disagreement, who will be under time pressure and holding one of the two in their hands.

The reconciler MUST be pointed at the PRODUCER of the behavior and MUST NOT be pointed at either statement about it. Both statements are claims; the code that runs is the fact. Three cases in one day, each caught only because someone stopped instead of reconciling: a YANG leaf declared optional while its handler had always refused the call without it, so making the prose agree would have published a grammar weaker than the code; an RFC annotation asserting that no code path existed to test, where the path existed and was untested, which had lowered a MUST-level obligation; and a rule that told a reader to establish REACHABILITY by reading the producer, when reading the producer settles what code DOES and reachability runs the other way, toward callers.

The last one is the shape worth remembering, and it states the rule's own limit: **a reconciler MUST NOT be the authority on which of its inputs is authoritative.** There the two statements were a rule and reality, and the rule was simultaneously one of the two and the thing ranking them, so no external referee existed and the error survived every reading. A ranking a reconciler declares about itself is not a ranking. The authority MUST come from outside the pair, and the code that runs is the only candidate that always qualifies.
