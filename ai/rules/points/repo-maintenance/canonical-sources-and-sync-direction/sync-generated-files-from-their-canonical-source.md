---
kind: directive
level: MUST NOT
stage:
---
**A generated file MUST NOT be edited. Edit its canonical source, then run its sync command.**

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `AGENTS.md`, which every agent reads (the sync removes a root `CLAUDE.md`, because Claude Code ignores `AGENTS.md` while one exists) | `./le ai sync update` |
| `ai/skills/*.md` | the per-tool `SKILL.md` mirrors | `./le ai sync update` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le ai rules update render` |
| A rule's points or manifest | `TRIGGERS.md`, `CORE.md` | `./le ai rules update render`, then `update condensed` |
