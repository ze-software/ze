---
kind: directive
level:
stage:
---
**Reads never block:** `Read`, `Grep`, `Glob`, `LSP`, `WebFetch`, `WebSearch` are never rejected. Two of them write a non-blocking freshness marker so the `design-without-lsp` gate knows the implementation was investigated: `LSP` (via `mark-lsp-invoked.sh`) and `Read` of a `.go` under `internal/`/`pkg/`/`cmd/` (via `mark-source-read.sh`). Only mutating/executing tools (`Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit`, `Task`, `Agent`) and `ToolSearch` (which loads LSP) are actually gated.
