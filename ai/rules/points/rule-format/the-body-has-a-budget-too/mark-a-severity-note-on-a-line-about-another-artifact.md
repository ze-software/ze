---
kind: note
level:
stage:
---
`internal/le/rules/lint.go` enforces all of this, and it reads the RENDERED
rule rather than the points, so the metadata contract is what it always was.
When a line legitimately
describes ANOTHER artifact's severity (as `repo-maintenance.md` does), mark that
line `<!-- severity-note: whose severity this is -->`. The marker is
line-scoped on purpose: a file-scoped opt-out would silently cover every later
addition to that file.
