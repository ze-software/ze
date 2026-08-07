---
kind: directive
level:
stage:
---
- `switch args[0] { case "x": ... }` for command dispatch
- Manual "unknown command" error messages (the dispatcher handles this)
- Hand-written help listing subcommands (derive from registration)
