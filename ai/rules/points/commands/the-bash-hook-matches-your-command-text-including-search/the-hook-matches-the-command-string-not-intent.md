---
kind: note
level:
stage:
---
`internal/le/hookruntime/bash.go` judges the command string. It cannot distinguish
a forbidden verb being executed from the same token appearing in a search
pattern, so a read-only search can be refused.
