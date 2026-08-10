---
kind: directive
level: MUST NOT
stage:
---
The following dispatch patterns MUST NOT be used:
- `switch args[0] { case "x": ... }` for command dispatch
- Manual "unknown command" error messages (the dispatcher handles this)
- Hand-written help listing subcommands (derive from registration)
