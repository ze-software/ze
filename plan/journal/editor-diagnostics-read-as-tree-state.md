| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | - | Claude Code LSP diagnostics (gopls) over shared `main` | Pushed compile errors were read as proof an agent broke the shared build; two agents were sent to fix breaks that did not exist. gopls also loads `tmp/session/*/` worktrees and other sessions' in-flight edits | open: keep gopls off `tmp/` worktrees; until then confirm with `go vet` in `main` plus `git status` |
