---
kind: table
level:
stage:
---
| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `make ze-ai-instructions-generate` or `make ze-ai-skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `make ze-ai-skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `make ze-rules-render-update` |
