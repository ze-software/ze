---
kind: directive
level:
stage:
---
**Prefer your session's own directory**: `dir=$(scripts/dev/session-scratch.sh)` gives
`tmp/s/<session-id>/`, which is removed at SessionEnd, so scratch cannot outlive its
owner or collide with a sibling session (`ai/rules/commands.md`).
