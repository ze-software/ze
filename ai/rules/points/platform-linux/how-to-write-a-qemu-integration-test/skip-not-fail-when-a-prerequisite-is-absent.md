---
kind: note
level:
stage:
---
The same test file may run in environments with different capabilities. Use
`t.Skip`, not `t.Fatal`, when a prerequisite is absent:
