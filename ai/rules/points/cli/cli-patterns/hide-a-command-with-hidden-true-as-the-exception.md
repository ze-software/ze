---
kind: directive
level: MUST
stage:
---
**Opt-out:** MUST set `Hidden: true` on a `CommandDecl` to suppress a command from
completion and help. The command still works when typed in full. MUST use this only
for internal/diagnostic commands that operators SHOULD NOT discover through
tab-completion. Hidden is the exception, not the default.
