---
kind: note
level:
stage:
---
Use `subdispatch.New(name, summary)` for any command group that has sub-actions.
Register each sub-action with its handler and description. The dispatcher handles
help, unknown-command errors, and suggestions automatically.
