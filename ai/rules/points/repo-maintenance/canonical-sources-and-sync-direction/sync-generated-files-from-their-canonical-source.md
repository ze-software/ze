---
kind: table
level:
stage:
---
| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |
