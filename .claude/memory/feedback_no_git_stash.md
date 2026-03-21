---
name: No git stash
description: git stash is disallowed - never use git stash in any form
type: feedback
---

Never use `git stash` (or any variant like `git stash pop`, `git stash apply`, `git stash drop`).

**Why:** Multiple Claude instances run concurrently on the same repo. `git stash` uses a shared repo-global stack, so concurrent stash operations cross-contaminate data between sessions. Recovery involves `git reset`, which destroys work.

**How to apply:** When needing to set aside uncommitted changes, use worktrees (already the project pattern -- see `.claude/worktrees/`) or commit to a branch. Never reach for `git stash` as a shortcut. This applies to all variants: `stash`, `stash pop`, `stash apply`, `stash drop`, etc.
