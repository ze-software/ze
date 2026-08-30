---
kind: directive
level: MUST NOT
stage:
---
**A generated file MUST NOT be edited. Edit its canonical source, then run its sync command.**

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |
| A rule's points or manifest | `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` | `./le rules render-update`, then `./le rules condensed-update` |
| A rule's points or manifest | `ai/rules/INDEX.md` | `./le rules render-update`, then `./le rules index-update` |
