---
kind: table
level:
stage:
---
| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `make ze-ai-instructions` or `make ze-ai-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `make ze-ai-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `make ze-rules-render` |
