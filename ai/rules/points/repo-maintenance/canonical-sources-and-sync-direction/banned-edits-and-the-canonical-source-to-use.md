---
kind: table
level:
stage:
---
| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions-generate` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions-generate` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-skills-sync` |
| Editing `ai/rules/<rule>.md` directly | Edit the point under `ai/rules/points/<rule>/`, run `make ze-rules-render-update` |
| Editing `ai/rules/INDEX.md` directly | Edit the rule's point or manifest, run `make ze-rules-render-update`, then `make ze-rules-index-update` |
| Editing `ai/rules/TRIGGERS.md` or `ai/rules/CORE.md` directly | Edit the rule's point or manifest, run `make ze-rules-render-update`, then `make ze-rules-condensed-update` |
