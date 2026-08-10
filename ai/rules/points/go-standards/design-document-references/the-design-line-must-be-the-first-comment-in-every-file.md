---
kind: directive
level: MUST
stage:
---
- The `// Design:` line MUST be the first comment in every file. Only compiler directives (`//go:build`) MAY precede it.
- `// Package` doc comments MUST go after the header block, not before it.
