---
kind: directive
level: MUST
stage:
---
- **A `UserPromptSubmit` reminder that MUST land in the context writes to stdout; a banner that MUST cost no context tokens writes to stderr.** The reminders fire on every turn, so each one MUST stay a single line. `.claude/hooks/README.md` lists the lifecycle actions and their events.
