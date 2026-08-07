---
kind: directive
level:
stage:
---
**Opt-out:** Set `Hidden: true` on a `CommandDecl` to suppress a command from
completion and help. The command still works when typed in full. Use this only
for internal/diagnostic commands that operators should not discover through
tab-completion. Hidden is the exception, not the default.
