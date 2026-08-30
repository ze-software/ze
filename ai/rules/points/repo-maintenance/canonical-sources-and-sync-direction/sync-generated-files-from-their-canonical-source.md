---
kind: directive
level: MUST NOT
stage:
---
**A generated file MUST NOT be edited. Edit its canonical source, then run its sync command.**

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `./le ai skills-sync` |
| `ai/skills/*.md` | the per-tool `SKILL.md` mirrors | `./le ai skills-sync` |
| `ai/rules/points/<rule>/` | `ai/rules/<rule>.md` | `./le rules render-update` |
| A rule's points or manifest | `TRIGGERS.md`, `CORE.md`, `INDEX.md` | `./le rules render-update`, then `condensed-update` and `index-update` |
