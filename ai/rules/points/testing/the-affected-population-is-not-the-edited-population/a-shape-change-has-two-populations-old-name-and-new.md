---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
**When a payload SHAPE changes, you MUST search for the NEW name as well as the old one.** Searching what you REMOVE finds code that stops working. It cannot find code that breaks because of what you ADD. An added key CAN land in a branch that already reads it for a different producer. That branch then handles the new payload wrongly and quietly. Nothing prompts this search, because the new name is yours and feels safe.

**For the OLD name, a hit is not yet a consumer: you MUST establish WHICH producer emits the key it matched.** One key name can have several producers, and only some of them are yours. A key name is not a producer.

**A consumer of a GENERIC payload is reached by neither search.** No search for the old key names it, because it never read that key. No list of the changed command's consumers names it either, because it consumes whatever payload it is handed. It is found only by searching the new key.

**The audit is owed AGAIN for the fix, and this is the half a reader will miss.** The natural reading of the rule above is "audit the consumers before you change the shape". But a repair to a shape change is itself a shape change. It lands in a function whose branches each carry a prior contract, so it earns its own pass over both populations.

**A repair that normalizes every element and drops the rest breaks the branches that were passing their elements through.** State each branch's prior contract instead: a branch passes through what it cannot normalize, and the branch that owns the new shape skips what does not carry it.

**The colliding branch does not fail, which is why it stays quiet.** It returns a valid empty result, and an empty answer reads as a true answer about empty state. Refusing would be loud and correct.

**The pre-push gate catches this and the focused tests do not.** A focused run covers the code you edited, and a shape change is defined by who READS it.
