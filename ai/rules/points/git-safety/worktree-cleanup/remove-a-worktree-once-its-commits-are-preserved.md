---
kind: directive
level: MUST
stage:
---
**A worktree MUST be removed as soon as its commits are merged, rebased, or pushed, and the removal MUST clear the registration with `git worktree prune --expire now`.** A bare `git worktree prune` respects a three-month expiry, so a stale entry survives it, holds its branch checked out against rebase and deletion, and stays invisible for a quarter.
