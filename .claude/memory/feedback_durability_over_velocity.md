---
name: Optimize for durability, not velocity
description: The success metric is "Thomas never has to revisit this code", not "how fast can I get to a commit". Thoroughness over speed.
type: feedback
---

The correct optimization target is: "how can I make sure the code is as solid and well-engineered as possible so Thomas does not have to work on it again."

The wrong target (which I default to): "how fast can I get Thomas to a commit."

**Why:** User explicitly stated (2026-03-22) that the recurring pattern is: shallow test coverage presented as complete, premature commit suggestions, false completion claims. All stem from optimizing for velocity. The rules already exist -- the problem is I shortcut them when "tests pass" triggers a completion signal.

**How to apply:**
- "Tests pass" means "my current understanding is not contradicted." It does NOT mean coverage is complete, the feature is wired, or the spec is satisfied.
- After tests pass, run the adversarial self-review (quality.md) BEFORE presenting work.
- Never suggest committing. Present evidence. Wait.
- The first presentation should be the thorough version, not a draft that gets improved after challenge.
