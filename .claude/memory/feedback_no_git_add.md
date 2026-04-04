---
name: No git add from Claude sessions
description: Never run git add -- write a commit script instead. Multiple sessions share the staging area.
type: feedback
---

Never run `git add`. Write a commit script to `tmp/commit-SESSION.sh` containing the add + commit commands. The user runs it.

**Why:** Multiple Claude sessions run concurrently on the same repo. When one session runs `git add`, it contaminates the staging area for other sessions. This caused duplicate/wrong commits when two sessions staged conflicting files.

**How to apply:** After completing work, write `tmp/commit-XXXX.sh` with `git add` + `git commit` commands. Never touch the staging area directly.
