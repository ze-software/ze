---
kind: directive
level: MUST
stage:
---
When you migrate a command, you MUST also drop its verb from `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) when the verb was there only as a
legacy noun-first form.
