---
kind: note
level:
stage:
---
The `design-without-lsp` check in `.claude/hooks/pretool-writeedit.py` blocks
writing a `plan/spec-*.md` or `plan/design-*.md` file unless this session has
investigated implementation source (read a `.go` under `internal/`, `pkg/`, or
`cmd/`, or used the LSP tool) within the last 30 minutes. It catches the case
where a spec is authored for a behavioral claim that was never traced to the
producing code. It is a backstop, not a guarantee: it cannot verify that the
code you read was the code your claim depends on. See `ai/rules/repo-maintenance.md`.
