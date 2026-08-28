---
kind: note
level:
stage:
---
Do not add first-party development or test tooling outside `internal/le`,
`internal/test`, or `internal/appliance`. Register every command or fixture in
its native Go inventory; a source file with no caller is not a tool.
