---
kind: directive
level: MUST NOT
stage:
---
**First-party development or test tooling MUST NOT be added outside
`internal/le`, `internal/test`, or `internal/appliance`, and every command or
fixture MUST be registered in its native Go inventory.** A source file with no
caller is not a tool.
