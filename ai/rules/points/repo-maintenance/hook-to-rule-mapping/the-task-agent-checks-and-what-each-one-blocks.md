---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `verdict` | `cli.md` | Task/Agent | Blocks a raw agent spawn when a skill covers the task. Name the skill instead. BLOCKING. |
| `review_model_refusal` | `planning.md` | Task/Agent | Blocks a review agent spawned off Opus 5. The other half of the same rule is `_model_refusal` in `scripts/dev/review_gate.py`, which refuses to RECORD a review taken off that model. BLOCKING. |
