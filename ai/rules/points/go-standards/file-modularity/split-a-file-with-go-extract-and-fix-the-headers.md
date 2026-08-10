---
kind: directive
level: MUST
stage:
---
- **Tool:** `go build -o bin/go_extract ./scripts/dev/go_extract.go && bin/go_extract <source.go> <dest.go> <symbol1> [symbol2 ...]` moves named declarations (with doc comments) to dest, runs `goimports` on both. Note: `goimports` cannot resolve aliased imports; you MUST add those manually to the new file.
- Zero semantic effect: Go compiles all files in a package together
- File-local types move with their functions
- Shared test helpers stay in base `_test.go`
- `goimports` handles import cleanup
- You MUST name the file after its concern: `reactor_announce.go`, `session_handlers.go`
- New files: you MUST copy `// Design:` from original, and review the topic annotation (see "Design Document References" below)
- All resulting files: you MUST add `// Related:` to siblings (see "File Cross-References" below)
