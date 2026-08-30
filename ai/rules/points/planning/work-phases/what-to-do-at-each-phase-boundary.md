---
kind: directive
level: MUST
stage:
---
**At each boundary below, the action in its row MUST be taken.**

| Situation | Do |
|-----------|-----|
| The spec is approved and coding is about to start | Start. No model switch is needed, and no announcement is owed |
| Implementation is complete and the Review Gate is next | Spawn ONE closure agent and let it run the review itself, or hand off to a fresh session. Never review your own implementation inline, and never let the closure agent spawn readers of its own |
| A review or audit produces fixes | The fixes are implementation, so make them. The re-review that follows is a fresh pass, not the same context re-reading itself |
| You are mid-phase and the work has changed shape | Say so plainly and let the operator decide. Do not silently continue as if nothing moved |
| The work is a one-line mechanical edit with no design or review content | Proceed. This rule governs phases, not keystrokes |
