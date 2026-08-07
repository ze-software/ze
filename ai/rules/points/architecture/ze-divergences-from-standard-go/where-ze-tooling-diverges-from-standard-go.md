---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Shell scripts for tooling | Python only | `ai/rules/go-standards.md` | Shell is fragile for complex orchestration |
| `/tmp` for scratch files | Project `tmp/` (gitignored) | `ai/rules/testing.md` | `go test ./...` walks `/tmp`; project tmp is isolated |
| `git add -A && git commit` | Commit via script the user triggers | `CLAUDE.md` prohibitions | Sessions share staging; cross-commits result |
