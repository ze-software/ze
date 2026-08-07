---
kind: note
level:
stage:
---
When you migrate, also drop the command's verb from `IsReadOnlyPath`
(`internal/component/plugin/server/command.go`) if it was only there as a legacy
noun-first form.
