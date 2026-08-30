---
kind: directive
level: MUST
stage:
---
1. The command MUST route its output through `ApplyPipes` or a `ProcessPipes*` wrapper.
2. If the command has a custom display path that bypasses `ApplyPipes` (e.g. `| log`
   rendering directly from in-memory state), that path MUST still honor data-transform
   pipes (`| resolve`, `| origin`) by applying them to the data before rendering.
3. Display-mode pipes (`| log`, `| no-more`) are flags, not data transforms. They
   change HOW output is shown, not WHAT data is shown. Data-transform pipes apply
   regardless of display mode.

What each operator does, and which class it belongs to, is
`docs/features/formatting.md`; the catalog it is generated from is
`internal/component/command/pipe_catalog.go`.
