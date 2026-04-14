---
name: Parallel sessions, never use git stash
description: Multiple Claude sessions run in parallel on this repo; git stash is shared and will corrupt other sessions' work
type: feedback
originSessionId: 42272e16-ce48-4806-b423-98a8905a799c
---
The user runs multiple Claude sessions in parallel on this repo.

**Why:** `git stash` is a shared stack across all sessions in the same
worktree. If session A stashes to "get a clean tree", session B's
uncommitted work ends up in A's stash. When A later pops, B's changes
reappear in A's working tree — corrupting A's diff and hiding files B
was still editing. The user has been burned by this.

**How to apply:**
- Never run `git stash`, `git stash pop`, `git stash drop`, or any stash
  command. Not to "check what's pre-existing", not to "temporarily set
  aside", not for any reason.
- Never run destructive git verbs (`reset`, `restore`, `checkout --`,
  `clean`). The CLAUDE.md ABSOLUTE PROHIBITIONS cover this but it's
  worth restating.
- When files appear in the working tree that you did not create — check
  whether another session might have just committed or written them.
  Commits ahead of `origin/main` that you didn't author are a strong
  signal another session is active.
- To compare your change against what's committed, use `git diff`,
  `git diff HEAD`, `git diff --staged` — never `git stash` as a
  comparison tool.
- If you need a clean tree to test something: you don't. Test with the
  dirty tree. If you truly need isolation, ask the user first.
- If something you expect to be uncommitted shows up as clean + there's
  a new stash entry: stop and ask the user. Do not pop the stash.
