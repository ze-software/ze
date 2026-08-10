---
kind: directive
level: MUST NOT
stage:
---
- Subagents inherit the PHASE, not the task shape: reviewer subagents spawned during review stay on the review model, implementation subagents stay on the implementation model.
- The `Agent` tool's `model` parameter selects a family (`opus`, `sonnet`, `haiku`), not a minor version, so it cannot pin 4.8 against 5. The phase-to-model mapping above is about the session driving the work.
- MUST NOT downgrade a subagent to a cheaper model because its lens looks mechanical. `ai/skills/ze-review-deep.md` and `ai/skills/ze-debug.md` spawn every agent on `opus` for this reason. If cost forces a reduction, cut the NUMBER of agents, never the model they run on.
