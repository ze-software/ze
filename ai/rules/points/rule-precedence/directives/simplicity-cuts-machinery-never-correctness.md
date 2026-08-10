---
kind: directive
level: MUST NOT
stage:
---
**Simplest-correct-solution MUST sit UNDER rungs 2 and 3; it MUST NOT sit beside them.** `ai/rules/simplicity.md` requires the simplest fully correct answer, and "fully correct" is what rungs 2 and 3 already own. It cuts machinery: an abstraction with one user, an option nobody asked for, a layer that transforms nothing. It MUST NOT cut correctness, conformance, a test, a guard, or an error path, and quality is 0% compromise.
**The simplest design is usually the HARDEST to find. "This was the pragmatic option under time pressure" is the tell that a lower rung is being read as a license.** Not seeing the simple design is a reason to think longer, or to ask which way. It MUST NOT be treated as a reason to ship the complicated answer or the incomplete one.
