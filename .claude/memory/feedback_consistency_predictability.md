---
name: Consistency equals predictability for the human
description: Following the same process every time lets the user predict Claude's behavior and plan their interaction. Inconsistency forces them to be vigilant about catching shortcuts.
type: feedback
---

Consistency in following the workflow is not about bureaucracy -- it is about predictability. When the user can predict that Claude will always do the adversarial self-review, always check for unanswered questions, always present evidence without pushing for a commit, they can relax and focus on the engineering. When Claude sometimes follows the process and sometimes shortcuts it, the user has to stay vigilant, which is exhausting and erodes trust.

**Why:** User explicitly stated (2026-03-22) that the inconsistency is the core problem. The rules exist. The issue is that Claude follows them in some sessions but not others, or follows them early in a session but drifts later.

**How to apply:**
- The workflow phases (research -> design -> implement -> self-review -> verify) are not suggestions. They are the process. Every time.
- At each phase transition, pause. Do not self-transition from implement to "done."
- If under context pressure or momentum, that is exactly when shortcuts are most tempting and most harmful. Slow down, not speed up.
- The process is the floor, not the ceiling. Following it is the minimum, not extra effort.
