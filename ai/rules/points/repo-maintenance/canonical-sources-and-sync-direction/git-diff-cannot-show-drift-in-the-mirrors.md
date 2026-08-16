---
kind: directive
level: MUST
stage:
---
**The `CLAUDE.md`, `AGENTS.md` and skill mirrors are gitignored, so `git diff` can NEVER show drift for them.** `make ze-ai-sync-check` (also part of `make ze-generated-files-update-check`) compares content against a fresh generation; the session-start hook runs it and warns `generated agent files are stale` when a resync is needed. You MUST fix it with `make ze-generated-files-update`. `ai/rules/<rule>.md` is the one "Generates" target that IS tracked, so `git diff` does show its drift, and `make ze-rules-render-check` reaches the same verdict, but writes nothing.
