---
kind: directive
level: MUST
stage:
---
**A file modified during implementation and then removed MUST be committed in its current state first**, before the `remove` that deletes it (`ai/rules/planning.md`, Spec Closure). `./le commit create` already refuses a `remove` path that is not tracked.
