---
kind: directive
level: SHOULD
stage:
---
- **`writeGoPatterns` in `internal/le/hookruntime/writeedit.go` refuses allocation-heavy formatting and a hand-built buffer handle at edit time. The full encoding path SHOULD be audited with `/ze-find-alloc`, and a finding fixed with `/ze-fix-alloc file:line`.**
