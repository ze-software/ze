---
kind: note
level:
stage:
---
In loops the cost compounds. Each iteration allocates, and collecting into
`[]string` + `strings.Join` adds yet another:
