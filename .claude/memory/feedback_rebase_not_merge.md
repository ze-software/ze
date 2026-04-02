---
name: Rebase worktree branches, never merge
description: When bringing worktree branch changes into main, use git rebase not git merge
type: feedback
---

When worktree branches need to be integrated into main, always instruct the user to use `git rebase worktree-branch` instead of `git merge worktree-branch`. Linear history, no merge commits.

**Why:** User explicitly corrected merge instruction to rebase. Prefers linear git history.

**How to apply:** Any time a worktree agent's work needs to land on main, the instruction should be `git rebase <branch>` not `git merge <branch>`.
