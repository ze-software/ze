---
name: Memory directory is inside the repo
description: ~/.claude/projects/.../memory/ resolves to the repo's .claude/memory/. Files written there are git-tracked and must be committed.
type: feedback
---

The Claude memory path `~/.claude/projects/-Users-thomas-Code-codeberg-org-thomas-mangin-ze-main/memory/` resolves to the repo's `.claude/memory/` directory. They are the same location.

**Why:** Memory files written there show up in `git status` and must be committed like any other repo file. Treating them as "outside the repo" leads to forgetting to stage them.

**How to apply:**
- When creating or updating memory files, include them in the next relevant commit.
- MEMORY.md is git-tracked. Changes to it must be staged and committed.
- After writing any memory file, check `git status .claude/memory/` and stage the changes.
