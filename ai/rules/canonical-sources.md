# Canonical Sources and Sync Direction

**BLOCKING.** Never edit a generated file. Edit the canonical source, then sync.

## Sync Flows

| Canonical source | Generates | Sync command |
|------------------|-----------|--------------|
| `ai/INSTRUCTIONS.md` | `CLAUDE.md`, `AGENTS.md` | `make ze-ai-instructions` |
| `ai/skills/*.md` | `.claude/skills/*/SKILL.md`, `.codex/skills/*/SKILL.md`, `.agents/skills/*/SKILL.md` | `make ze-ai-sync` |

## What Is NOT Generated

`.claude/rules/*.md` are Claude-specific originals. Edit them directly.
`ai/rules/*.md` are tool-agnostic originals. Edit them directly.
These two directories are independent; neither generates the other.

## Mechanical Check

Before editing any file listed in the "Generates" column above, STOP.
Find its canonical source in the left column and edit that instead.

## Banned Actions

| Action | Fix |
|--------|-----|
| Editing `CLAUDE.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions` |
| Editing `AGENTS.md` directly | Edit `ai/INSTRUCTIONS.md`, run `make ze-ai-instructions` |
| Editing `.claude/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |
| Editing `.codex/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |
| Editing `.agents/skills/*/SKILL.md` directly | Edit `ai/skills/*.md`, run `make ze-ai-sync` |
