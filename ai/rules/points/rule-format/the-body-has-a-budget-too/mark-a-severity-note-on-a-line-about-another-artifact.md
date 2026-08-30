---
kind: directive
level: MUST
stage:
---
**A line that legitimately describes ANOTHER artifact's severity MUST be marked `<!-- severity-note: whose severity this is -->`.** `internal/le/rules/lint.go` reads the RENDERED rule, so an unmarked line reads as this rule's own severity. The marker is line-scoped on purpose: a file-scoped opt-out would silently cover every later addition to that file.
