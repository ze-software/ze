---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `agentReviewModel` | `planning.md` | Task/Agent review prompts | Blocks a review agent spawned off Opus 5 and reports the accepted model evidence. BLOCKING. |
| `agentSkill` | `cli.md` | Task/Agent prompts covered by a ze skill | Blocks a raw agent spawn when a named skill owns the workflow. BLOCKING. |
| `agentStyleGuide` | `go-standards.md` | Task/Agent briefs that will produce Go | Warns when the brief does not name `docs/contributing/ze-go-style.md`. Advisory. |
