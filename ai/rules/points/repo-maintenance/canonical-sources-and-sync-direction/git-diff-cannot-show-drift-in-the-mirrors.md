---
kind: directive
level: MUST
stage:
---
**The `CLAUDE.md`, `AGENTS.md`, and skill mirrors are gitignored, so `git diff` can NEVER show drift for them.** `./le ai sync-check` compares them against a fresh generation; the session-start hook warns `generated agent files are stale` when a resync is needed. So drift in a mirror MUST be read from `./le ai sync-check` rather than from a
diff, and it MUST be fixed with `./le ai skills-sync`. `ai/rules/<rule>.md` is the one generated rule surface that IS tracked, so `git diff` shows its drift, and `./le rules render-check` reaches the same verdict without writing.
