---
name: ze-extract
description: Use when working in the Ze repo and the user asks for ze-extract or wants Go symbols moved between files. Validate the source, destination, and symbols, use the repo extraction tool, verify the result, and clean up file annotations.
---

# Ze Extract

This skill moves Go symbols with the repo helper instead of manual cut-and-paste.

## Workflow

1. Require `source.go`, `dest.go`, and one or more symbol names.
2. Confirm the source exists, the symbols exist, and the destination is in the same Go package if it already exists.
3. Show the move plan before editing.
4. Build and run `scripts/dev/go_extract.go` through the repo helper binary.
5. Read both files afterward and repair `// Design:` and `// Related:` comments if needed.
6. If the destination is new, carry over any build tags and basic file annotations.
7. Run `go vet` on the affected package.

## Rules

- Do not use destructive git restore commands for recovery.
- If the tool leaves broken code, repair it explicitly and verify again.
