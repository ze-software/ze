---
kind: note
level:
stage:
---
`writeGoPatterns` in `internal/le/hookruntime/writeedit.go` blocks allocation-heavy formatting and fake buffer handles. Audit the full encoding path with `/ze-find-alloc`; fix it with `/ze-fix-alloc file:line`.
