---
kind: directive
level: MUST
stage:
---
**UserPromptSubmit stdout reaches the model. UserPromptSubmit stderr does not.** A reminder that MUST land in the context writes to stdout. A banner that MUST cost no context tokens writes to stderr, as the native lifecycle actions in `internal/le/hookruntime/lifecycle.go` do. The two stdout reminders below fire on every turn, so each one stays a single line.
