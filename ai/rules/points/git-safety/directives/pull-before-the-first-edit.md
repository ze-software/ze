---
kind: directive
level: MUST
stage:
---
**Pull before the first edit. Every new top-level session MUST successfully run `git pull --rebase` on the current branch before its first edit in each checkout it uses, including a separate site or wiki checkout.** Inspect the working tree and index first and disable autostash for the pull. If the checkout is dirty, a merge or rebase is in progress, the upstream is missing, another writer makes the pull unsafe, or the pull fails or conflicts, MUST stop before editing and report the blocker; MUST NOT stash, discard work, or change branches to get past it. A subagent sharing that checkout MUST use its parent's successful pull and MUST NOT pull again underneath the session's edits.
