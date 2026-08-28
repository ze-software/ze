---
kind: table
level:
stage:
---
| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `./le ai skills-sync` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `./le ai skills-sync` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `./le ai skills-sync` |
| Editing `ai/rules/<rule>.md` directly | Edit the point under `ai/rules/points/<rule>/`, run `./le rules render-update` |
| Editing `ai/rules/INDEX.md` directly | Edit the rule's point or manifest, run `./le rules render-update`, then `./le rules index-update` |
| Editing `ai/rules/TRIGGERS.md` or `ai/rules/CORE.md` directly | Edit the rule's point or manifest, run `./le rules render-update`, then `./le rules condensed-update` |
