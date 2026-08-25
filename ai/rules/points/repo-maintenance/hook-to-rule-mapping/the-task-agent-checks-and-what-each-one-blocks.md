---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `verdict` | `cli.md` | Task/Agent | Blocks a raw agent spawn when a skill covers the task. Name the skill instead. BLOCKING. |
| `review_model_refusal` | `planning.md` | Task/Agent | Blocks a review agent spawned off Opus 5. The other half of the same rule is `_model_refusal` in `scripts/dev/review_gate.py`, which refuses to RECORD a review taken off that model. BLOCKING. |
| `style_guide_reminder` | `go-standards.md` | Task/Agent | Warns when a brief that will produce Go never names `docs/contributing/ze-go-style.md`. A subagent inherits the session-start style read through no mechanism the main thread can verify, and cannot be audited afterwards either, because subagent transcripts live under `/tmp` and `check_system_tmp` refuses that path. WARN, never blocking: the population is a heuristic over prose, so a block would refuse correct work. |
